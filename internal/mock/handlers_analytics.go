package mock

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

const analyticsRefreshedAt = "2026-01-17T12:00:00Z"

func (s *Server) analyticsRoutes() {
	s.handle("GET "+base+"/analytics/users", s.analyticsUsers)
	s.handle("GET "+base+"/analytics/skills", s.analyticsSkills)
	s.handle("GET "+base+"/analytics/connectors", s.analyticsConnectors)
	s.handle("GET "+base+"/analytics/plugins", s.analyticsPlugins)
	s.handle("GET "+base+"/analytics/artifacts", s.analyticsArtifacts)
	s.handle("GET "+base+"/analytics/usage_report", s.analyticsUsageReport)
	s.handle("GET "+base+"/analytics/cost_report", s.analyticsCostReport)
	s.handle("GET "+base+"/analytics/user_usage_report", s.analyticsUserUsage)
	s.handle("GET "+base+"/analytics/user_cost_report", s.analyticsUserCost)
}

// listParams validates the shared per-entity list parameters.
func (s *Server) listParams(w http.ResponseWriter, r *http.Request, dims, groups []string, dateRequired bool) (groupBy []string, ok bool) {
	q := r.URL.Query()
	date, start := q.Get("date"), q.Get("starting_date")
	if date != "" && start != "" {
		writeError(w, 400, "invalid_request_error", "use either date or starting_date, not both")
		return nil, false
	}
	if dateRequired && date == "" {
		writeError(w, 400, "invalid_request_error", "date is required")
		return nil, false
	}
	for _, d := range []string{date, start, q.Get("ending_date")} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				writeError(w, 400, "invalid_request_error", "dates must be YYYY-MM-DD")
				return nil, false
			}
		}
	}
	for _, f := range q["filter[]"] {
		dim, _, found := strings.Cut(f, ":")
		if !found || !slices.Contains(dims, dim) {
			writeError(w, 400, "invalid_request_error", "unsupported filter dimension: "+dim)
			return nil, false
		}
	}
	for _, g := range q["group_by[]"] {
		if !slices.Contains(groups, g) {
			writeError(w, 400, "invalid_request_error", "unsupported group_by dimension: "+g)
			return nil, false
		}
	}
	if o := q.Get("order"); o != "" && o != "asc" && o != "desc" {
		writeError(w, 400, "invalid_request_error", "order must be asc or desc")
		return nil, false
	}
	return q["group_by[]"], true
}

func groupDims(groupBy []string, userID string, group *client.RBACGroup, product string) (p, gid, gname, uid *string) {
	if slices.Contains(groupBy, "product") {
		p = ptr(product)
	}
	if slices.Contains(groupBy, "rbac_group_id") && group != nil {
		gid, gname = ptr(group.ID), ptr(group.Name)
	}
	if slices.Contains(groupBy, "user_id") {
		uid = ptr(userID)
	}
	return
}

func (s *Server) analyticsUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	groupBy, ok := s.listParams(w, r, []string{"project_id", "rbac_group_id", "user_id"}, []string{"rbac_group_id"}, false)
	if !ok {
		return
	}
	out := []client.AnalyticsUserActivity{}
	for i, u := range s.store.users {
		row := client.AnalyticsUserActivity{User: &client.AnalyticsUserRef{Type: "user", ID: u.ID, EmailAddress: u.Email},
			WebSearchCount: int64(i), LastActivityDate: ptr("2026-01-15")}
		row.ChatMetrics.MessageCount = int64(10 * (i + 1))
		row.ChatMetrics.DistinctConversationCount = int64(i + 1)
		row.ClaudeCodeMetrics.CoreMetrics.DistinctSessionCount = int64(i)
		row.ClaudeCodeMetrics.CoreMetrics.CommitCount = int64(2 * i)
		row.ClaudeCodeMetrics.ToolActions = map[string]client.CountPair{"edit_tool": {AcceptedCount: int64(3 * i), RejectedCount: 1}}
		if slices.Contains(groupBy, "rbac_group_id") && len(s.store.groups) > 0 {
			row.RBACGroupID, row.RBACGroupName = ptr(s.store.groups[0].ID), ptr(s.store.groups[0].Name)
			row.DistinctUserCount = ptr(int64(1))
		}
		out = append(out, row)
	}
	tokenList(w, r, out)
}

