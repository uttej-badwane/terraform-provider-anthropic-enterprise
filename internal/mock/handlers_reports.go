package mock

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// Additional mock credentials for the compliance and analytics surfaces.
const (
	ComplianceKey = "sk-ant-api01-compliance-test-0000"
	AnalyticsKey  = "sk-ant-api01-analytics-test-0000"
)

// Linked organizations returned by the compliance directory.
var complianceOrgs = []client.ComplianceOrganization{
	{UUID: "11111111-2222-3333-4444-555555555555", Name: "Example Organization", CreatedAt: "2025-06-01T10:00:00Z"},
	{UUID: "66666666-7777-8888-9999-000000000000", Name: "Example Organization Legal", CreatedAt: "2025-07-15T14:30:00Z"},
}

func (s *Server) requireCompliance(w http.ResponseWriter, r *http.Request) bool {
	switch r.Header.Get("X-Api-Key") {
	case ComplianceKey, EnterpriseKey:
		return true
	case AdminKey, AgentsKey:
		writeError(w, http.StatusForbidden, "permission_error", "this key type cannot call the Compliance API")
		return false
	}
	writeError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
	return false
}

func (s *Server) requireAnalytics(w http.ResponseWriter, r *http.Request) bool {
	switch r.Header.Get("X-Api-Key") {
	case AnalyticsKey, EnterpriseKey:
		return true
	case AdminKey, AgentsKey:
		writeError(w, http.StatusForbidden, "permission_error", "this key type cannot call the Analytics API")
		return false
	}
	writeError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
	return false
}

func (s *Server) reportRoutes() {
	s.handle("GET "+base+"/spend_limit_increase_requests", s.listIncreaseRequests)
	s.handle("GET "+base+"/spend_limit_increase_requests/{id}", s.getIncreaseRequest)
	s.handle("GET "+base+"/usage_report/messages", s.usageReport)
	s.handle("GET "+base+"/cost_report", s.costReport)
	s.handle("GET "+base+"/usage_report/claude_code", s.claudeCodeReport)
	s.handle("GET "+base+"/analytics/summaries", s.analyticsSummaries)

	s.handle("GET /v1/compliance/organizations", s.complianceOrganizations)
	s.handle("GET /v1/compliance/organizations/{org}/users", s.complianceUsers)
	s.handle("GET /v1/compliance/organizations/{org}/roles", s.complianceRoles)
	s.handle("GET /v1/compliance/organizations/{org}/roles/{id}", s.complianceRole)
	s.handle("GET /v1/compliance/organizations/{org}/roles/{id}/permissions", s.complianceRolePermissions)
	s.handle("GET /v1/compliance/organizations/{org}/settings", s.complianceSettings)
	s.handle("GET /v1/compliance/groups", s.complianceGroups)
	s.handle("GET /v1/compliance/groups/{id}", s.complianceGroup)
	s.handle("GET /v1/compliance/groups/{id}/members", s.complianceGroupMembers)
}

// --- spend limit increase requests ------------------------------------------

func (s *Server) seedIncreaseRequests() {
	u := s.store.users[3]
	admin := s.store.users[1]
	s.store.increaseRequests = []*client.SpendLimitIncreaseRequest{
		{ID: s.nextID("slir"), Type: "spend_limit_increase_request", Status: "pending", Period: "monthly", CreatedAt: now(),
			Actor:        client.RequestActor{Type: "user_actor", UserID: ptr(u.ID), Name: ptr(u.Name), EmailAddress: ptr(u.Email), Deleted: ptr(false)},
			SpendSummary: &client.IncreaseRequestSpendSummary{Actor: client.RequestActor{Type: "user_actor", UserID: ptr(u.ID)}, Amount: ptr("10000"), Currency: "USD", Period: "monthly", PeriodToDateSpend: "9950.5"}},
		{ID: s.nextID("slir"), Type: "spend_limit_increase_request", Status: "approved", Period: "monthly", CreatedAt: now(), ResolvedAt: ptr(now()),
			Actor:      client.RequestActor{Type: "user_actor", UserID: ptr(s.store.users[2].ID), Name: ptr(s.store.users[2].Name), EmailAddress: ptr(s.store.users[2].Email), Deleted: ptr(false)},
			ResolvedBy: &client.RequestActor{Type: "user_actor", UserID: ptr(admin.ID), Name: ptr(admin.Name), EmailAddress: ptr(admin.Email)}},
	}
}

func (s *Server) listIncreaseRequests(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	q := r.URL.Query()
	out := []*client.SpendLimitIncreaseRequest{}
	for _, req := range s.store.increaseRequests {
		if st := q["status[]"]; len(st) > 0 && !slices.Contains(st, req.Status) {
			continue
		}
		if ids := q["actor_ids[]"]; len(ids) > 0 && (req.Actor.UserID == nil || !slices.Contains(ids, *req.Actor.UserID)) {
			continue
		}
		out = append(out, req)
	}
	tokenList(w, r, out)
}

