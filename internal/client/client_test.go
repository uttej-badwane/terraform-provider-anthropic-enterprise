package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/mock"
)

func newClients(t *testing.T) (*mock.Server, *client.Client, *client.Client, *client.Client) {
	t.Helper()
	srv := mock.NewServer()
	t.Cleanup(srv.Close)
	mk := func(cfg client.Config) *client.Client {
		cfg.BaseURL = srv.URL
		c, err := client.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	return srv,
		mk(client.Config{AdminAPIKey: mock.AdminKey}),
		mk(client.Config{OAuthToken: mock.OAuthToken}),
		mk(client.Config{EnterpriseAPIKey: mock.EnterpriseKey})
}

// A client with no credential is valid, because the federated token exchange
// authenticates with the assertion in its body rather than a key. Endpoints
// that do need one refuse at request time, naming the credential they want.
func TestNewWithoutCredentials(t *testing.T) {
	c, err := client.New(client.Config{})
	if err != nil {
		t.Fatalf("a credential-free client is valid: %v", err)
	}

	_, err = c.ListWorkspaces(context.Background(), false)
	if err == nil {
		t.Fatal("expected a request needing a credential to fail")
	}
	var missing *client.MissingCredentialError
	if !errors.As(err, &missing) {
		t.Fatalf("got %v, want a MissingCredentialError naming the credential", err)
	}
	if missing.Class != client.CredAdmin {
		t.Fatalf("got class %v, want %v", missing.Class, client.CredAdmin)
	}
}

func TestNewRejectsABadBaseURL(t *testing.T) {
	if _, err := client.New(client.Config{BaseURL: "::bad", AdminAPIKey: "x"}); err == nil {
		t.Fatal("expected error for bad base url")
	}
}

func TestCredentialRouting(t *testing.T) {
	_, admin, oauth, ent := newClients(t)
	ctx := context.Background()

	if _, err := admin.GetOrganization(ctx); err != nil {
		t.Fatalf("admin /me: %v", err)
	}
	if _, err := oauth.GetOrganization(ctx); err != nil {
		t.Fatalf("oauth /me: %v", err)
	}
	// OAuth-only endpoint with only an admin key: client-side missing credential.
	_, err := admin.ListServiceAccounts(ctx, false)
	if !client.IsMissingCredential(err) {
		t.Fatalf("expected MissingCredentialError, got %v", err)
	}
	// Enterprise endpoint with only an admin key.
	_, err = admin.ListRBACGroups(ctx)
	if !client.IsMissingCredential(err) {
		t.Fatalf("expected MissingCredentialError, got %v", err)
	}
	// Enterprise key on a Console endpoint: the API says not supported (400).
	_, err = ent.ListWorkspaces(ctx, false)
	if !client.IsMissingCredential(err) {
		t.Fatalf("expected MissingCredentialError (no admin cred), got %v", err)
	}
	// Enterprise key reaches users and groups.
	if _, err := ent.ListUsers(ctx, client.UserListOptions{}); err != nil {
		t.Fatalf("enterprise users: %v", err)
	}
	if _, err := ent.ListRBACGroups(ctx); err != nil {
		t.Fatalf("enterprise groups: %v", err)
	}
	// OAuth reaches everything on the Console side.
	if _, err := oauth.ListServiceAccounts(ctx, false); err != nil {
		t.Fatalf("oauth service accounts: %v", err)
	}
}

func TestNotFoundAndErrors(t *testing.T) {
	_, admin, _, _ := newClients(t)
	_, err := admin.GetWorkspace(context.Background(), "wrkspc_missing")
	if !client.IsNotFound(err) {
		t.Fatalf("expected 404, got %v", err)
	}
	var apiErr *client.APIError
	if ok := asAPIError(err, &apiErr); !ok || apiErr.Type != "not_found_error" || apiErr.RequestID == "" {
		t.Fatalf("unexpected error shape: %#v", err)
	}
}

func asAPIError(err error, target **client.APIError) bool {
	e, ok := err.(*client.APIError)
	if ok {
		*target = e
	}
	return ok
}

func TestCursorPagination(t *testing.T) {
	_, admin, _, _ := newClients(t)
	ctx := context.Background()
	for i := 0; i < 250; i++ {
		if _, err := admin.CreateInvite(ctx, client.InviteCreate{Email: fmt.Sprintf("p%03d@example.com", i), Role: "user"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := admin.ListInvites(ctx, client.InviteListOptions{Statuses: []string{"pending"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 250 {
		t.Fatalf("expected 250 invites, got %d", len(got))
	}
	seen := map[string]bool{}
	for _, inv := range got {
		if seen[inv.ID] {
			t.Fatalf("duplicate id across pages: %s", inv.ID)
		}
		seen[inv.ID] = true
	}
}

func TestTokenPagination(t *testing.T) {
	_, _, _, ent := newClients(t)
	ctx := context.Background()
	for i := 0; i < 230; i++ {
		if _, err := ent.CreateRBACGroup(ctx, client.RBACGroupWrite{Name: fmt.Sprintf("group-%03d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ent.ListRBACGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 231 { // 230 + seeded SCIM group
		t.Fatalf("expected 231 groups, got %d", len(got))
	}
}

func TestRetryOnServerError(t *testing.T) {
	srv, admin, _, _ := newClients(t)
	srv.FailNext(2)
	if _, err := admin.GetOrganization(context.Background()); err != nil {
		t.Fatalf("expected retries to succeed, got %v", err)
	}
	if n := srv.Requests(); n != 3 {
		t.Fatalf("expected 3 requests (2 failures + 1 success), got %d", n)
	}
}

func TestWorkspaceLifecycle(t *testing.T) {
	_, admin, _, _ := newClients(t)
	ctx := context.Background()
	ws, err := admin.CreateWorkspace(ctx, client.WorkspaceCreate{Name: "tf-acc-ws", Tags: map[string]string{"env": "test"}})
	if err != nil {
		t.Fatal(err)
	}
	if ws.ID == "" || ws.Type != "workspace" || ws.ArchivedAt != nil || !ws.DataResidency.AllowedInferenceGeos.Unrestricted {
		t.Fatalf("unexpected workspace: %+v", ws)
	}
	upd, err := admin.UpdateWorkspace(ctx, ws.ID, client.WorkspaceUpdate{Name: ptr("tf-acc-ws-2"), Tags: map[string]*string{"env": nil, "team": ptr("x")}})
	if err != nil {
		t.Fatal(err)
	}
	if upd.Name != "tf-acc-ws-2" || len(upd.Tags) != 1 || upd.Tags["team"] != "x" {
		t.Fatalf("unexpected update: %+v", upd)
	}
	arch, err := admin.ArchiveWorkspace(ctx, ws.ID)
	if err != nil || arch.ArchivedAt == nil {
		t.Fatalf("archive failed: %v %+v", err, arch)
	}
	live, _ := admin.ListWorkspaces(ctx, false)
	all, _ := admin.ListWorkspaces(ctx, true)
	if len(live) != 0 || len(all) != 1 {
		t.Fatalf("expected 0 live / 1 total, got %d / %d", len(live), len(all))
	}
}

func TestFederationLifecycle(t *testing.T) {
	_, admin, oauth, _ := newClients(t)
	ctx := context.Background()
	ws, _ := admin.CreateWorkspace(ctx, client.WorkspaceCreate{Name: "prod"})
	sa, err := oauth.CreateServiceAccount(ctx, client.ServiceAccountCreate{Name: "ci-bot", Description: ptr("CI")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := oauth.CreateServiceAccount(ctx, client.ServiceAccountCreate{Name: "ci-bot"}); !client.IsConflict(err) {
		t.Fatalf("expected 409 on duplicate slug, got %v", err)
	}
	iss, err := oauth.CreateFederationIssuer(ctx, client.FederationIssuerCreate{Name: "gha", IssuerURL: "https://token.actions.githubusercontent.com"})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := oauth.CreateFederationRule(ctx, client.FederationRuleCreate{IssuerID: iss.ID, Name: "deploy", OAuthScope: "workspace:inference",
		Match:  client.RuleMatch{SubjectPrefix: ptr("repo:org/repo:ref:refs/heads/main")},
		Target: client.RuleTarget{Type: "service_account", ServiceAccountID: sa.ID}, WorkspaceID: &ws.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(rule.WorkspaceIDs) != 1 || rule.Target.ServiceAccountName != nil {
		t.Fatalf("create response should omit read-time names: %+v", rule)
	}
	if got, err := oauth.GetFederationRule(ctx, rule.ID); err != nil || got.Target.ServiceAccountName == nil || got.IssuerName == nil {
		t.Fatalf("read must resolve issuer_name and service_account_name: %v %+v", err, got)
	}
	if _, err := oauth.ArchiveServiceAccount(ctx, sa.ID); !client.IsBadRequest(err) {
		t.Fatalf("expected 400 archiving targeted service account, got %v", err)
	}
	upd, err := oauth.UpdateFederationRule(ctx, rule.ID, client.FederationRuleUpdate{Description: client.Null[string]()})
	if err != nil || upd.Description != nil {
		t.Fatalf("null description update failed: %v %+v", err, upd)
	}
	if _, err := oauth.ArchiveFederationRule(ctx, rule.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := oauth.ArchiveServiceAccount(ctx, sa.ID); err != nil {
		t.Fatalf("archive after rule archived: %v", err)
	}
	if _, err := oauth.ArchiveFederationIssuer(ctx, iss.ID); err != nil {
		t.Fatal(err)
	}
}

func TestOptJSON(t *testing.T) {
	type body struct {
		Description client.Opt[string] `json:"description,omitzero"`
		Role        *string            `json:"role,omitempty"`
	}
	b, _ := json.Marshal(body{})
	if string(b) != `{}` {
		t.Fatalf("unset Opt should be omitted, got %s", b)
	}
	b, _ = json.Marshal(body{Description: client.Null[string]()})
	if string(b) != `{"description":null}` {
		t.Fatalf("null Opt, got %s", b)
	}
	b, _ = json.Marshal(body{Description: client.Some("x")})
	if string(b) != `{"description":"x"}` {
		t.Fatalf("set Opt, got %s", b)
	}
	var g client.AllowedGeos
	if err := json.Unmarshal([]byte(`"unrestricted"`), &g); err != nil || !g.Unrestricted {
		t.Fatalf("unrestricted union: %v %+v", err, g)
	}
	if err := json.Unmarshal([]byte(`["us","global"]`), &g); err != nil || g.Unrestricted || len(g.Geos) != 2 {
		t.Fatalf("geos union: %v %+v", err, g)
	}
}

func ptr[T any](v T) *T { return &v }

func TestWave2Endpoints(t *testing.T) {
	srv, admin, _, ent := newClients(t)
	ctx := context.Background()
	mk := func(cfg client.Config) *client.Client {
		cfg.BaseURL = srv.URL
		c, err := client.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	comp := mk(client.Config{ComplianceAPIKey: mock.ComplianceKey})
	ana := mk(client.Config{AnalyticsAPIKey: mock.AnalyticsKey})

	reqs, err := ent.ListSpendLimitIncreaseRequests(ctx, client.IncreaseRequestListOptions{Statuses: []string{"pending"}})
	if err != nil || len(reqs) != 1 || reqs[0].SpendSummary == nil {
		t.Fatalf("increase requests: %v %+v", err, reqs)
	}
	buckets, err := admin.GetUsageReport(ctx, client.UsageReportParams{StartingAt: "2026-01-01T00:00:00Z", EndingAt: "2026-01-05T00:00:00Z", GroupBy: []string{"model"}})
	if err != nil || len(buckets) != 4 || buckets[0].Results[0].Model == nil {
		t.Fatalf("usage: %v %d", err, len(buckets))
	}
	costs, err := admin.GetCostReport(ctx, client.CostReportParams{StartingAt: "2026-01-01T00:00:00Z", GroupBy: []string{"description"}})
	if err != nil || len(costs) != 3 || costs[0].Results[0].TokenType == nil {
		t.Fatalf("cost: %v %d", err, len(costs))
	}
	cc, err := admin.GetClaudeCodeUsageReport(ctx, "2026-01-01")
	if err != nil || len(cc) != 2 || cc[0].Actor.EmailAddress == nil {
		t.Fatalf("claude code: %v %d", err, len(cc))
	}
	sums, err := ana.GetAnalyticsSummaries(ctx, client.AnalyticsSummariesParams{StartingDate: "2026-01-01", EndingDate: "2026-01-03"})
	if err != nil || len(sums) != 2 {
		t.Fatalf("summaries: %v %d", err, len(sums))
	}
	if _, err := admin.GetAnalyticsSummaries(ctx, client.AnalyticsSummariesParams{StartingDate: "2026-01-01"}); !client.IsMissingCredential(err) {
		t.Fatalf("admin key must not satisfy analytics: %v", err)
	}
	orgs, err := comp.ListComplianceOrganizations(ctx)
	if err != nil || len(orgs) != 2 {
		t.Fatalf("compliance orgs: %v %d", err, len(orgs))
	}
	users, err := ent.ListComplianceUsers(ctx, orgs[0].UUID) // enterprise key falls back
	if err != nil || len(users) == 0 {
		t.Fatalf("compliance users via enterprise key: %v", err)
	}
	set, err := comp.GetEffectiveOrganizationSettings(ctx, orgs[1].UUID)
	if err != nil || len(set.Settings) == 0 || len(set.APIKeys) != 1 {
		t.Fatalf("settings: %v", err)
	}
	if _, err := comp.GetEffectiveOrganizationSettings(ctx, orgs[0].UUID); !client.IsNotFound(err) {
		t.Fatalf("parent org must 404 on settings: %v", err)
	}
	groups, err := comp.ListComplianceGroups(ctx)
	if err != nil || len(groups) != 1 || len(groups[0].Roles) != 1 {
		t.Fatalf("compliance groups: %v %+v", err, groups)
	}
}

func TestAnalyticsEndpoints(t *testing.T) {
	srv, admin, _, _ := newClients(t)
	ctx := context.Background()
	ana, err := client.New(client.Config{BaseURL: srv.URL, AnalyticsAPIKey: mock.AnalyticsKey})
	if err != nil {
		t.Fatal(err)
	}
	users, err := ana.ListAnalyticsUsers(ctx, client.AnalyticsListParams{Date: "2026-01-15", GroupBy: []string{"rbac_group_id"}})
	if err != nil || len(users) == 0 || users[0].RBACGroupID == nil || users[0].ClaudeCodeMetrics.ToolActions["edit_tool"].RejectedCount != 1 {
		t.Fatalf("users: %v %+v", err, users)
	}
	if _, err := ana.ListAnalyticsUsers(ctx, client.AnalyticsListParams{Date: "2026-01-15", StartingDate: "2026-01-01"}); !client.IsBadRequest(err) {
		t.Fatalf("date + starting_date must 400: %v", err)
	}
	skills, err := ana.ListAnalyticsSkills(ctx, client.AnalyticsListParams{StartingDate: "2026-01-01", EndingDate: "2026-01-08", GroupBy: []string{"product"}, Filters: []string{"share_status:organization"}})
	if err != nil || len(skills) != 3 || skills[0].Product == nil || *skills[0].EstimatedOverageSpend != "1250" {
		t.Fatalf("skills: %v %+v", err, skills)
	}
	if _, err := ana.ListAnalyticsSkills(ctx, client.AnalyticsListParams{Date: "2026-01-15", Filters: []string{"bogus:x"}}); !client.IsBadRequest(err) {
		t.Fatalf("bad filter dim must 400: %v", err)
	}
	conns, err := ana.ListAnalyticsConnectors(ctx, client.AnalyticsListParams{Date: "2026-01-15"})
	if err != nil || len(conns) != 3 || *conns[0].ReadCallCount != 40 {
		t.Fatalf("connectors: %v", err)
	}
	plugins, err := ana.ListAnalyticsPlugins(ctx, client.AnalyticsListParams{Date: "2026-01-15"})
	if err != nil || len(plugins) != 2 || plugins[1].PluginID != nil {
		t.Fatalf("plugins: %v %+v", err, plugins)
	}
	arts, err := ana.ListAnalyticsArtifacts(ctx, client.AnalyticsListParams{Date: "2026-01-15"})
	if err != nil || len(arts) != 4 {
		t.Fatalf("artifacts: %v", err)
	}
	if _, err := ana.ListAnalyticsArtifacts(ctx, client.AnalyticsListParams{StartingDate: "2026-01-15"}); !client.IsBadRequest(err) {
		t.Fatalf("artifacts requires date: %v", err)
	}
	usage, err := ana.GetAnalyticsUsageReport(ctx, client.AnalyticsReportParams{StartingAt: "2026-01-01T00:00:00Z", EndingAt: "2026-01-04T00:00:00Z", GroupBy: []string{"model", "product"}})
	if err != nil || len(usage.Data) != 3 || usage.DataRefreshedAt == nil || usage.OrganizationID == "" || usage.Data[0].Results[0].Model == nil {
		t.Fatalf("usage report: %v %+v", err, usage)
	}
	if _, err := ana.GetAnalyticsUsageReport(ctx, client.AnalyticsReportParams{StartingAt: "2026-01-01T00:00:00Z", EndingAt: "2026-03-01T00:00:00Z"}); !client.IsBadRequest(err) {
		t.Fatalf("31 day span must 400: %v", err)
	}
	cost, err := ana.GetAnalyticsCostReport(ctx, client.AnalyticsReportParams{StartingAt: "2026-01-01T00:00:00Z", GroupBy: []string{"cost_type"}})
	if err != nil || len(cost.Data) != 3 || cost.Data[0].Results[0].CostType == nil || cost.Data[0].Results[0].Requests != nil {
		t.Fatalf("cost report: %v", err)
	}
	uu, err := ana.ListAnalyticsUserUsage(ctx, client.AnalyticsReportParams{StartingAt: "2026-01-01T00:00:00Z", OrderBy: "total_tokens"})
	if err != nil || len(uu.Data) == 0 || uu.Data[0].TotalTokens == 0 {
		t.Fatalf("user usage: %v", err)
	}
	uc, err := ana.ListAnalyticsUserCost(ctx, client.AnalyticsReportParams{StartingAt: "2026-01-01T00:00:00Z", UserIDs: []string{uu.Data[0].Actor.UserID}})
	if err != nil || len(uc.Data) != 1 || uc.Data[0].Amount == "" {
		t.Fatalf("user cost: %v %d", err, len(uc.Data))
	}
	if _, err := admin.ListAnalyticsSkills(ctx, client.AnalyticsListParams{Date: "2026-01-15"}); !client.IsMissingCredential(err) {
		t.Fatalf("admin key must not satisfy analytics: %v", err)
	}
}