func (s *Server) analyticsSkills(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	groupBy, ok := s.listParams(w, r, []string{"product", "rbac_group_id", "share_status", "skill_name", "user_id"}, []string{"product", "rbac_group_id", "user_id"}, false)
	if !ok {
		return
	}
	var g *client.RBACGroup
	if len(s.store.groups) > 0 {
		g = s.store.groups[0]
	}
	out := []client.AnalyticsSkillUsage{}
	for i, name := range []string{"docx", "pdf", "xlsx"} {
		p, gid, gname, uid := groupDims(groupBy, s.store.users[2].ID, g, "claude_code")
		row := client.AnalyticsSkillUsage{SkillName: name, DistinctUserCount: int64(i + 2), InvocationCount: ptr(int64(10 * (i + 1))),
			EstimatedOverageSpend: ptr("1250"), AttributedListPrice: ptr("2000"), Currency: ptr("USD"), ShareStatus: ptr("organization"),
			Product: p, RBACGroupID: gid, RBACGroupName: gname, UserID: uid}
		row.ChatMetrics.DistinctConversationSkillUsedCount = ptr(int64(i + 1))
		row.ClaudeCodeMetrics.DistinctSessionSkillUsedCount = ptr(int64(i))
		out = append(out, row)
	}
	tokenList(w, r, out)
}

func (s *Server) analyticsConnectors(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	groupBy, ok := s.listParams(w, r, []string{"connector_name", "product", "rbac_group_id", "user_id"}, []string{"product", "rbac_group_id", "user_id"}, false)
	if !ok {
		return
	}
	var g *client.RBACGroup
	if len(s.store.groups) > 0 {
		g = s.store.groups[0]
	}
	out := []client.AnalyticsConnectorUsage{}
	for i, name := range []string{"atlassian", "github", "slack"} {
		p, gid, gname, uid := groupDims(groupBy, s.store.users[2].ID, g, "chat")
		row := client.AnalyticsConnectorUsage{ConnectorName: name, DistinctUserCount: int64(i + 1), ReadCallCount: ptr(int64(40)), WriteCallCount: ptr(int64(5)),
			UnclassifiedCallCount: ptr(int64(2)), ManagedAuthDistinctUserCount: ptr(int64(1)), IndividualAuthDistinctUserCount: ptr(int64(i)),
			Product: p, RBACGroupID: gid, RBACGroupName: gname, UserID: uid}
		row.ChatMetrics.DistinctConversationConnectorUsedCount = ptr(int64(i + 3))
		out = append(out, row)
	}
	tokenList(w, r, out)
}

func (s *Server) analyticsPlugins(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	groupBy, ok := s.listParams(w, r, []string{"plugin_name", "product", "rbac_group_id", "user_id"}, []string{"product", "rbac_group_id", "user_id"}, false)
	if !ok {
		return
	}
	var g *client.RBACGroup
	if len(s.store.groups) > 0 {
		g = s.store.groups[0]
	}
	out := []client.AnalyticsPluginUsage{}
	for i, name := range []string{"example-plugin", "third-party"} {
		p, gid, gname, uid := groupDims(groupBy, s.store.users[2].ID, g, "claude_code")
		row := client.AnalyticsPluginUsage{PluginName: name, DistinctUserCount: int64(i + 1), InstallCount: ptr(int64(i + 1)), InvocationCount: int64(7 * (i + 1)),
			Product: p, RBACGroupID: gid, RBACGroupName: gname, UserID: uid}
		if name != "third-party" {
			row.PluginID = ptr(name + "@example-marketplace")
		}
		row.ClaudeCodeMetrics.DistinctSessionPluginUsedCount = ptr(int64(i + 2))
		out = append(out, row)
	}
	tokenList(w, r, out)
}

func (s *Server) analyticsArtifacts(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	groupBy, ok := s.listParams(w, r, []string{"artifact_type", "is_shared", "product", "rbac_group_id", "user_id"}, []string{"product", "rbac_group_id", "user_id"}, true)
	if !ok {
		return
	}
	var g *client.RBACGroup
	if len(s.store.groups) > 0 {
		g = s.store.groups[0]
	}
	out := []client.AnalyticsArtifactUsage{}
	for _, t := range []string{"text/markdown", "text/html"} {
		for _, shared := range []bool{false, true} {
			p, gid, gname, uid := groupDims(groupBy, s.store.users[2].ID, g, "chat")
			out = append(out, client.AnalyticsArtifactUsage{ArtifactType: t, IsShared: shared, ArtifactsCreatedCount: 4, PublishedArtifactsCreatedCount: 1, DistinctUserCount: 2,
				Product: p, RBACGroupID: gid, RBACGroupName: gname, UserID: uid})
		}
	}
	tokenList(w, r, out)
}

