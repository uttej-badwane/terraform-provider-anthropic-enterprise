package client_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/mock"
)

func agentClients(t *testing.T) (*mock.Server, *client.Client, *client.Client) {
	t.Helper()
	srv := mock.NewServer()
	t.Cleanup(srv.Close)
	api, err := client.New(client.Config{BaseURL: srv.URL, APIKey: mock.AgentsKey, WorkspaceID: "wrkspc_test"})
	if err != nil {
		t.Fatal(err)
	}
	admin, _ := client.New(client.Config{BaseURL: srv.URL, AdminAPIKey: mock.AdminKey})
	return srv, api, admin
}

func TestAgentLifecycleAndNormalization(t *testing.T) {
	_, api, admin := agentClients(t)
	ctx := context.Background()

	if _, err := admin.ListAgents(ctx, false); !client.IsMissingCredential(err) {
		t.Fatalf("admin key must not satisfy CredAPIKey: %v", err)
	}

	a, err := api.CreateAgent(ctx, client.AgentCreate{
		Name:       "reviewer",
		Model:      client.ModelParams{ID: "claude-opus-5"},
		Tools:      json.RawMessage(`[{"type":"agent_toolset_20260401","configs":[{"name":"bash","permission_policy":{"type":"always_allow"}}]},{"type":"mcp_toolset","mcp_server_name":"docs"}]`),
		MCPServers: []client.MCPServer{{Name: "docs", URL: "https://mcp.example.com/sse"}},
		Skills:     []client.SkillRef{{Type: "anthropic", SkillID: "xlsx"}},
		Metadata:   map[string]string{"team": "platform"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.Version != 1 || a.Model.Effort == nil || a.Model.Effort.Type != "high" || *a.Model.Speed != "standard" {
		t.Fatalf("model not normalized: %+v", a.Model)
	}
	if *a.Skills[0].Version != "20251013" {
		t.Fatalf("skill version not resolved: %+v", a.Skills)
	}
	var tools []struct {
		DefaultConfig struct {
			Enabled bool `json:"enabled"`
		} `json:"default_config"`
		Configs []struct {
			Type             string `json:"type"`
			Enabled          bool   `json:"enabled"`
			PermissionPolicy struct {
				Type string `json:"type"`
			} `json:"permission_policy"`
		} `json:"configs"`
	}
	if err := json.Unmarshal(a.Tools, &tools); err != nil || len(tools) != 2 || len(tools[0].Configs) != 1 {
		t.Fatalf("tools shape: %v %s", err, a.Tools)
	}
	if !tools[0].DefaultConfig.Enabled || tools[0].Configs[0].Type != "bash" || !tools[0].Configs[0].Enabled || tools[0].Configs[0].PermissionPolicy.Type != "always_allow" {
		t.Fatalf("tools not normalized: %s", a.Tools)
	}

	// No-op update keeps the version.
	same, err := api.UpdateAgent(ctx, a.ID, client.AgentUpdate{Name: ptr("reviewer")})
	if err != nil || same.Version != 1 {
		t.Fatalf("no-op update changed version: %v %d", err, same.Version)
	}
	// Effective update bumps the version.
	v2, err := api.UpdateAgent(ctx, a.ID, client.AgentUpdate{System: client.Some("Be terse."), Metadata: client.MetadataPatch{"team": nil, "owner": ptr("infra")}})
	if err != nil || v2.Version != 2 || v2.Metadata["team"] != "" || v2.Metadata["owner"] != "infra" {
		t.Fatalf("update failed: %v %+v", err, v2)
	}
	if _, err := api.UpdateAgent(ctx, a.ID, client.AgentUpdate{Name: ptr("x"), Version: ptr(int64(1))}); !client.IsConflict(err) {
		t.Fatalf("stale version must 409: %v", err)
	}
	versions, err := api.ListAgentVersions(ctx, a.ID)
	if err != nil || len(versions) != 2 || versions[0].Version != 2 {
		t.Fatalf("versions: %v %d", err, len(versions))
	}
	// Unreferenced MCP server -> 400.
	if _, err := api.UpdateAgent(ctx, a.ID, client.AgentUpdate{MCPServers: &[]client.MCPServer{{Name: "docs", URL: "https://x"}, {Name: "orphan", URL: "https://y"}}}); !client.IsBadRequest(err) {
		t.Fatalf("orphan mcp server must 400: %v", err)
	}
	if _, err := api.ArchiveAgent(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := api.UpdateAgent(ctx, a.ID, client.AgentUpdate{Name: ptr("y")}); !client.IsBadRequest(err) {
		t.Fatalf("archived update must 400: %v", err)
	}
	live, _ := api.ListAgents(ctx, false)
	all, _ := api.ListAgents(ctx, true)
	if len(live) != 0 || len(all) != 1 {
		t.Fatalf("archived listing: %d %d", len(live), len(all))
	}
}

func TestAgentBetaHeaderEnforcement(t *testing.T) {
	srv, api, _ := agentClients(t)
	ctx := context.Background()
	// Memory stores need their own header; the client sends it, so the happy path works...
	ms, err := api.CreateMemoryStore(ctx, client.MemoryStoreCreate{Name: "notes"})
	if err != nil || ms.Description != "" {
		t.Fatalf("memory store: %v", err)
	}
	// ...and a raw request with the wrong header is rejected by the mock.
	req, _ := newRawRequest(srv.URL+"/v1/memory_stores", mock.AgentsKey, client.BetaManagedAgents)
	if req.StatusCode != 400 {
		t.Fatalf("wrong beta must 400, got %d", req.StatusCode)
	}
	req, _ = newRawRequest(srv.URL+"/v1/agents", mock.AgentsKey, "")
	if req.StatusCode != 400 {
		t.Fatalf("missing beta must 400, got %d", req.StatusCode)
	}
	req, _ = newRawRequest(srv.URL+"/v1/agents", mock.AdminKey, client.BetaManagedAgents)
	if req.StatusCode != 401 {
		t.Fatalf("admin key must 401, got %d", req.StatusCode)
	}
	if got := req.Header.Get("anthropic-workspace-id"); got != "" {
		t.Fatalf("unexpected workspace echo on failure: %s", got)
	}
}

func TestEnvironmentVaultCredential(t *testing.T) {
	_, api, _ := agentClients(t)
	ctx := context.Background()
	env, err := api.CreateEnvironment(ctx, client.EnvironmentCreate{Name: "ci", Config: client.EnvironmentConfig{Type: "cloud",
		Networking: &client.Networking{Type: "limited", AllowedHosts: []string{"*.example.com"}, AllowPackageManagers: ptr(true)},
		Packages:   &client.Packages{Pip: []string{"requests"}}}})
	if err != nil || env.Config.Packages == nil || len(env.Config.Packages.Apt) != 0 || env.Config.Networking.AllowMCPServers == nil {
		t.Fatalf("environment: %v %+v", err, env)
	}
	if _, err := api.CreateEnvironment(ctx, client.EnvironmentCreate{Name: "ci", Config: client.EnvironmentConfig{Type: "cloud"}}); !client.IsConflict(err) {
		t.Fatalf("duplicate env name must 409: %v", err)
	}
	if _, err := api.CreateEnvironment(ctx, client.EnvironmentCreate{Name: "bad", Config: client.EnvironmentConfig{Type: "cloud",
		Networking: &client.Networking{Type: "limited"}, Packages: &client.Packages{Npm: []string{"left-pad"}}}}); !client.IsBadRequest(err) {
		t.Fatalf("packages without allow_package_managers must 400: %v", err)
	}
	upd, err := api.UpdateEnvironment(ctx, env.ID, client.EnvironmentUpdate{Config: &client.EnvironmentConfig{Networking: &client.Networking{AllowMCPServers: ptr(true)}}})
	if err != nil || len(upd.Config.Networking.AllowedHosts) != 1 || !*upd.Config.Networking.AllowMCPServers {
		t.Fatalf("partial merge lost fields: %v %+v", err, upd.Config.Networking)
	}

	v, err := api.CreateVault(ctx, client.VaultCreate{DisplayName: "ci secrets"})
	if err != nil {
		t.Fatal(err)
	}
	cred, err := api.CreateVaultCredential(ctx, v.ID, client.VaultCredentialCreate{DisplayName: ptr("gh"),
		Auth: json.RawMessage(`{"type":"environment_variable","secret_name":"GH_TOKEN","secret_value":"s3cret","networking":{"type":"limited","allowed_hosts":["api.github.com"]}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cred.Auth), "s3cret") || !strings.Contains(string(cred.Auth), `"header":true`) {
		t.Fatalf("secret leaked or injection_location not resolved: %s", cred.Auth)
	}
	if _, err := api.CreateVaultCredential(ctx, v.ID, client.VaultCredentialCreate{Auth: json.RawMessage(`{"type":"environment_variable","secret_name":"GH_TOKEN","secret_value":"x","networking":{"type":"unrestricted"}}`)}); !client.IsConflict(err) {
		t.Fatalf("duplicate secret_name must 409: %v", err)
	}
	if _, err := api.UpdateVaultCredential(ctx, v.ID, cred.ID, client.VaultCredentialUpdate{Auth: json.RawMessage(`{"type":"environment_variable","secret_name":"OTHER","secret_value":"y"}`)}); !client.IsBadRequest(err) {
		t.Fatalf("secret_name change must 400: %v", err)
	}
	rot, err := api.UpdateVaultCredential(ctx, v.ID, cred.ID, client.VaultCredentialUpdate{Auth: json.RawMessage(`{"type":"environment_variable","secret_value":"new"}`)})
	if err != nil || !strings.Contains(string(rot.Auth), `"secret_name":"GH_TOKEN"`) {
		t.Fatalf("rotation failed: %v %s", err, rot.Auth)
	}
	if err := api.DeleteVaultCredential(ctx, v.ID, cred.ID); err != nil {
		t.Fatal(err)
	}
	if err := api.DeleteVault(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDeploymentAndSkills(t *testing.T) {
	_, api, _ := agentClients(t)
	ctx := context.Background()
	a, _ := api.CreateAgent(ctx, client.AgentCreate{Name: "nightly", Model: client.ModelParams{ID: "claude-sonnet-5"}})
	env, _ := api.CreateEnvironment(ctx, client.EnvironmentCreate{Name: "prod", Config: client.EnvironmentConfig{Type: "cloud"}})
	d, err := api.CreateDeployment(ctx, client.DeploymentCreate{Name: "nightly-report", Agent: client.AgentRef{Type: "agent", ID: a.ID}, EnvironmentID: env.ID,
		InitialEvents: json.RawMessage(`[{"type":"user.message","content":[{"type":"text","text":"Summarize yesterday."}]}]`),
		Schedule:      &client.Schedule{Type: "cron", Expression: "0 6 * * *", Timezone: "UTC"},
		Resources:     json.RawMessage(`[{"type":"github_repository","url":"https://github.com/example-org/example-repo","authorization_token":"ghp_secret"}]`)})
	if err != nil {
		t.Fatal(err)
	}
	if d.Agent.Version == nil || *d.Agent.Version != 1 || d.Status != "active" || len(d.UpcomingRuns) != 3 || strings.Contains(string(d.Resources), "ghp_secret") {
		t.Fatalf("deployment: %+v", d)
	}
	if _, err := api.CreateDeployment(ctx, client.DeploymentCreate{Name: "bad", Agent: client.AgentRef{Type: "agent", ID: a.ID}, EnvironmentID: env.ID,
		InitialEvents: json.RawMessage(`[]`)}); !client.IsBadRequest(err) {
		t.Fatalf("empty initial_events must 400: %v", err)
	}
	p, err := api.PauseDeployment(ctx, d.ID)
	if err != nil || p.Status != "paused" || p.PausedReason.Type != "manual" || len(p.UpcomingRuns) != 0 {
		t.Fatalf("pause: %v %+v", err, p)
	}
	if u, err := api.UnpauseDeployment(ctx, d.ID); err != nil || u.Status != "active" {
		t.Fatalf("unpause: %v", err)
	}
	cleared, err := api.UpdateDeployment(ctx, d.ID, client.DeploymentUpdate{Schedule: client.Null[*client.Schedule]()})
	if err != nil || cleared.Schedule != nil {
		t.Fatalf("schedule clear: %v %+v", err, cleared.Schedule)
	}
	if _, err := api.ArchiveAgent(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := api.GetDeployment(ctx, d.ID)
	if got.ArchivedAt == nil {
		t.Fatal("archiving the agent must archive its deployments")
	}

	// Skills: multipart create, version, latest alias, delete.
	files := []client.SkillFile{
		{Path: "release-notes/SKILL.md", Data: []byte("---\nname: release-notes\ndescription: Draft release notes from a changelog.\n---\n# Release notes\n")},
		{Path: "release-notes/template.md", Data: []byte("## {{version}}\n")},
	}
	sk, err := api.CreateSkill(ctx, nil, files)
	if err != nil || sk.DisplayName != "release-notes" || sk.Source.Type != "custom" || sk.LatestVersionID == "" {
		t.Fatalf("skill create: %v %+v", err, sk)
	}
	if _, err := api.CreateSkill(ctx, nil, []client.SkillFile{{Path: "x/README.md", Data: []byte("no skill md")}}); !client.IsBadRequest(err) {
		t.Fatalf("missing SKILL.md must 400: %v", err)
	}
	files[1].Data = []byte("## {{version}} changed\n")
	v2, err := api.CreateSkillVersion(ctx, sk.ID, files)
	if err != nil || v2.ID == sk.LatestVersionID {
		t.Fatalf("skill version: %v", err)
	}
	latest, err := api.GetSkillVersion(ctx, sk.ID, "latest")
	if err != nil || latest.ID != v2.ID {
		t.Fatalf("latest alias: %v", err)
	}
	all, _ := api.ListSkills(ctx, "")
	anth, _ := api.ListSkills(ctx, "anthropic")
	if len(all) != 3 || len(anth) != 2 {
		t.Fatalf("skill listing: %d %d", len(all), len(anth))
	}
	if err := api.DeleteSkill(ctx, sk.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := api.GetSkill(ctx, sk.ID); !client.IsNotFound(err) {
		t.Fatalf("deleted skill must 404: %v", err)
	}
}
