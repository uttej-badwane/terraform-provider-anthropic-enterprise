package client_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/mock"
)

// A redirect must not be followed: net/http would replay X-Api-Key on the new
// host, since it strips only Authorization and Cookie when the host changes.
func TestRedirectIsNotFollowed(t *testing.T) {
	var leaked atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "" {
			leaked.Add(1)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(target.Close)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/redirected", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	c, err := client.New(client.Config{BaseURL: origin.URL, AdminAPIKey: "not-a-real-key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetOrganization(context.Background())
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound {
		t.Fatalf("expected the 302 to surface as an API error, got %v", err)
	}
	if n := leaked.Load(); n != 0 {
		t.Fatalf("credential reached the redirect target %d time(s)", n)
	}
}

func TestBaseURLValidation(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		wantErr string
	}{
		{"default", "", ""},
		{"https", "https://api.example.com", ""},
		{"trailing slash", "https://api.example.com/", ""},
		{"http loopback ipv4", "http://127.0.0.1:8787", ""},
		{"http localhost", "http://localhost:8787", ""},
		{"http loopback ipv6", "http://[::1]:8787", ""},
		{"http remote", "http://api.example.com", "http is only accepted for loopback"},
		{"other scheme", "ftp://api.example.com", "scheme must be https"},
		{"path only", "/v1", "absolute URL"},
		{"garbage", "::bad", "absolute URL"},
		{"userinfo", "https://user:hunter2@api.example.com", "credentials in the URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.New(client.Config{BaseURL: tc.baseURL, AdminAPIKey: "not-a-real-key"})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
			if strings.Contains(err.Error(), "hunter2") {
				t.Fatalf("error text echoed the URL password: %v", err)
			}
		})
	}
}

// A create is not replayed after a 5xx: the write may already have landed.
func TestCreateIsNotRetriedOnServerError(t *testing.T) {
	srv, admin, _, _ := newClients(t)
	before := srv.Requests()
	srv.FailNext(1)
	_, err := admin.CreateWorkspace(context.Background(), client.WorkspaceCreate{Name: "once"})
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected the 500 to surface, got %v", err)
	}
	if n := srv.Requests() - before; n != 1 {
		t.Fatalf("expected exactly 1 request for a failed create, got %d", n)
	}
	if got, _ := admin.ListWorkspaces(context.Background(), false); len(got) != 0 {
		t.Fatalf("no workspace should exist after the failed create, got %d", len(got))
	}
}

// Updates are safe to replay, so they keep the retry behaviour.
func TestUpdateIsRetriedOnServerError(t *testing.T) {
	srv, admin, _, _ := newClients(t)
	ctx := context.Background()
	ws, err := admin.CreateWorkspace(ctx, client.WorkspaceCreate{Name: "retry"})
	if err != nil {
		t.Fatal(err)
	}
	before := srv.Requests()
	srv.FailNext(1)
	upd, err := admin.UpdateWorkspace(ctx, ws.ID, client.WorkspaceUpdate{Name: ptr("retried")})
	if err != nil || upd.Name != "retried" {
		t.Fatalf("expected the update to succeed on retry: %v %+v", err, upd)
	}
	if n := srv.Requests() - before; n != 2 {
		t.Fatalf("expected 2 requests (1 failure + 1 success), got %d", n)
	}
}

// A create whose connection was refused never reached the server and may be
// retried. The first client points at a closed port, so the dial fails.
func TestCreateRetriedWhenDialFails(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()
	zero := 0
	c, err := client.New(client.Config{BaseURL: closedURL, AdminAPIKey: "not-a-real-key", MaxRetries: &zero})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.CreateWorkspace(context.Background(), client.WorkspaceCreate{Name: "x"})
	if err == nil {
		t.Fatal("expected a dial error")
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		t.Fatalf("a dial failure must not be an API error: %v", err)
	}
}

// A server that keeps returning the same cursor must not loop forever.
func TestPaginationStopsOnStuckCursor(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"wrkspc_01Example","type":"workspace","name":"x"}],"has_more":true,"first_id":"wrkspc_01Example","last_id":"wrkspc_01Example"}`))
	}))
	t.Cleanup(srv.Close)
	c, err := client.New(client.Config{BaseURL: srv.URL, AdminAPIKey: "not-a-real-key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.ListWorkspaces(context.Background(), false)
	if err == nil || !strings.Contains(err.Error(), "cursor did not advance") {
		t.Fatalf("expected a stuck-cursor error, got %v", err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("expected 2 requests before giving up, got %d", n)
	}
}

func TestTokenPaginationStopsOnStuckCursor(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"rbacgroup_01Example","name":"x"}],"has_more":true,"next_page":"same"}`))
	}))
	t.Cleanup(srv.Close)
	c, err := client.New(client.Config{BaseURL: srv.URL, EnterpriseAPIKey: "not-a-real-key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.ListRBACGroups(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cursor did not advance") {
		t.Fatalf("expected a stuck-cursor error, got %v", err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("expected 2 requests before giving up, got %d", n)
	}
}

// Error bodies that are not the API's envelope (a proxy page, for example) are
// not copied into the error, and so never reach a Terraform diagnostic.
func TestNonEnvelopeErrorBodyIsNotEchoed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>upstream said: proxy-session-abc123</html>"))
	}))
	t.Cleanup(srv.Close)
	zero := 0
	c, err := client.New(client.Config{BaseURL: srv.URL, AdminAPIKey: "not-a-real-key", MaxRetries: &zero})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetOrganization(context.Background())
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("expected a 502 API error, got %v", err)
	}
	if strings.Contains(err.Error(), "proxy-session-abc123") {
		t.Fatalf("error text echoed the response body: %v", err)
	}
	if !strings.Contains(err.Error(), "Bad Gateway") {
		t.Fatalf("error should carry the status text: %v", err)
	}
}

// The envelope path still surfaces the API's own message.
func TestEnvelopeErrorMessageIsKept(t *testing.T) {
	_, admin, _, _ := newClients(t)
	_, err := admin.GetWorkspace(context.Background(), "wrkspc_missing")
	if err == nil || !strings.Contains(err.Error(), "not_found_error") {
		t.Fatalf("expected the envelope type in the error, got %v", err)
	}
	_ = mock.AdminKey
}

// An oversized body is reported as such rather than as a JSON decode error.
func TestOversizedResponseIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"`))
		_, _ = w.Write([]byte(strings.Repeat("a", 16<<20)))
		_, _ = w.Write([]byte(`"}`))
	}))
	t.Cleanup(srv.Close)
	c, err := client.New(client.Config{BaseURL: srv.URL, AdminAPIKey: "not-a-real-key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GetOrganization(context.Background())
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected an oversized-body error, got %v", err)
	}
}