// reportWindow validates the Enterprise report window parameters.
func (s *Server) reportWindow(w http.ResponseWriter, r *http.Request, groups []string) (time.Time, time.Time, time.Duration, bool) {
	q := r.URL.Query()
	for _, g := range q["group_by[]"] {
		if !slices.Contains(groups, g) {
			writeError(w, 400, "invalid_request_error", "unsupported group_by dimension: "+g)
			return time.Time{}, time.Time{}, 0, false
		}
	}
	for _, p := range q["products[]"] {
		if !slices.Contains([]string{"chat", "claude-tag", "claude_code", "claude_design", "claude_in_chrome", "cowork", "office_agent"}, p) {
			writeError(w, 400, "invalid_request_error", "unsupported product: "+p)
			return time.Time{}, time.Time{}, 0, false
		}
	}
	start, end, step, ok := parseWindow(w, r, "1d")
	if !ok {
		return start, end, step, false
	}
	if end.Sub(start) > 31*24*time.Hour {
		writeError(w, 400, "invalid_request_error", "the range may span at most 31 days")
		return start, end, step, false
	}
	return start, end, step, true
}

func dims(groupBy []string, group *client.RBACGroup) client.AnalyticsDims {
	d := client.AnalyticsDims{}
	if slices.Contains(groupBy, "model") {
		d.Model = ptr("claude-sonnet-5")
	}
	if slices.Contains(groupBy, "product") {
		d.Product = ptr("chat")
	}
	if slices.Contains(groupBy, "context_window") {
		d.ContextWindow = ptr("0-200k")
	}
	if slices.Contains(groupBy, "inference_geo") {
		d.InferenceGeo = ptr("global")
	}
	if slices.Contains(groupBy, "speed") {
		d.Speed = ptr("standard")
	}
	if slices.Contains(groupBy, "rbac_group_id") && group != nil {
		d.RBACGroupID = ptr(group.ID)
	}
	return d
}

func writeAnalyticsReport(w http.ResponseWriter, r *http.Request, s *Server, data any) {
	// tokenList handles paging; wrap to add the report metadata.
	rw := &captureWriter{ResponseWriter: w}
	switch v := data.(type) {
	case []client.AnalyticsUsageBucket:
		tokenList(rw, r, v)
	case []client.AnalyticsCostBucket:
		tokenList(rw, r, v)
	case []client.AnalyticsUserUsageRow:
		tokenList(rw, r, v)
	case []client.AnalyticsUserCostRow:
		tokenList(rw, r, v)
	}
	rw.body["data_refreshed_at"] = analyticsRefreshedAt
	rw.body["organization_id"] = "org_" + strings.ReplaceAll(s.OrgID, "-", "")[:24]
	writeJSON(w, rw.body)
}

func (s *Server) analyticsUsageReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	start, end, step, ok := s.reportWindow(w, r, []string{"claude_tag_category", "claude_tag_user_id", "context_window", "inference_geo", "model", "product", "rbac_group_id", "slack_channel_id", "speed"})
	if !ok {
		return
	}
	groupBy := r.URL.Query()["group_by[]"]
	var g *client.RBACGroup
	if len(s.store.groups) > 0 {
		g = s.store.groups[0]
	}
	var buckets []client.AnalyticsUsageBucket
	i := int64(0)
	for t := start; t.Before(end) && len(buckets) < 744; t = t.Add(step) {
		i++
		res := client.AnalyticsUsageResult{AnalyticsDims: dims(groupBy, g), UncachedInputTokens: 1000 * i, CacheReadInputTokens: 300 * i, OutputTokens: 200 * i, Requests: ptr(10 * i),
			CacheCreation: client.CacheCreation{Ephemeral5mInputTokens: 5 * i}, ServerToolUse: client.ServerToolUse{WebSearchRequests: i}}
		buckets = append(buckets, client.AnalyticsUsageBucket{StartingAt: t.Format(time.RFC3339), EndingAt: t.Add(step).Format(time.RFC3339), Results: []client.AnalyticsUsageResult{res}})
	}
	writeAnalyticsReport(w, r, s, buckets)
}

