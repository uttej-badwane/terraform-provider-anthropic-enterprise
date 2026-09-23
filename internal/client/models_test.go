package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// The models endpoints are inference-scoped. Against the live API an org:admin
// OAuth token is refused with "does not meet scope requirement
// any_of(user:inference, workspace:inference, ...)", so these must go out on
// the workspace key in X-Api-Key and never as a bearer token.
//
// This is worth pinning because the provider spans several APIs that each
// accept a different credential, and the official reference for a neighbouring
// endpoint documents the wrong one. Routing is not something to infer.
func TestListModelsUsesTheInferenceCredential(t *testing.T) {
	t.Parallel()

	const workspaceKey = "sk-ant-api03-workspace-test" //nolint:gosec // fixture
	const adminKey = "sk-ant-admin-test"               //nolint:gosec // fixture

	var gotAPIKey, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"next_page":null}`))
	}))
	defer srv.Close()

	// Both credentials configured: the client must still pick the workspace key.
	c, err := client.New(client.Config{BaseURL: srv.URL, APIKey: workspaceKey, AdminAPIKey: adminKey})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if _, err := c.ListModels(context.Background()); err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if gotAPIKey != workspaceKey {
		t.Errorf("X-Api-Key = %q, want the workspace key", gotAPIKey)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want it unset: this endpoint rejects bearer tokens", gotAuth)
	}
}

// A model with no api_key configured must fail before a request goes out, with
// a message naming the attribute to set, rather than surfacing a bare 401.
func TestListModelsWithoutAPIKeyFailsBeforeRequest(t *testing.T) {
	t.Parallel()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer srv.Close()

	c, err := client.New(client.Config{BaseURL: srv.URL, AdminAPIKey: "sk-ant-admin-test"}) //nolint:gosec // fixture
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if _, err := c.ListModels(context.Background()); err == nil {
		t.Fatal("ListModels succeeded without an api_key; expected a missing-credential error")
	}
	if called {
		t.Error("a request was sent without the required credential")
	}
}

// Capabilities are not a flat map: context_management and effort nest further
// sub-capabilities under their own supported flag. The flat view must report
// the top-level flag of a nesting capability rather than skipping it.
func TestSupportedCapabilitiesFlattensOnlyTheTopLevel(t *testing.T) {
	t.Parallel()

	m := client.Model{Capabilities: json.RawMessage(`{
      "batch": {"supported": true},
      "citations": {"supported": false},
      "context_management": {
        "supported": true,
        "compact_20260112": {"supported": false}
      },
      "malformed": 42,
      "no_supported_key": {"other": true}
    }`)}

	got, err := m.SupportedCapabilities()
	if err != nil {
		t.Fatalf("SupportedCapabilities: %v", err)
	}

	want := map[string]bool{"batch": true, "citations": false, "context_management": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("capability %q = %v, want %v", k, got[k], v)
		}
	}
	// Entries that cannot be read are omitted rather than guessed at.
	if _, ok := got["malformed"]; ok {
		t.Error("a non-object capability should be omitted, not defaulted")
	}
	if _, ok := got["no_supported_key"]; ok {
		t.Error("a capability with no supported key should be omitted, not defaulted")
	}
}
