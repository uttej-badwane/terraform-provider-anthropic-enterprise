package provider

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewCostReportDataSource) }

var _ datasource.DataSourceWithConfigure = &costReportDataSource{}

// NewCostReportDataSource returns the anthropic_cost_report data source.
func NewCostReportDataSource() datasource.DataSource { return &costReportDataSource{} }

type costReportDataSource struct{ client *client.Client }

type costReportModel struct {
	StartingAt  types.String `tfsdk:"starting_at"`
	EndingAt    types.String `tfsdk:"ending_at"`
	GroupBy     types.List   `tfsdk:"group_by"`
	Buckets     types.List   `tfsdk:"buckets"`
	TotalAmount types.String `tfsdk:"total_amount"`
}

var attrTypesCostResult = map[string]attr.Type{
	"amount":         types.StringType,
	"currency":       types.StringType,
	"workspace_id":   types.StringType,
	"description":    types.StringType,
	"cost_type":      types.StringType,
	"model":          types.StringType,
	"service_tier":   types.StringType,
	"token_type":     types.StringType,
	"context_window": types.StringType,
	"inference_geo":  types.StringType,
}

var attrTypesCostBucket = map[string]attr.Type{
	"starting_at": types.StringType,
	"ending_at":   types.StringType,
	"results":     types.ListType{ElemType: types.ObjectType{AttrTypes: attrTypesCostResult}},
}

func (d *costReportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cost_report"
}

func (d *costReportDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Daily cost of a Console organization in USD cents, optionally grouped by workspace or line-item " +
			"description. Priority Tier costs are not included. Requires `admin_api_key` or `oauth_token`." + reportNote,
		Attributes: map[string]schema.Attribute{
			"starting_at": schema.StringAttribute{MarkdownDescription: "Inclusive start, RFC 3339. Buckets are daily and snap to midnight UTC.", Required: true},
			"ending_at":   schema.StringAttribute{MarkdownDescription: "Exclusive end, RFC 3339. At most 31 days.", Optional: true},
			"group_by": optionalStringList("Dimensions to group by: `workspace_id`, `description`.",
				listvalidator.ValueStringsAre(stringvalidator.OneOf("workspace_id", "description"))),
			"buckets": schema.ListNestedAttribute{
				MarkdownDescription: "Daily buckets, oldest first.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"starting_at": dsString("Bucket start (inclusive)."),
					"ending_at":   dsString("Bucket end (exclusive)."),
					"results": schema.ListNestedAttribute{
						MarkdownDescription: "One row per group within the bucket.",
						Computed:            true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"amount":         dsString("Cost in minor units (cents) as a decimal string."),
							"currency":       dsString("Currency code, currently always `USD`."),
							"workspace_id":   dsString("Workspace id; null unless grouped, and null for the default workspace."),
							"description":    dsString("Line-item description; null unless grouped by description."),
							"cost_type":      dsString("`tokens`, `web_search`, `code_execution` or `session_usage`; null unless grouped by description."),
							"model":          dsString("Model; null unless grouped by description or for non-token costs."),
							"service_tier":   dsString("Service tier; null unless grouped by description."),
							"token_type":     dsString("Token type; null unless grouped by description."),
							"context_window": dsString("Context window; null unless grouped by description."),
							"inference_geo":  dsString("Inference geography; null unless grouped by description."),
						}},
					},
				}},
			},
			"total_amount": dsString("Sum of every `amount` across all buckets, as a decimal string in cents."),
		},
	}
}

func (d *costReportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *costReportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg costReportModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := client.CostReportParams{
		StartingAt: cfg.StartingAt.ValueString(),
		EndingAt:   cfg.EndingAt.ValueString(),
		GroupBy:    listStrings(&resp.Diagnostics, cfg.GroupBy),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	buckets, err := d.client.GetCostReport(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading cost report", err)
		return
	}

	total := new(big.Rat)
	bucketVals := make([]map[string]attr.Value, 0, len(buckets))
	for _, b := range buckets {
		rows := make([]map[string]attr.Value, 0, len(b.Results))
		for _, r := range b.Results {
			if amt, ok := new(big.Rat).SetString(r.Amount); ok {
				total.Add(total, amt)
			} else {
				resp.Diagnostics.AddWarning("Unparseable cost amount", fmt.Sprintf("amount %q was skipped when computing total_amount", r.Amount))
			}
			rows = append(rows, map[string]attr.Value{
				"amount":         types.StringValue(r.Amount),
				"currency":       types.StringValue(r.Currency),
				"workspace_id":   stringFromPtr(r.WorkspaceID),
				"description":    stringFromPtr(r.Description),
				"cost_type":      stringFromPtr(r.CostType),
				"model":          stringFromPtr(r.Model),
				"service_tier":   stringFromPtr(r.ServiceTier),
				"token_type":     stringFromPtr(r.TokenType),
				"context_window": stringFromPtr(r.ContextWindow),
				"inference_geo":  stringFromPtr(r.InferenceGeo),
			})
		}
		bucketVals = append(bucketVals, map[string]attr.Value{
			"starting_at": types.StringValue(b.StartingAt),
			"ending_at":   types.StringValue(b.EndingAt),
			"results":     objectList(&resp.Diagnostics, attrTypesCostResult, rows),
		})
	}
	cfg.Buckets = objectList(&resp.Diagnostics, attrTypesCostBucket, bucketVals)
	cfg.TotalAmount = types.StringValue(formatRat(total))
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// formatRat renders a rational with up to 6 decimals, trimming trailing zeros.
func formatRat(r *big.Rat) string {
	s := r.FloatString(6)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "" || s == "-" {
		return "0"
	}
	return s
}
