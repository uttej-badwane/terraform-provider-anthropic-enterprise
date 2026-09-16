package provider

import (
	"context"
	"regexp"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewClaudeCodeUsageReportDataSource) }

var _ datasource.DataSourceWithConfigure = &claudeCodeUsageReportDataSource{}

// NewClaudeCodeUsageReportDataSource returns the anthropic_claude_code_usage_report data source.
func NewClaudeCodeUsageReportDataSource() datasource.DataSource {
	return &claudeCodeUsageReportDataSource{}
}

type claudeCodeUsageReportDataSource struct{ client *client.Client }

type claudeCodeUsageReportModel struct {
	Date    types.String `tfsdk:"date"`
	Records types.List   `tfsdk:"records"`
}

var dateRegexp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

var attrTypesModelBreakdown = map[string]attr.Type{
	"model":                   types.StringType,
	"input_tokens":            types.Int64Type,
	"output_tokens":           types.Int64Type,
	"cache_read_tokens":       types.Int64Type,
	"cache_creation_tokens":   types.Int64Type,
	"estimated_cost_amount":   types.Float64Type,
	"estimated_cost_currency": types.StringType,
}

var attrTypesToolAction = map[string]attr.Type{
	"tool":     types.StringType,
	"accepted": types.Int64Type,
	"rejected": types.Int64Type,
}

var attrTypesClaudeCodeRecord = map[string]attr.Type{
	"actor_type":         types.StringType,
	"actor_email":        types.StringType,
	"actor_api_key_name": types.StringType,
	"customer_type":      types.StringType,
	"subscription_type":  types.StringType,
	"is_remote":          types.BoolType,
	"terminal_type":      types.StringType,
	"organization_id":    types.StringType,
	"num_sessions":       types.Int64Type,
	"commits":            types.Int64Type,
	"pull_requests":      types.Int64Type,
	"lines_added":        types.Int64Type,
	"lines_removed":      types.Int64Type,
	"model_breakdown":    types.ListType{ElemType: types.ObjectType{AttrTypes: attrTypesModelBreakdown}},
	"tool_actions":       types.ListType{ElemType: types.ObjectType{AttrTypes: attrTypesToolAction}},
}

func (d *claudeCodeUsageReportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_claude_code_usage_report"
}

func (d *claudeCodeUsageReportDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Daily Claude Code productivity metrics per user or API key for one UTC day. " +
			"Requires `admin_api_key` or `oauth_token`." + reportNote,
		Attributes: map[string]schema.Attribute{
			"date": schema.StringAttribute{
				MarkdownDescription: "UTC day to report, `YYYY-MM-DD`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(dateRegexp, "must be YYYY-MM-DD")},
			},
			"records": schema.ListNestedAttribute{
				MarkdownDescription: "One row per actor; remote and local usage are separate rows.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"actor_type":         dsString("`user_actor` or `api_actor`."),
					"actor_email":        dsString("User email; null for API key actors."),
					"actor_api_key_name": dsString("API key name; null for user actors."),
					"customer_type":      dsString("`api` or `subscription`."),
					"subscription_type":  dsString("`enterprise` or `team`; null for API customers."),
					"is_remote":          dsBool("Whether the usage came from remote (web) sessions."),
					"terminal_type":      dsString("Terminal or environment used."),
					"organization_id":    dsString("Owning organization id."),
					"num_sessions":       dsInt64("Distinct sessions."),
					"commits":            dsInt64("Commits created through Claude Code."),
					"pull_requests":      dsInt64("Pull requests created through Claude Code."),
					"lines_added":        dsInt64("Lines of code added."),
					"lines_removed":      dsInt64("Lines of code removed."),
					"model_breakdown": schema.ListNestedAttribute{
						MarkdownDescription: "Token usage and estimated cost per model.",
						Computed:            true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"model":                   dsString("Model name."),
							"input_tokens":            dsInt64("Input tokens."),
							"output_tokens":           dsInt64("Output tokens."),
							"cache_read_tokens":       dsInt64("Cache read tokens."),
							"cache_creation_tokens":   dsInt64("Cache creation tokens."),
							"estimated_cost_amount":   schema.Float64Attribute{MarkdownDescription: "Estimated cost in minor units (cents).", Computed: true},
							"estimated_cost_currency": dsString("Currency code of the estimate."),
						}},
					},
					"tool_actions": schema.ListNestedAttribute{
						MarkdownDescription: "Accepted and rejected proposals per tool, sorted by tool name.",
						Computed:            true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"tool":     dsString("Tool name (for example `edit_tool`)."),
							"accepted": dsInt64("Accepted proposals."),
							"rejected": dsInt64("Rejected proposals."),
						}},
					},
				}},
			},
		},
	}
}

