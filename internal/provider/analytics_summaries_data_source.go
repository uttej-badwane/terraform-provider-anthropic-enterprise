package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAnalyticsSummariesDataSource) }

var _ datasource.DataSourceWithConfigure = &analyticsSummariesDataSource{}

// NewAnalyticsSummariesDataSource returns the anthropic_analytics_summaries data source.
func NewAnalyticsSummariesDataSource() datasource.DataSource { return &analyticsSummariesDataSource{} }

type analyticsSummariesDataSource struct{ client *client.Client }

type analyticsSummariesModel struct {
	StartingDate types.String `tfsdk:"starting_date"`
	EndingDate   types.String `tfsdk:"ending_date"`
	RBACGroupIDs types.List   `tfsdk:"rbac_group_ids"`
	Summaries    types.List   `tfsdk:"summaries"`
}

// summaryInt64Fields maps attribute names to accessors, keeping schema and flattening in sync.
var summaryInt64Fields = []struct {
	name string
	desc string
	get  func(*client.AnalyticsSummary) int64
}{
	{"assigned_seat_count", "Seats assigned.", func(s *client.AnalyticsSummary) int64 { return s.AssignedSeatCount }},
	{"pending_invite_count", "Pending invitations.", func(s *client.AnalyticsSummary) int64 { return s.PendingInviteCount }},
	{"daily_active_user_count", "Daily active users.", func(s *client.AnalyticsSummary) int64 { return s.DailyActiveUserCount }},
	{"weekly_active_user_count", "Weekly active users.", func(s *client.AnalyticsSummary) int64 { return s.WeeklyActiveUserCount }},
	{"monthly_active_user_count", "Monthly active users.", func(s *client.AnalyticsSummary) int64 { return s.MonthlyActiveUserCount }},
	{"chat_daily_active_user_count", "Chat daily active users.", func(s *client.AnalyticsSummary) int64 { return s.ChatDailyActiveUserCount }},
	{"chat_weekly_active_user_count", "Chat weekly active users.", func(s *client.AnalyticsSummary) int64 { return s.ChatWeeklyActiveUserCount }},
	{"chat_monthly_active_user_count", "Chat monthly active users.", func(s *client.AnalyticsSummary) int64 { return s.ChatMonthlyActiveUserCount }},
	{"claude_code_daily_active_user_count", "Claude Code daily active users.", func(s *client.AnalyticsSummary) int64 { return s.ClaudeCodeDailyActiveUserCount }},
	{"claude_code_weekly_active_user_count", "Claude Code weekly active users.", func(s *client.AnalyticsSummary) int64 { return s.ClaudeCodeWeeklyActiveUserCount }},
	{"claude_code_monthly_active_user_count", "Claude Code monthly active users.", func(s *client.AnalyticsSummary) int64 { return s.ClaudeCodeMonthlyActiveUserCount }},
	{"cowork_daily_active_user_count", "Cowork daily active users.", func(s *client.AnalyticsSummary) int64 { return s.CoworkDailyActiveUserCount }},
	{"cowork_weekly_active_user_count", "Cowork weekly active users.", func(s *client.AnalyticsSummary) int64 { return s.CoworkWeeklyActiveUserCount }},
	{"cowork_monthly_active_user_count", "Cowork monthly active users.", func(s *client.AnalyticsSummary) int64 { return s.CoworkMonthlyActiveUserCount }},
	{"claude_design_daily_active_user_count", "Claude Design daily active users.", func(s *client.AnalyticsSummary) int64 { return s.ClaudeDesignDailyActiveUserCount }},
	{"claude_design_weekly_active_user_count", "Claude Design weekly active users.", func(s *client.AnalyticsSummary) int64 { return s.ClaudeDesignWeeklyActiveUserCount }},
	{"claude_design_monthly_active_user_count", "Claude Design monthly active users.", func(s *client.AnalyticsSummary) int64 { return s.ClaudeDesignMonthlyActiveUserCount }},
	{"office_agent_daily_active_user_count", "Office agent daily active users.", func(s *client.AnalyticsSummary) int64 { return s.OfficeAgentDailyActiveUserCount }},
	{"office_agent_weekly_active_user_count", "Office agent weekly active users.", func(s *client.AnalyticsSummary) int64 { return s.OfficeAgentWeeklyActiveUserCount }},
	{"office_agent_monthly_active_user_count", "Office agent monthly active users.", func(s *client.AnalyticsSummary) int64 { return s.OfficeAgentMonthlyActiveUserCount }},
	{"science_daily_active_user_count", "Science daily active users.", func(s *client.AnalyticsSummary) int64 { return s.ScienceDailyActiveUserCount }},
	{"science_weekly_active_user_count", "Science weekly active users.", func(s *client.AnalyticsSummary) int64 { return s.ScienceWeeklyActiveUserCount }},
	{"science_monthly_active_user_count", "Science monthly active users.", func(s *client.AnalyticsSummary) int64 { return s.ScienceMonthlyActiveUserCount }},
	{"science_entitled_user_count", "Users entitled to Science.", func(s *client.AnalyticsSummary) int64 { return s.ScienceEntitledUserCount }},
}