func (s *Server) getIncreaseRequest(w http.ResponseWriter, r *http.Request) {
	if !s.requireEnterprise(w, r) {
		return
	}
	for _, req := range s.store.increaseRequests {
		if req.ID == r.PathValue("id") {
			writeJSON(w, req)
			return
		}
	}
	notFound(w, "spend_limit_increase_request")
}

// --- usage / cost / claude code ---------------------------------------------

func parseWindow(w http.ResponseWriter, r *http.Request, defaultWidth string) (time.Time, time.Time, time.Duration, bool) {
	q := r.URL.Query()
	start, err := time.Parse(time.RFC3339, q.Get("starting_at"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "starting_at must be an RFC 3339 timestamp")
		return time.Time{}, time.Time{}, 0, false
	}
	width := q.Get("bucket_width")
	if width == "" {
		width = defaultWidth
	}
	var step time.Duration
	switch width {
	case "1d":
		step = 24 * time.Hour
	case "1h":
		step = time.Hour
	case "1m":
		step = time.Minute
	default:
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bucket_width must be 1d, 1h or 1m")
		return time.Time{}, time.Time{}, 0, false
	}
	start = start.UTC().Truncate(step)
	end := start.Add(3 * step)
	if e := q.Get("ending_at"); e != "" {
		end, err = time.Parse(time.RFC3339, e)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "ending_at must be an RFC 3339 timestamp")
			return time.Time{}, time.Time{}, 0, false
		}
		end = end.UTC()
	}
	if !end.After(start) {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "ending_at must be after starting_at")
		return time.Time{}, time.Time{}, 0, false
	}
	return start, end, step, true
}

func (s *Server) usageReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	start, end, step, ok := parseWindow(w, r, "1d")
	if !ok {
		return
	}
	groupBy := r.URL.Query()["group_by[]"]
	speeds := r.URL.Query()["speeds[]"]

	// The real API gates the speed dimension on a beta header and rejects it
	// with a 400 listing the dimensions it does accept. Mirroring that is what
	// makes the client's "send the header only when asked" logic testable: a
	// mock that accepted speed unconditionally would pass either way.
	if slices.Contains(groupBy, "speed") || len(speeds) > 0 {
		if !slices.Contains(strings.Split(r.Header.Get("Anthropic-Beta"), ","), client.BetaFastMode) {
			writeError(w, http.StatusBadRequest, "invalid_request_error",
				`Invalid `+"`group_by[]`"+`: "speed". Valid options are "account_id", "api_key_id", "context_window", "inference_geo", "model", "service_account_id", "service_tier", "workspace_id".`)
			return
		}
	}

	var buckets []client.UsageBucket
	i := int64(0)
	for t := start; t.Before(end) && len(buckets) < 31; t = t.Add(step) {
		i++
		res := client.UsageResult{UncachedInputTokens: 1500 * i, CacheReadInputTokens: 200 * i, OutputTokens: 500 * i,
			CacheCreation: client.CacheCreation{Ephemeral5mInputTokens: 10 * i}, ServerToolUse: client.ServerToolUse{WebSearchRequests: i}}
		if slices.Contains(groupBy, "model") {
			res.Model = ptr("claude-sonnet-5")
		}
		if slices.Contains(groupBy, "workspace_id") {
			res.WorkspaceID = ptr(s.store.defaultWorkspace)
		}
		if slices.Contains(groupBy, "service_tier") {
			res.ServiceTier = ptr("standard")
		}
		if slices.Contains(groupBy, "context_window") {
			res.ContextWindow = ptr("0-200k")
		}
		if slices.Contains(groupBy, "inference_geo") {
			res.InferenceGeo = ptr("global")
		}
		if slices.Contains(groupBy, "speed") {
			// When the caller also filtered, report the speed they asked for,
			// so a filtered-and-grouped query does not contradict itself.
			if len(speeds) > 0 {
				res.Speed = ptr(speeds[0])
			} else {
				res.Speed = ptr("standard")
			}
		}
		if slices.Contains(groupBy, "api_key_id") && len(s.store.apiKeys) > 0 {
			res.APIKeyID = ptr(s.store.apiKeys[0].ID)
		}
		buckets = append(buckets, client.UsageBucket{StartingAt: t.Format(time.RFC3339), EndingAt: t.Add(step).Format(time.RFC3339), Results: []client.UsageResult{res}})
	}
	tokenList(w, r, buckets)
}

