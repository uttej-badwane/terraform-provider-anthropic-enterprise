package apidrift

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Trimmed from the live api_keys reference. The query parameter and the nested
// scope.workspace_id share a name with the deprecated top-level field, and a
// name-only scan flags all three; only the top-level one is deprecated.
const apiKeysPage = "## Parameters\n\n" +
	"- `workspace_id: optional string`\n\n  Filter by Workspace ID.\n\n" +
	"## Returns\n\n" +
	"- `BetaAPIKey object`\n\n" +
	"  - `id: string`\n\n" +
	"  - `scope: BetaAPIKeyOrganizationScope or BetaAPIKeyWorkspaceScope`\n\n" +
	"    - `BetaAPIKeyWorkspaceScope object`\n\n" +
	"      - `type: \"workspace\"`\n\n" +
	"      - `workspace_id: string`\n\n" +
	"  - `workspace_id: string or null`\n\n" +
	"    **Deprecated**: Use `scope` instead. `workspace_id` is `null` both for an API key in the default Workspace.\n\n" +
	"    Deprecated: use `scope` instead.\n"

// Trimmed from the live rate_limits reference: a list response, a nested
// replacement, and a marker that appears twice for the same field.
const rateLimitsPage = "## Returns\n\n" +
	"- `data: array of BetaOrganizationRateLimit`\n\n" +
	"  - `id: string`\n\n" +
	"  - `group: BetaOrganizationRateLimitModelGroup or BetaOrganizationRateLimitBatchGroup`\n\n" +
	"    - `BetaOrganizationRateLimitModelGroup object`\n\n" +
	"      - `type: \"model_group\"`\n\n" +
	"  - `group_type: \"batch\" or \"files\"`\n\n" +
	"    **Deprecated**: Use `group.type` instead. `group_type` is still returned.\n\n" +
	"    **Deprecated**: Use `group.type` instead.\n\n" +
	"- `next_page: string or null`\n"

const externalKeysPage = "## Returns\n\n" +
	"- `provider_config: object`\n\n" +
	"  - `role_arn: optional string or null`\n\n" +
	"    **Deprecated**\n\n" +
	"    IAM role ARN. Deprecated; this field is ignored.\n"

func TestParseDeprecationsReadsOnlyTheResponse(t *testing.T) {
	t.Parallel()
	got := ParseDeprecations("api_keys", apiKeysPage)
	if len(got) != 1 {
		t.Fatalf("got %d deprecations, want exactly the top-level workspace_id: %+v", len(got), got)
	}
	d := got[0]
	if !slices.Equal(d.Path, []string{"workspace_id"}) {
		t.Errorf("path = %v, want [workspace_id]; scope.workspace_id and the query parameter are not deprecated", d.Path)
	}
	if !slices.Equal(d.Replacement, []string{"scope"}) {
		t.Errorf("replacement = %v, want [scope]", d.Replacement)
	}
}

func TestParseDeprecationsListNestedReplacementAndRepeatedMarker(t *testing.T) {
	t.Parallel()
	got := ParseDeprecations("rate_limits", rateLimitsPage)
	if len(got) != 1 {
		t.Fatalf("got %d deprecations, want 1 despite the repeated marker: %+v", len(got), got)
	}
	if !slices.Equal(got[0].Path, []string{"group_type"}) {
		t.Errorf("path = %v, want [group_type]: the list wrapper must be removed", got[0].Path)
	}
	if !slices.Equal(got[0].Replacement, []string{"group", "type"}) {
		t.Errorf("replacement = %v, want [group type]", got[0].Replacement)
	}
}

func TestParseDeprecationsWithoutReplacement(t *testing.T) {
	t.Parallel()
	got := ParseDeprecations("external_keys", externalKeysPage)
	if len(got) != 1 || got[0].Replacement != nil {
		t.Fatalf("got %+v, want one deprecation with no replacement", got)
	}
	if !strings.Contains(got[0].Note, "ignored") {
		t.Errorf("note %q lost the reference's explanation", got[0].Note)
	}
}

const sampleClient = `package client

import "encoding/json"

type Opt[T any] struct{ v T }

type KeyScope struct {
	Type        string  ` + "`json:\"type\"`" + `
	WorkspaceID *string ` + "`json:\"workspace_id\"`" + `
}

type Dims struct {
	Model string ` + "`json:\"model\"`" + `
}

type APIKey struct {
	Dims
	ID          string          ` + "`json:\"id\"`" + `
	Scope       *KeyScope       ` + "`json:\"scope,omitempty\"`" + `
	WorkspaceID *string         ` + "`json:\"workspace_id\"`" + `
	Tags        []KeyScope      ` + "`json:\"tags\"`" + `
	Maybe       Opt[*KeyScope]  ` + "`json:\"maybe,omitzero\"`" + `
	Raw         json.RawMessage ` + "`json:\"raw\"`" + `
	Hidden      string          ` + "`json:\"-\"`" + `
}
`

func loadSample(t *testing.T) Types {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "types.go"), []byte(sampleClient), 0o600); err != nil {
		t.Fatal(err)
	}
	types, err := LoadTypes(dir)
	if err != nil {
		t.Fatal(err)
	}
	return types
}

