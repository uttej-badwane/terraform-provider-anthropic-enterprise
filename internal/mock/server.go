// Package mock is an in-memory implementation of the Anthropic Admin API used
// by unit and acceptance tests. It enforces credential classes the same way
// the real API does so routing bugs surface in tests.
package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
)

// Credentials accepted by the mock.
const (
	AdminKey      = "sk-ant-admin-test-0000"
	OAuthToken    = "test-org-admin-oauth-token" //nolint:gosec // fixture accepted only by this mock
	EnterpriseKey = "sk-ant-api01-enterprise-test-0000"
)

// Server is a running mock Admin API.
type Server struct {
	*httptest.Server
	mu  sync.Mutex
	seq int
	mux *http.ServeMux

	// OrgID / OrgName are returned by GET /v1/organizations/me.
	OrgID   string
	OrgName string

	store *store

	requests int
	failNext int
}

// NewServer starts a mock on a random loopback port.
func NewServer() *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		OrgID:   "11111111-2222-3333-4444-555555555555",
		OrgName: "Example Organization",
		store:   newStore(),
	}
	s.seed()
	s.routes()
	s.Server = httptest.NewServer(s.mux)
	return s
}

func (s *Server) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s_%012d", prefix, s.seq)
}

type credentialKind int

const (
	credNone credentialKind = iota
	credAdminKey
	credOAuth
	credEnterprise
	credScopedOther
)

func (s *Server) credential(r *http.Request) credentialKind {
	if v := r.Header.Get("Authorization"); strings.HasPrefix(v, "Bearer ") {
		if strings.TrimPrefix(v, "Bearer ") == OAuthToken {
			return credOAuth
		}
		return credNone
	}
	switch r.Header.Get("X-Api-Key") {
	case AdminKey:
		return credAdminKey
	case EnterpriseKey:
		return credEnterprise
	case ComplianceKey, AnalyticsKey, AgentsKey:
		return credScopedOther
	}
	return credNone
}

// requireAdmin accepts an Admin API key or an OAuth token.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Anthropic-Version") == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "anthropic-version header is required")
		return false
	}
	switch s.credential(r) {
	case credAdminKey, credOAuth:
		return true
	case credEnterprise:
		writeError(w, http.StatusBadRequest, "invalid_request_error", "this endpoint is not supported for this organization type")
		return false
	}
	writeError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
	return false
}

// requireOAuth accepts only an OAuth token.
func (s *Server) requireOAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.credential(r) == credOAuth {
		return true
	}
	writeError(w, http.StatusUnauthorized, "authentication_error", "this endpoint requires an org:admin OAuth token")
	return false
}

// requireEnterprise accepts only the Enterprise scoped key.
func (s *Server) requireEnterprise(w http.ResponseWriter, r *http.Request) bool {
	switch s.credential(r) {
	case credEnterprise:
		return true
	case credAdminKey, credOAuth:
		writeError(w, http.StatusBadRequest, "invalid_request_error", "this endpoint is not supported for this organization type")
		return false
	}
	writeError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
	return false
}

// requireAdminOrEnterprise is for users and invites, which exist on both surfaces.
func (s *Server) requireAdminOrEnterprise(w http.ResponseWriter, r *http.Request) bool {
	if s.credential(r) != credNone {
		return true
	}
	writeError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
	return false
}

func writeError(w http.ResponseWriter, status int, typ, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("request-id", "req_mock")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]string{"type": typ, "message": msg},
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("request-id", "req_mock")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func notFound(w http.ResponseWriter, what string) {
	writeError(w, http.StatusNotFound, "not_found_error", what+" not found")
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func limitParam(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if n <= 0 || n > 1000 {
		n = 20
	}
	return n
}

// cursorList writes a has_more / first_id / last_id envelope over items,
// which must already be ordered. ids returns the id for an item.
func cursorList[T any](w http.ResponseWriter, r *http.Request, items []T, id func(T) string) {
	limit := limitParam(r)
	start := 0
	if after := r.URL.Query().Get("after_id"); after != "" {
		for i, it := range items {
			if id(it) == after {
				start = i + 1
				break
			}
		}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := items[start:end]
	out := map[string]any{"data": page, "has_more": end < len(items), "first_id": nil, "last_id": nil}
	if len(page) > 0 {
		out["first_id"] = id(page[0])
		out["last_id"] = id(page[len(page)-1])
	}
	writeJSON(w, out)
}

// tokenList writes a next_page envelope; the token is an offset.
func tokenList[T any](w http.ResponseWriter, r *http.Request, items []T) {
	limit := limitParam(r)
	start, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if start < 0 || start > len(items) {
		start = 0
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	out := map[string]any{"data": items[start:end], "has_more": end < len(items), "next_page": nil}
	if end < len(items) {
		out["next_page"] = strconv.Itoa(end)
	}
	writeJSON(w, out)
}

func deleted(w http.ResponseWriter, id, typ string) {
	writeJSON(w, map[string]string{"id": id, "type": typ})
}

// handle registers a method+pattern route (Go 1.22 pattern syntax).
func (s *Server) handle(pattern string, fn func(http.ResponseWriter, *http.Request)) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.requests++
		if s.failNext > 0 {
			s.failNext--
			writeError(w, http.StatusInternalServerError, "api_error", "injected failure")
			return
		}
		fn(w, r)
	})
}

// Reset clears all stored objects (tests may call between cases).
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = newStore()
	s.seed()
}