func (s *Server) costReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if bw := r.URL.Query().Get("bucket_width"); bw != "" && bw != "1d" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "bucket_width must be 1d")
		return
	}
	start, end, step, ok := parseWindow(w, r, "1d")
	if !ok {
		return
	}
	groupBy := r.URL.Query()["group_by[]"]
	var buckets []client.CostBucket
	i := int64(0)
	for t := start; t.Before(end) && len(buckets) < 31; t = t.Add(step) {
		i++
		res := client.CostResult{Amount: "123.5", Currency: "USD"}
		if slices.Contains(groupBy, "workspace_id") {
			res.WorkspaceID = ptr(s.store.defaultWorkspace)
		}
		if slices.Contains(groupBy, "description") {
			res.Description = ptr("Claude Sonnet 5 Usage - Input Tokens")
			res.CostType = ptr("tokens")
			res.Model = ptr("claude-sonnet-5")
			res.ServiceTier = ptr("standard")
			res.TokenType = ptr("uncached_input_tokens")
			res.ContextWindow = ptr("0-200k")
			res.InferenceGeo = ptr("global")
		}
		buckets = append(buckets, client.CostBucket{StartingAt: t.Format(time.RFC3339), EndingAt: t.Add(step).Format(time.RFC3339), Results: []client.CostResult{res}})
	}
	tokenList(w, r, buckets)
}

func (s *Server) claudeCodeReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	date := r.URL.Query().Get("starting_at")
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "starting_at must be YYYY-MM-DD")
		return
	}
	day := d.UTC().Format(time.RFC3339)
	rows := []client.ClaudeCodeUsage{
		{Actor: client.ClaudeCodeActor{Type: "user_actor", EmailAddress: ptr(s.store.users[2].Email)}, CustomerType: "api", Date: day, OrganizationID: s.OrgID,
			TerminalType: "iTerm.app", CoreMetrics: client.ClaudeCodeCoreMetrics{CommitsByClaudeCode: 8, LinesOfCode: client.LinesOfCode{Added: 342, Removed: 128}, NumSessions: 15, PullRequestsByClaudeCode: 2},
			ModelBreakdown: []client.ModelBreakdown{{Model: "claude-sonnet-5", EstimatedCost: client.EstimatedCost{Amount: 186, Currency: "USD"}, Tokens: client.ModelTokens{CacheCreation: 2340, CacheRead: 8790, Input: 45230, Output: 12450}}},
			ToolActions:    map[string]client.ToolActionCounts{"edit_tool": {Accepted: 25, Rejected: 3}, "write_tool": {Accepted: 8}}},
		{Actor: client.ClaudeCodeActor{Type: "api_actor", APIKeyName: ptr("Developer Key")}, CustomerType: "api", Date: day, OrganizationID: s.OrgID, IsRemote: true,
			TerminalType: "claude-code-web", CoreMetrics: client.ClaudeCodeCoreMetrics{NumSessions: 2},
			ModelBreakdown: []client.ModelBreakdown{}, ToolActions: map[string]client.ToolActionCounts{}},
	}
	tokenList(w, r, rows)
}

func (s *Server) analyticsSummaries(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	q := r.URL.Query()
	start, err := time.Parse("2006-01-02", q.Get("starting_date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "starting_date must be YYYY-MM-DD")
		return
	}
	end := start.AddDate(0, 0, 3)
	if e := q.Get("ending_date"); e != "" {
		if end, err = time.Parse("2006-01-02", e); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "ending_date must be YYYY-MM-DD")
			return
		}
	}
	for _, f := range q["filter[]"] {
		if !strings.HasPrefix(f, "rbac_group_id:") {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "unsupported filter dimension")
			return
		}
	}
	out := []client.AnalyticsSummary{}
	seats := int64(len(s.store.users))
	for t := start; t.Before(end); t = t.AddDate(0, 0, 1) {
		out = append(out, client.AnalyticsSummary{StartingAt: t.Format(time.RFC3339), EndingAt: t.AddDate(0, 0, 1).Format(time.RFC3339),
			AssignedSeatCount: seats, PendingInviteCount: 1, DailyActiveUserCount: 3, WeeklyActiveUserCount: 4, MonthlyActiveUserCount: seats,
			DailyAdoptionRate: 0.6, WeeklyAdoptionRate: 0.8, MonthlyAdoptionRate: 1,
			ChatDailyActiveUserCount: 3, ChatWeeklyActiveUserCount: 4, ChatMonthlyActiveUserCount: seats,
			ClaudeCodeDailyActiveUserCount: 1, ClaudeCodeWeeklyActiveUserCount: 2, ClaudeCodeMonthlyActiveUserCount: 2})
	}
	writeJSON(w, map[string]any{"summaries": out})
}

// --- compliance directory ---------------------------------------------------

func (s *Server) complianceOrg(w http.ResponseWriter, r *http.Request) bool {
	for _, o := range complianceOrgs {
		if o.UUID == r.PathValue("org") {
			return true
		}
	}
	notFound(w, "organization")
	return false
}

