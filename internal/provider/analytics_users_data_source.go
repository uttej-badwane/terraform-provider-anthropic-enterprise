package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAnalyticsUsersDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &analyticsUsersDataSource{}
	_ datasource.DataSourceWithConfigValidators = &analyticsUsersDataSource{}
)

// NewAnalyticsUsersDataSource returns the anthropic_analytics_users data source.
func NewAnalyticsUsersDataSource() datasource.DataSource { return &analyticsUsersDataSource{} }

type analyticsUsersDataSource struct{ client *client.Client }

// userInt64Fields keeps the schema and flattening of the flat counters in sync.
var userInt64Fields = []struct {
	name string
	get  func(*client.AnalyticsUserActivity) int64
}{
	{"chat_connectors_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.ConnectorsUsedCount }},
	{"chat_distinct_artifacts_created_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.DistinctArtifactsCreatedCount }},
	{"chat_distinct_connectors_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.DistinctConnectorsUsedCount }},
	{"chat_distinct_conversation_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.DistinctConversationCount }},
	{"chat_distinct_files_uploaded_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.DistinctFilesUploadedCount }},
	{"chat_distinct_projects_created_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.DistinctProjectsCreatedCount }},
	{"chat_distinct_projects_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.DistinctProjectsUsedCount }},
	{"chat_distinct_shared_artifacts_viewed_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.DistinctSharedArtifactsViewedCount }},
	{"chat_distinct_skills_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.DistinctSkillsUsedCount }},
	{"chat_message_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.MessageCount }},
	{"chat_shared_conversations_viewed_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.SharedConversationsViewedCount }},
	{"chat_thinking_message_count", func(u *client.AnalyticsUserActivity) int64 { return u.ChatMetrics.ThinkingMessageCount }},
	{"claude_code_artifacts_created_count", func(u *client.AnalyticsUserActivity) int64 {
		return u.ClaudeCodeMetrics.CoreMetrics.ArtifactsCreatedCount
	}},
	{"claude_code_commit_count", func(u *client.AnalyticsUserActivity) int64 { return u.ClaudeCodeMetrics.CoreMetrics.CommitCount }},
	{"claude_code_distinct_session_count", func(u *client.AnalyticsUserActivity) int64 {
		return u.ClaudeCodeMetrics.CoreMetrics.DistinctSessionCount
	}},
	{"claude_code_lines_added_count", func(u *client.AnalyticsUserActivity) int64 {
		return u.ClaudeCodeMetrics.CoreMetrics.LinesOfCode.AddedCount
	}},
	{"claude_code_lines_removed_count", func(u *client.AnalyticsUserActivity) int64 {
		return u.ClaudeCodeMetrics.CoreMetrics.LinesOfCode.RemovedCount
	}},
	{"claude_code_pull_request_count", func(u *client.AnalyticsUserActivity) int64 { return u.ClaudeCodeMetrics.CoreMetrics.PullRequestCount }},
	{"cowork_action_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.ActionCount }},
	{"cowork_artifacts_created_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.ArtifactsCreatedCount }},
	{"cowork_connectors_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.ConnectorsUsedCount }},
	{"cowork_dispatch_turn_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.DispatchTurnCount }},
	{"cowork_distinct_connectors_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.DistinctConnectorsUsedCount }},
	{"cowork_distinct_session_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.DistinctSessionCount }},
	{"cowork_distinct_skills_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.DistinctSkillsUsedCount }},
	{"cowork_message_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.MessageCount }},
	{"cowork_skills_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.SkillsUsedCount }},
	{"cowork_distinct_plugins_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.DistinctPluginsUsedCount }},
	{"cowork_edit_tool_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.EditToolCount }},
	{"cowork_file_edit_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.FileEditCount }},
	{"cowork_multi_edit_tool_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.MultiEditToolCount }},
	{"cowork_notebook_edit_tool_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.NotebookEditToolCount }},
	{"cowork_plugins_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.PluginsUsedCount }},
	{"cowork_sessions_with_file_edits_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.SessionsWithFileEditsCount }},
	{"cowork_write_tool_count", func(u *client.AnalyticsUserActivity) int64 { return u.CoworkMetrics.WriteToolCount }},
	{"design_distinct_projects_created_count", func(u *client.AnalyticsUserActivity) int64 { return u.DesignMetrics.DistinctProjectsCreatedCount }},
	{"design_distinct_projects_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.DesignMetrics.DistinctProjectsUsedCount }},
	{"design_distinct_session_count", func(u *client.AnalyticsUserActivity) int64 { return u.DesignMetrics.DistinctSessionCount }},
	{"design_message_count", func(u *client.AnalyticsUserActivity) int64 { return u.DesignMetrics.MessageCount }},
	{"science_delegation_count", func(u *client.AnalyticsUserActivity) int64 { return u.ScienceMetrics.DelegationCount }},
	{"science_distinct_session_count", func(u *client.AnalyticsUserActivity) int64 { return u.ScienceMetrics.DistinctSessionCount }},
	{"science_message_count", func(u *client.AnalyticsUserActivity) int64 { return u.ScienceMetrics.MessageCount }},
	{"science_remote_compute_job_count", func(u *client.AnalyticsUserActivity) int64 { return u.ScienceMetrics.RemoteComputeJobCount }},
	{"science_skills_used_count", func(u *client.AnalyticsUserActivity) int64 { return u.ScienceMetrics.SkillsUsedCount }},
	{"web_search_count", func(u *client.AnalyticsUserActivity) int64 { return u.WebSearchCount }},
}

var officeApps = []struct {
	prefix string
	get    func(*client.OfficeMetrics) *client.OfficeProductMetrics
}{
	{"office_excel_", func(o *client.OfficeMetrics) *client.OfficeProductMetrics { return &o.Excel }},
	{"office_outlook_", func(o *client.OfficeMetrics) *client.OfficeProductMetrics { return &o.Outlook }},
	{"office_powerpoint_", func(o *client.OfficeMetrics) *client.OfficeProductMetrics { return &o.PowerPoint }},
	{"office_word_", func(o *client.OfficeMetrics) *client.OfficeProductMetrics { return &o.Word }},
}

var officeFields = []struct {
	name string
	get  func(*client.OfficeProductMetrics) int64
}{
	{"connectors_used_count", func(m *client.OfficeProductMetrics) int64 { return m.ConnectorsUsedCount }},
	{"distinct_connectors_used_count", func(m *client.OfficeProductMetrics) int64 { return m.DistinctConnectorsUsedCount }},
	{"distinct_session_count", func(m *client.OfficeProductMetrics) int64 { return m.DistinctSessionCount }},
	{"distinct_skills_used_count", func(m *client.OfficeProductMetrics) int64 { return m.DistinctSkillsUsedCount }},
	{"message_count", func(m *client.OfficeProductMetrics) int64 { return m.MessageCount }},
	{"skills_used_count", func(m *client.OfficeProductMetrics) int64 { return m.SkillsUsedCount }},
}

var attrTypesUserToolAction = map[string]attr.Type{"tool": types.StringType, "accepted_count": types.Int64Type, "rejected_count": types.Int64Type}

func userRowAttrs() map[string]schema.Attribute {
	m := map[string]schema.Attribute{
		"user_id":       dsString("User id."),
		"email_address": dsString("User email address."),
		"claude_code_tool_actions": schema.ListNestedAttribute{
			MarkdownDescription: "Accepted and rejected Claude Code tool proposals per tool, sorted by tool name.",
			Computed:            true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"tool": dsString("Tool name."), "accepted_count": dsInt64("Accepted proposals."), "rejected_count": dsInt64("Rejected proposals."),
			}},
		},
		"distinct_user_count": dsInt64("Users in the group row; null on per-user rows."),
		"last_activity_date":  dsString("Most recent activity day, `YYYY-MM-DD`."),
		"rbac_group_id":       dsString("RBAC group id; present only when grouped by `rbac_group_id`."),
		"rbac_group_name":     dsString("RBAC group name."),
	}
	for _, f := range userInt64Fields {
		m[f.name] = dsInt64("Counter `" + f.name + "`.")
	}
	for _, app := range officeApps {
		for _, f := range officeFields {
			m[app.prefix+f.name] = dsInt64("Office Agent counter `" + app.prefix + f.name + "`.")
		}
	}
	return m
}

func (d *analyticsUsersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_users"
}

func (d *analyticsUsersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := analyticsListInputs([]string{"rbac_group_id", "user_id", "project_id"}, []string{"rbac_group_id"})
	attrs["rows"] = schema.ListNestedAttribute{
		MarkdownDescription: "One row per user (or per RBAC group when grouped).",
		Computed:            true,
		NestedObject:        schema.NestedAttributeObject{Attributes: userRowAttrs()},
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Per-user activity across chat, Claude Code, Cowork, Claude Design, Office Agent and Science for a Claude " +
			"Enterprise organization." + analyticsNote,
		Attributes: attrs,
	}
}

func (d *analyticsUsersDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return analyticsListValidators()
}

func (d *analyticsUsersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsUsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsListModel
	p, ok := readAnalytics(ctx, req, resp, &cfg)
	if !ok {
		return
	}
	rows, err := d.client.ListAnalyticsUsers(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics users", err)
		return
	}
	rows = capRows(rows, cfg.LimitRows)
	rowTypes := attrTypesFromSchema(userRowAttrs())
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		u := &rows[i]
		v := map[string]attr.Value{
			"user_id":             types.StringNull(),
			"email_address":       types.StringNull(),
			"distinct_user_count": int64FromPtr(u.DistinctUserCount),
			"last_activity_date":  stringFromPtr(u.LastActivityDate),
			"rbac_group_id":       stringFromPtr(u.RBACGroupID),
			"rbac_group_name":     stringFromPtr(u.RBACGroupName),
		}
		if u.User != nil {
			v["user_id"] = types.StringValue(u.User.ID)
			v["email_address"] = types.StringValue(u.User.EmailAddress)
		}
		for _, f := range userInt64Fields {
			v[f.name] = types.Int64Value(f.get(u))
		}
		for _, app := range officeApps {
			m := app.get(&u.OfficeMetrics)
			for _, f := range officeFields {
				v[app.prefix+f.name] = types.Int64Value(f.get(m))
			}
		}
		tools := make([]string, 0, len(u.ClaudeCodeMetrics.ToolActions))
		for t := range u.ClaudeCodeMetrics.ToolActions {
			tools = append(tools, t)
		}
		sort.Strings(tools)
		acts := make([]map[string]attr.Value, 0, len(tools))
		for _, t := range tools {
			c := u.ClaudeCodeMetrics.ToolActions[t]
			acts = append(acts, map[string]attr.Value{"tool": types.StringValue(t), "accepted_count": types.Int64Value(c.AcceptedCount), "rejected_count": types.Int64Value(c.RejectedCount)})
		}
		v["claude_code_tool_actions"] = objectList(&resp.Diagnostics, attrTypesUserToolAction, acts)
		vals = append(vals, v)
	}
	cfg.Rows = objectList(&resp.Diagnostics, rowTypes, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
