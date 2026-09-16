package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAnalyticsSkillsDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &analyticsSkillsDataSource{}
	_ datasource.DataSourceWithConfigValidators = &analyticsSkillsDataSource{}
)

// NewAnalyticsSkillsDataSource returns the anthropic_analytics_skills data source.
func NewAnalyticsSkillsDataSource() datasource.DataSource { return &analyticsSkillsDataSource{} }

type analyticsSkillsDataSource struct{ client *client.Client }

func skillRowAttrs() map[string]schema.Attribute {
	return groupDimAttrs(map[string]schema.Attribute{
		"skill_name":              dsString("Skill name, or an opaque skill id for private skills."),
		"skill_display_name":      dsString("Resolved display name when `skill_name` is an opaque id."),
		"distinct_user_count":     dsInt64("Distinct users who used the skill."),
		"invocation_count":        dsInt64("Number of invocations; null when invocation reporting is not enabled."),
		"enable_count":            dsInt64("Distinct accounts that enabled the skill that day; null on grouped or range rows."),
		"estimated_overage_spend": dsString("Estimated overage spend attributed to the skill, minor units as a decimal string."),
		"attributed_list_price":   dsString("List-price value of requests that involved the skill, minor units."),
		"currency":                dsString("Currency of the monetary fields."),
		"share_status":            dsString("`private`, `organization` or `public` (claude.ai only)."),
		"chat_distinct_conversation_skill_used_count":         dsInt64("Distinct chat conversations that used the skill."),
		"claude_code_distinct_session_skill_used_count":       dsInt64("Distinct Claude Code sessions that used the skill."),
		"cowork_distinct_session_skill_used_count":            dsInt64("Distinct Cowork sessions that used the skill."),
		"office_excel_distinct_session_skill_used_count":      dsInt64("Distinct Excel sessions that used the skill."),
		"office_outlook_distinct_session_skill_used_count":    dsInt64("Distinct Outlook sessions that used the skill."),
		"office_powerpoint_distinct_session_skill_used_count": dsInt64("Distinct PowerPoint sessions that used the skill."),
		"office_word_distinct_session_skill_used_count":       dsInt64("Distinct Word sessions that used the skill."),
	})
}

func (d *analyticsSkillsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_skills"
}

func (d *analyticsSkillsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := analyticsListInputs([]string{"product", "rbac_group_id", "share_status", "skill_name", "user_id"}, []string{"product", "rbac_group_id", "user_id"})
	attrs["rows"] = schema.ListNestedAttribute{MarkdownDescription: "One row per skill (times group dimensions).", Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: skillRowAttrs()}}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Per-skill usage for a Claude Enterprise organization." + analyticsNote,
		Attributes:          attrs,
	}
}

func (d *analyticsSkillsDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return analyticsListValidators()
}

func (d *analyticsSkillsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsSkillsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsListModel
	p, ok := readAnalytics(ctx, req, resp, &cfg)
	if !ok {
		return
	}
	rows, err := d.client.ListAnalyticsSkills(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics skills", err)
		return
	}
	rows = capRows(rows, cfg.LimitRows)
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		vals = append(vals, groupDimValues(map[string]attr.Value{
			"skill_name":              types.StringValue(r.SkillName),
			"skill_display_name":      stringFromPtr(r.SkillDisplayName),
			"distinct_user_count":     types.Int64Value(r.DistinctUserCount),
			"invocation_count":        int64FromPtr(r.InvocationCount),
			"enable_count":            int64FromPtr(r.EnableCount),
			"estimated_overage_spend": stringFromPtr(r.EstimatedOverageSpend),
			"attributed_list_price":   stringFromPtr(r.AttributedListPrice),
			"currency":                stringFromPtr(r.Currency),
			"share_status":            stringFromPtr(r.ShareStatus),
			"chat_distinct_conversation_skill_used_count":         int64FromPtr(r.ChatMetrics.DistinctConversationSkillUsedCount),
			"claude_code_distinct_session_skill_used_count":       int64FromPtr(r.ClaudeCodeMetrics.DistinctSessionSkillUsedCount),
			"cowork_distinct_session_skill_used_count":            int64FromPtr(r.CoworkMetrics.DistinctSessionSkillUsedCount),
			"office_excel_distinct_session_skill_used_count":      int64FromPtr(r.OfficeMetrics.Excel.DistinctSessionSkillUsedCount),
			"office_outlook_distinct_session_skill_used_count":    int64FromPtr(r.OfficeMetrics.Outlook.DistinctSessionSkillUsedCount),
			"office_powerpoint_distinct_session_skill_used_count": int64FromPtr(r.OfficeMetrics.PowerPoint.DistinctSessionSkillUsedCount),
			"office_word_distinct_session_skill_used_count":       int64FromPtr(r.OfficeMetrics.Word.DistinctSessionSkillUsedCount),
		}, r.Product, r.RBACGroupID, r.RBACGroupName, r.UserID))
	}
	cfg.Rows = objectList(&resp.Diagnostics, attrTypesFromSchema(skillRowAttrs()), vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