func (s *Server) complianceOrganizations(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) {
		return
	}
	tokenList(w, r, complianceOrgs)
}

func (s *Server) complianceUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) || !s.complianceOrg(w, r) {
		return
	}
	out := []client.ComplianceUser{}
	for _, u := range s.store.users {
		out = append(out, client.ComplianceUser{ID: u.ID, FullName: u.Name, Email: u.Email, OrganizationRole: u.Role, CreatedAt: u.AddedAt})
	}
	tokenList(w, r, out)
}

func (s *Server) complianceRoleOf(role *client.RBACRole) client.ComplianceRole {
	return client.ComplianceRole{ID: role.ID, Name: role.Name, Description: ptr("Custom role"), CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt}
}

func (s *Server) complianceRoles(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) || !s.complianceOrg(w, r) {
		return
	}
	out := []client.ComplianceRole{}
	for _, role := range s.store.roles {
		out = append(out, s.complianceRoleOf(role))
	}
	tokenList(w, r, out)
}

func (s *Server) complianceRole(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) || !s.complianceOrg(w, r) {
		return
	}
	for _, role := range s.store.roles {
		if role.ID == r.PathValue("id") {
			writeJSON(w, s.complianceRoleOf(role))
			return
		}
	}
	notFound(w, "role")
}

func (s *Server) complianceRolePermissions(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) || !s.complianceOrg(w, r) {
		return
	}
	for _, role := range s.store.roles {
		if role.ID == r.PathValue("id") {
			tokenList(w, r, []client.RBACRolePermission{
				{Action: "chat", Resource: map[string]any{"type": "organization", "organization_id": s.OrgID}, Type: "rbac_role_permission"},
			})
			return
		}
	}
	notFound(w, "role")
}

func (s *Server) complianceGroupOf(g *client.RBACGroup) client.ComplianceGroup {
	roles := g.Roles
	if roles == nil {
		roles = []string{}
	}
	return client.ComplianceGroup{ID: g.ID, Name: g.Name, Description: ptr(""), SourceType: g.SourceType, Roles: roles, CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt}
}

func (s *Server) complianceGroups(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) {
		return
	}
	out := []client.ComplianceGroup{}
	for _, g := range s.store.groups {
		out = append(out, s.complianceGroupOf(g))
	}
	tokenList(w, r, out)
}

func (s *Server) complianceGroup(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) {
		return
	}
	g := s.findGroup(r.PathValue("id"))
	if g == nil {
		notFound(w, "group")
		return
	}
	writeJSON(w, s.complianceGroupOf(g))
}

func (s *Server) complianceGroupMembers(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) {
		return
	}
	g := s.findGroup(r.PathValue("id"))
	if g == nil {
		notFound(w, "group")
		return
	}
	out := []client.ComplianceGroupMember{}
	for _, m := range s.store.groupMembers {
		if m.GroupID == g.ID {
			out = append(out, client.ComplianceGroupMember{UserID: m.UserID, Email: m.Email, CreatedAt: m.CreatedAt, UpdatedAt: m.CreatedAt})
		}
	}
	tokenList(w, r, out)
}

func (s *Server) complianceSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireCompliance(w, r) {
		return
	}
	org := r.PathValue("org")
	// The parent itself is not a valid target; only linked orgs after the first.
	if org == complianceOrgs[0].UUID || !s.complianceOrg(w, r) {
		if org == complianceOrgs[0].UUID {
			notFound(w, "organization")
		}
		return
	}
	raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	writeJSON(w, client.EffectiveOrganizationSettings{
		Type: "effective_organization_settings", OrganizationID: org,
		Settings: []client.EffectiveSetting{
			{Name: "data_retention_periods", Type: "data_retention", Value: raw(map[string]any{"chat": map[string]any{"type": "fixed", "timescale": "day", "duration": 90}})},
			{Name: "content_redaction_enabled", Type: "boolean", Value: raw(true)},
			{Name: "ip_allowlist_enabled", Type: "boolean", Value: raw(false)},
			{Name: "ip_allowlist_ip_ranges", Type: "string_list", Value: raw([]string{"10.0.0.0/8", "203.0.113.0/24"})},
			{Name: "account_session_duration_seconds", Type: "integer", Value: raw(28800)},
			{Name: "sso_provisioning_mode", Type: "provisioning_mode", Value: raw("jit_advanced")},
		},
		APIKeys: []client.ComplianceAPIKeyInfo{{Type: "compliance_api_key", ID: "apikey_000000000099", Name: "Compliance Export Key",
			Scopes: []string{"read:compliance_activities", "read:compliance_org_data"}, IsActive: true, CreatedAt: "2026-03-14T09:30:00Z", CreatedByID: ptr(s.store.users[0].ID)}},
	})
}