func TestLoadTypes(t *testing.T) {
	t.Parallel()
	types := loadSample(t)
	k := types["APIKey"]
	cases := map[string]string{"scope": "KeyScope", "tags": "KeyScope", "maybe": "KeyScope", "raw": "json.RawMessage", "model": "string"}
	for name, want := range cases {
		if got := k[name].Type; got != want {
			t.Errorf("APIKey.%s element type = %q, want %q", name, got, want)
		}
	}
	if _, ok := k["-"]; ok {
		t.Error(`a json:"-" field was recorded`)
	}
	if _, ok := k["model"]; !ok {
		t.Error("the embedded struct's field was not promoted")
	}
}

func TestCheckClassifiesEachCase(t *testing.T) {
	t.Parallel()
	types := loadSample(t)
	m := Manifest{
		Pages:  []PageEntry{{Page: "keys", Type: "APIKey"}},
		Ignore: []IgnoreEntry{{Type: "APIKey", Field: "raw", Reason: "reviewed"}, {Type: "APIKey", Field: "gone", Reason: "stale"}},
	}
	deps := map[string][]Deprecation{"keys": {
		// Both decoded: safe.
		{Path: []string{"workspace_id"}, Replacement: []string{"scope"}},
		// Replacement not decoded: the bug.
		{Path: []string{"id"}, Replacement: []string{"key_id"}},
		// Nested, resolved through the struct field's type.
		{Path: []string{"scope", "type"}, Replacement: []string{"kind"}},
		// No replacement, but ignored with a reason.
		{Path: []string{"raw"}},
		// Not decoded at all: nothing to report.
		{Path: []string{"not_decoded"}, Replacement: []string{"x"}},
		{Path: []string{"missing_parent", "x"}, Replacement: []string{"y"}},
	}}

	r := Check(m, types, deps)

	var migrate []string
	for _, f := range r.Findings {
		if f.Kind != KindMigrate {
			t.Errorf("unexpected finding kind %s for %s.%s", f.Kind, f.Type, f.Field)
		}
		migrate = append(migrate, f.Type+"."+f.Field)
	}
	slices.Sort(migrate)
	if want := []string{"APIKey.id", "KeyScope.type"}; !slices.Equal(migrate, want) {
		t.Errorf("findings = %v, want %v", migrate, want)
	}
	if len(r.Fallbacks) != 1 || r.Fallbacks[0].Field != "workspace_id" {
		t.Errorf("fallbacks = %+v, want only workspace_id", r.Fallbacks)
	}
	if len(r.UnusedIgnores) != 1 || r.UnusedIgnores[0].Field != "gone" {
		t.Errorf("unused ignores = %+v, want only the stale one", r.UnusedIgnores)
	}
}

func TestValidateRejectsBrokenEntries(t *testing.T) {
	t.Parallel()
	types := loadSample(t)
	m := Manifest{
		Pages:  []PageEntry{{Page: "a", Type: "APIKey"}, {Page: "a", Type: "APIKey"}, {Page: "b", Type: "NoSuchType"}},
		Ignore: []IgnoreEntry{{Type: "APIKey", Field: "raw"}},
	}
	if errs := Validate(m, types); len(errs) != 3 {
		t.Errorf("got %d errors, want 3 (duplicate page, unknown type, ignore without a reason): %v", len(errs), errs)
	}
}

func TestParseReleaseNotes(t *testing.T) {
	t.Parallel()
	md := "### September 24, 2026\n\n" +
		"* The Compliance API Activity Feed no longer returns file names.\n" +
		"* Responses from the Messages API now always include `diagnostics`.\n\n" +
		"### September 1, 2026\n\n" +
		"* The `read:compliance_org_settings` scope has been retired.\n"
	since := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	got := ParseReleaseNotes(md, since)
	if len(got) != 1 || !strings.Contains(got[0].Text, "Activity Feed") {
		t.Fatalf("got %+v, want only the in-window note about something the provider manages", got)
	}
}

// The manifest is only as good as its type names. A renamed client struct
// would make its entry check nothing, silently, so this runs on every pull
// request rather than waiting for the weekly job.
func TestManifestMatchesTheClient(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	types, err := LoadTypes(filepath.Join("..", "client"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range Validate(m, types) {
		t.Error(e)
	}
}

// The deprecations fixed in #76, as the reference publishes them. Against the
// real client they must be fallbacks: reverting any of them to reading only
// the deprecated field turns this red.
func TestKnownDeprecationsStayFixed(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	types, err := LoadTypes(filepath.Join("..", "client"))
	if err != nil {
		t.Fatal(err)
	}
	deps := map[string][]Deprecation{
		"beta/organization/rbac_groups/retrieve":        {{Path: []string{"roles"}, Replacement: []string{"role_ids"}}},
		"beta/organization/rbac_groups/members/list":    {{Path: []string{"group_id"}, Replacement: []string{"rbac_group_id"}}},
		"beta/organization/rate_limits/list":            {{Path: []string{"group_type"}, Replacement: []string{"group", "type"}}},
		"beta/organization/workspaces/rate_limits/list": {{Path: []string{"group_type"}, Replacement: []string{"group", "type"}}},
		"beta/organization/api_keys/retrieve":           {{Path: []string{"workspace_id"}, Replacement: []string{"scope"}}},
		"beta/organization/external_keys/retrieve":      {{Path: []string{"provider_config", "role_arn"}}},
	}
	r := Check(m, types, deps)
	for _, f := range r.Findings {
		t.Errorf("%s.%s is decoded without its replacement %q", f.Type, f.Field, f.Replacement)
	}
	if len(r.Fallbacks) != 5 {
		t.Errorf("got %d fallbacks, want 5", len(r.Fallbacks))
	}
}
