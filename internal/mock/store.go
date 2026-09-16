package mock

import (
	"regexp"
	"time"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

type store struct {
	users            []*client.User
	invites          []*client.Invite
	workspaces       []*client.Workspace
	members          []*client.WorkspaceMember
	saMembers        []*client.ServiceAccountWorkspaceMember
	apiKeys          []*client.APIKey
	rateLimits       []client.RateLimit
	wsRateLimits     map[string][]client.WorkspaceRateLimit
	serviceAccounts  []*client.ServiceAccount
	issuers          []*client.FederationIssuer
	rules            []*client.FederationRule
	ruleWorkspaces   []*client.FederationRuleWorkspace
	externalKeys     []*client.ExternalKey
	groups           []*client.RBACGroup
	groupMembers     []*client.RBACGroupMember
	roles            []*client.RBACRole
	spendLimits      []*client.SpendLimit
	increaseRequests []*client.SpendLimitIncreaseRequest
	compliance       client.ComplianceSettings
	defaultWorkspace string
	agentsStore      *agentStore
}

func newStore() *store {
	return &store{
		wsRateLimits: map[string][]client.WorkspaceRateLimit{},
		compliance:   client.ComplianceSettings{State: client.ComplianceState{Type: "enabled"}, Type: "compliance_settings"},
		agentsStore:  newAgentStore(),
	}
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func ptr[T any](v T) *T { return &v }

var slugRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// seed populates the mock with a baseline that resembles a real organization:
// existing members, a default workspace, one API key, rate limits, RBAC roles.
func (s *Server) seed() {
	st := s.store
	for _, u := range []struct{ email, name, role string }{
		{"owner@example.com", "Org Owner", "primary_owner"},
		{"admin@example.com", "Org Admin", "admin"},
		{"dev@example.com", "Dev One", "developer"},
		{"user@example.com", "User One", "user"},
		{"scim@example.com", "SCIM User", "managed"},
	} {
		st.users = append(st.users, &client.User{ID: s.nextID("user"), AddedAt: now(), Email: u.email, Name: u.name, Role: u.role, Type: "user"})
	}
	st.defaultWorkspace = s.nextID("wrkspc")
	st.apiKeys = append(st.apiKeys, &client.APIKey{
		ID: s.nextID("apikey"), CreatedAt: now(),
		CreatedBy:      &client.ActorRef{ID: st.users[2].ID, Type: "user"},
		Name:           "Developer Key",
		PartialKeyHint: ptr("sk-ant-api03-R2D...igAA"),
		Principal:      &client.Principal{Type: "user_actor", UserID: &st.users[2].ID},
		Scope:          client.KeyScope{Type: "workspace", WorkspaceID: &st.defaultWorkspace},
		Status:         "active", Type: "api_key", WorkspaceID: nil,
	})
	st.rateLimits = []client.RateLimit{
		{ID: "rl_model_group_sonnet", GroupType: "model_group", Type: "rate_limit", Models: []string{"claude-sonnet-5"},
			Limits: []client.RateLimitValue{{Type: "requests_per_minute", Value: 4000}, {Type: "input_tokens_per_minute", Value: 2000000}}},
		{ID: "rl_batch", GroupType: "batch", Type: "rate_limit",
			Limits: []client.RateLimitValue{{Type: "requests_per_minute", Value: 1000}}},
	}
	st.roles = []*client.RBACRole{
		{ID: s.nextID("rbac_role"), CreatedAt: now(), UpdatedAt: now(), Name: "Member", Type: "rbac_role"},
		{ID: s.nextID("rbac_role"), CreatedAt: now(), UpdatedAt: now(), Name: "Project Editor", Type: "rbac_role"},
	}
	st.groups = append(st.groups, &client.RBACGroup{ID: s.nextID("rbac_group"), CreatedAt: now(), UpdatedAt: now(),
		Name: "SCIM Synced", Roles: []string{st.roles[0].ID}, SourceType: "scim", Type: "rbac_group"})
	s.seedIncreaseRequests()
	s.seedAgents()
}

// --- exported test helpers ------------------------------------------------

// Users returns the seeded users (copy of the slice).
func (s *Server) Users() []client.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]client.User, 0, len(s.store.users))
	for _, u := range s.store.users {
		out = append(out, *u)
	}
	return out
}

// UserByEmail returns a seeded user.
func (s *Server) UserByEmail(email string) *client.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.store.users {
		if u.Email == email {
			c := *u
			return &c
		}
	}
	return nil
}

// APIKeys returns the seeded API key records.
func (s *Server) APIKeys() []client.APIKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]client.APIKey, 0, len(s.store.apiKeys))
	for _, k := range s.store.apiKeys {
		out = append(out, *k)
	}
	return out
}

// AcceptInvite simulates the invitee accepting: the invite becomes accepted
// and a user is created.
func (s *Server) AcceptInvite(id string) *client.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, inv := range s.store.invites {
		if inv.ID == id && inv.Status == "pending" {
			inv.Status = "accepted"
			inv.AcceptedAt = ptr(now())
			u := &client.User{ID: s.nextID("user"), AddedAt: now(), Email: inv.Email, Name: inv.Email, Role: inv.Role, Type: "user"}
			s.store.users = append(s.store.users, u)
			c := *u
			return &c
		}
	}
	return nil
}

// ExpireInvite marks a pending invite expired.
func (s *Server) ExpireInvite(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, inv := range s.store.invites {
		if inv.ID == id {
			inv.Status = "expired"
		}
	}
}

// SCIMGroupID returns the seeded SCIM-sourced group id.
func (s *Server) SCIMGroupID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.store.groups {
		if g.SourceType == "scim" {
			return g.ID
		}
	}
	return ""
}

// SeedWorkspaceRateLimit adds a workspace override row.
func (s *Server) SeedWorkspaceRateLimit(workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store.wsRateLimits[workspaceID] = append(s.store.wsRateLimits[workspaceID], client.WorkspaceRateLimit{
		GroupType: "model_group", Models: []string{"claude-sonnet-5"}, RateLimitID: "rl_model_group_sonnet",
		Type: "workspace_rate_limit", WorkspaceID: workspaceID,
		Limits: []client.RateLimitValue{{Type: "requests_per_minute", Value: 1000, OrgLimit: ptr(int64(4000))}},
	})
}

// ArchiveWorkspaceOutOfBand archives a workspace as if done in the Console.
func (s *Server) ArchiveWorkspaceOutOfBand(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.store.workspaces {
		if w.ID == id {
			w.ArchivedAt = ptr(now())
		}
	}
}

// Requests counts handled requests (for retry tests).
func (s *Server) Requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

// FailNext makes the next n requests return HTTP 500 (retry tests).
func (s *Server) FailNext(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failNext = n
}

// Workspaces returns all workspaces including archived ones.
func (s *Server) Workspaces() []client.Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]client.Workspace, 0, len(s.store.workspaces))
	for _, w := range s.store.workspaces {
		out = append(out, *w)
	}
	return out
}