func (s *Server) analyticsCostReport(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	start, end, step, ok := s.reportWindow(w, r, []string{"claude_tag_category", "claude_tag_user_id", "context_window", "cost_type", "inference_geo", "model", "product", "rbac_group_id", "slack_channel_id", "speed", "token_type"})
	if !ok {
		return
	}
	groupBy := r.URL.Query()["group_by[]"]
	var g *client.RBACGroup
	if len(s.store.groups) > 0 {
		g = s.store.groups[0]
	}
	var buckets []client.AnalyticsCostBucket
	for t := start; t.Before(end) && len(buckets) < 744; t = t.Add(step) {
		res := client.AnalyticsCostResult{AnalyticsDims: dims(groupBy, g), Amount: "41280.000000", ListAmount: "51600.000000", Currency: "USD", Requests: ptr(int64(128))}
		if slices.Contains(groupBy, "cost_type") {
			res.CostType = ptr("tokens")
			res.Requests = nil
		}
		if slices.Contains(groupBy, "token_type") {
			res.TokenType = ptr("uncached_input_tokens")
			res.Requests = nil
		}
		buckets = append(buckets, client.AnalyticsCostBucket{StartingAt: t.Format(time.RFC3339), EndingAt: t.Add(step).Format(time.RFC3339), Results: []client.AnalyticsCostResult{res}})
	}
	writeAnalyticsReport(w, r, s, buckets)
}

func (s *Server) analyticsUserUsage(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	if _, _, _, ok := s.reportWindow(w, r, []string{"claude_tag_category", "claude_tag_user_id", "context_window", "inference_geo", "model", "product", "rbac_group_id", "slack_channel_id", "speed"}); !ok {
		return
	}
	q := r.URL.Query()
	if ob := q.Get("order_by"); ob != "" && !slices.Contains([]string{"total_tokens", "uncached_input_tokens", "output_tokens", "requests"}, ob) {
		writeError(w, 400, "invalid_request_error", "unsupported order_by")
		return
	}
	rows := []client.AnalyticsUserUsageRow{}
	for i, u := range s.store.users {
		if len(q["user_ids[]"]) > 0 && !slices.Contains(q["user_ids[]"], u.ID) {
			continue
		}
		n := int64(len(s.store.users) - i)
		row := client.AnalyticsUserUsageRow{Actor: client.AnalyticsActor{Type: "user_actor", UserID: u.ID, Email: ptr(u.Email), Name: ptr(u.Name)}}
		row.AnalyticsDims = dims(q["group_by[]"], nil)
		row.UncachedInputTokens, row.OutputTokens, row.CacheReadInputTokens, row.Requests = 1000*n, 200*n, 300*n, ptr(10*n)
		row.TotalTokens = row.UncachedInputTokens + row.OutputTokens + row.CacheReadInputTokens
		if bw := q.Get("bucket_width"); bw != "" {
			row.StartingAt, row.EndingAt = ptr(q.Get("starting_at")), ptr(q.Get("ending_at"))
		}
		rows = append(rows, row)
	}
	writeAnalyticsReport(w, r, s, rows)
}

func (s *Server) analyticsUserCost(w http.ResponseWriter, r *http.Request) {
	if !s.requireAnalytics(w, r) {
		return
	}
	if _, _, _, ok := s.reportWindow(w, r, []string{"claude_tag_category", "claude_tag_user_id", "context_window", "cost_type", "inference_geo", "model", "product", "rbac_group_id", "slack_channel_id", "speed", "token_type"}); !ok {
		return
	}
	q := r.URL.Query()
	if ob := q.Get("order_by"); ob != "" && ob != "amount" && ob != "list_amount" {
		writeError(w, 400, "invalid_request_error", "unsupported order_by")
		return
	}
	rows := []client.AnalyticsUserCostRow{}
	for i, u := range s.store.users {
		if len(q["user_ids[]"]) > 0 && !slices.Contains(q["user_ids[]"], u.ID) {
			continue
		}
		row := client.AnalyticsUserCostRow{Actor: client.AnalyticsActor{Type: "user_actor", UserID: u.ID, Email: ptr(u.Email), Name: ptr(u.Name)}}
		row.AnalyticsDims = dims(q["group_by[]"], nil)
		row.Amount, row.ListAmount, row.Currency, row.Requests = "1000.500000", "1250.000000", "USD", ptr(int64(5*(i+1)))
		rows = append(rows, row)
	}
	writeAnalyticsReport(w, r, s, rows)
}

// captureWriter lets tokenList build the paged envelope which we then extend.
type captureWriter struct {
	http.ResponseWriter
	body map[string]any
}

func (c *captureWriter) WriteHeader(int) {}
func (c *captureWriter) Write(b []byte) (int, error) {
	c.body = map[string]any{}
	return len(b), jsonUnmarshal(b, &c.body)
}