var summaryFloatFields = []struct {
	name string
	desc string
	get  func(*client.AnalyticsSummary) float64
}{
	{"daily_adoption_rate", "Daily active users divided by assigned seats.", func(s *client.AnalyticsSummary) float64 { return s.DailyAdoptionRate }},
	{"weekly_adoption_rate", "Weekly active users divided by assigned seats.", func(s *client.AnalyticsSummary) float64 { return s.WeeklyAdoptionRate }},
	{"monthly_adoption_rate", "Monthly active users divided by assigned seats.", func(s *client.AnalyticsSummary) float64 { return s.MonthlyAdoptionRate }},
}

func attrTypesSummary() map[string]attr.Type {
	m := map[string]attr.Type{"starting_at": types.StringType, "ending_at": types.StringType}
	for _, f := range summaryInt64Fields {
		m[f.name] = types.Int64Type
	}
	for _, f := range summaryFloatFields {
		m[f.name] = types.Float64Type
	}
	return m
}

func (d *analyticsSummariesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_summaries"
}

func (d *analyticsSummariesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	rowAttrs := map[string]schema.Attribute{
		"starting_at": dsString("Day start (inclusive)."),
		"ending_at":   dsString("Day end (exclusive)."),
	}
	for _, f := range summaryInt64Fields {
		rowAttrs[f.name] = dsInt64(f.desc)
	}
	for _, f := range summaryFloatFields {
		rowAttrs[f.name] = schema.Float64Attribute{MarkdownDescription: f.desc, Computed: true}
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Daily organization-wide engagement summaries (active users, seats, adoption) for a Claude Enterprise " +
			"organization. Data is available from 2026-01-01 with about a one-day lag. Requires `analytics_api_key` " +
			"(or an `enterprise_api_key` carrying `read:analytics`)." + reportNote,
		Attributes: map[string]schema.Attribute{
			"starting_date": schema.StringAttribute{
				MarkdownDescription: "First day, `YYYY-MM-DD`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(dateRegexp, "must be YYYY-MM-DD")},
			},
			"ending_date": schema.StringAttribute{
				MarkdownDescription: "Exclusive last day, `YYYY-MM-DD`. Defaults to the API default.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(dateRegexp, "must be YYYY-MM-DD")},
			},
			"rbac_group_ids": optionalStringList("Restrict to members of these RBAC groups."),
			"summaries": schema.ListNestedAttribute{
				MarkdownDescription: "One row per day.",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: rowAttrs},
			},
		},
	}
}

func (d *analyticsSummariesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsSummariesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsSummariesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := client.AnalyticsSummariesParams{
		StartingDate: cfg.StartingDate.ValueString(),
		EndingDate:   cfg.EndingDate.ValueString(),
		RBACGroupIDs: listStrings(&resp.Diagnostics, cfg.RBACGroupIDs),
	}
	rows, err := d.client.GetAnalyticsSummaries(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics summaries", err)
		return
	}
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		s := &rows[i]
		v := map[string]attr.Value{"starting_at": types.StringValue(s.StartingAt), "ending_at": types.StringValue(s.EndingAt)}
		for _, f := range summaryInt64Fields {
			v[f.name] = types.Int64Value(f.get(s))
		}
		for _, f := range summaryFloatFields {
			v[f.name] = types.Float64Value(f.get(s))
		}
		vals = append(vals, v)
	}
	cfg.Summaries = objectList(&resp.Diagnostics, attrTypesSummary(), vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