func (d *claudeCodeUsageReportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *claudeCodeUsageReportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg claudeCodeUsageReportModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rows, err := d.client.GetClaudeCodeUsageReport(ctx, cfg.Date.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading Claude Code usage report", err)
		return
	}
	vals := make([]map[string]attr.Value, 0, len(rows))
	for _, r := range rows {
		models := make([]map[string]attr.Value, 0, len(r.ModelBreakdown))
		for _, m := range r.ModelBreakdown {
			models = append(models, map[string]attr.Value{
				"model":                   types.StringValue(m.Model),
				"input_tokens":            types.Int64Value(m.Tokens.Input),
				"output_tokens":           types.Int64Value(m.Tokens.Output),
				"cache_read_tokens":       types.Int64Value(m.Tokens.CacheRead),
				"cache_creation_tokens":   types.Int64Value(m.Tokens.CacheCreation),
				"estimated_cost_amount":   types.Float64Value(m.EstimatedCost.Amount),
				"estimated_cost_currency": types.StringValue(m.EstimatedCost.Currency),
			})
		}
		toolNames := make([]string, 0, len(r.ToolActions))
		for name := range r.ToolActions {
			toolNames = append(toolNames, name)
		}
		sort.Strings(toolNames)
		tools := make([]map[string]attr.Value, 0, len(toolNames))
		for _, name := range toolNames {
			tools = append(tools, map[string]attr.Value{
				"tool":     types.StringValue(name),
				"accepted": types.Int64Value(r.ToolActions[name].Accepted),
				"rejected": types.Int64Value(r.ToolActions[name].Rejected),
			})
		}
		vals = append(vals, map[string]attr.Value{
			"actor_type":         types.StringValue(r.Actor.Type),
			"actor_email":        stringFromPtr(r.Actor.EmailAddress),
			"actor_api_key_name": stringFromPtr(r.Actor.APIKeyName),
			"customer_type":      types.StringValue(r.CustomerType),
			"subscription_type":  stringFromPtr(r.SubscriptionType),
			"is_remote":          types.BoolValue(r.IsRemote),
			"terminal_type":      types.StringValue(r.TerminalType),
			"organization_id":    types.StringValue(r.OrganizationID),
			"num_sessions":       types.Int64Value(r.CoreMetrics.NumSessions),
			"commits":            types.Int64Value(r.CoreMetrics.CommitsByClaudeCode),
			"pull_requests":      types.Int64Value(r.CoreMetrics.PullRequestsByClaudeCode),
			"lines_added":        types.Int64Value(r.CoreMetrics.LinesOfCode.Added),
			"lines_removed":      types.Int64Value(r.CoreMetrics.LinesOfCode.Removed),
			"model_breakdown":    objectList(&resp.Diagnostics, attrTypesModelBreakdown, models),
			"tool_actions":       objectList(&resp.Diagnostics, attrTypesToolAction, tools),
		})
	}
	cfg.Records = objectList(&resp.Diagnostics, attrTypesClaudeCodeRecord, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
