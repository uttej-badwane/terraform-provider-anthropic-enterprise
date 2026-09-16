package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewUsageReportDataSource) }

var _ datasource.DataSourceWithConfigure = &usageReportDataSource{}

// NewUsageReportDataSource returns the anthropic_usage_report data source.
func NewUsageReportDataSource() datasource.DataSource { return &usageReportDataSource{} }

type usageReportDataSource struct{ client *client.Client }

type usageReportModel struct {
	StartingAt                types.String `tfsdk:"starting_at"`
	EndingAt                  types.String `tfsdk:"ending_at"`
	BucketWidth               types.String `tfsdk:"bucket_width"`
	GroupBy                   types.List   `tfsdk:"group_by"`
	APIKeyIDs                 types.List   `tfsdk:"api_key_ids"`
	WorkspaceIDs              types.List   `tfsdk:"workspace_ids"`
	Models                    types.List   `tfsdk:"models"`
	ServiceTiers              types.List   `tfsdk:"service_tiers"`
	ContextWindows            types.List   `tfsdk:"context_windows"`
	InferenceGeos             types.List   `tfsdk:"inference_geos"`
	ServiceAccountIDs         types.List   `tfsdk:"service_account_ids"`
	AccountIDs                types.List   `tfsdk:"account_ids"`
	Buckets                   types.List   `tfsdk:"buckets"`
	TotalUncachedInputTokens  types.Int64  `tfsdk:"total_uncached_input_tokens"`
	TotalCacheReadInputTokens types.Int64  `tfsdk:"total_cache_read_input_tokens"`
	TotalOutputTokens         types.Int64  `tfsdk:"total_output_tokens"`
}

var attrTypesUsageResult = map[string]attr.Type{
	"uncached_input_tokens":          types.Int64Type,
	"cache_read_input_tokens":        types.Int64Type,
	"cache_creation_1h_input_tokens": types.Int64Type,
	"cache_creation_5m_input_tokens": types.Int64Type,
	"output_tokens":                  types.Int64Type,
	"web_search_requests":            types.Int64Type,
	"model":                          types.StringType,
	"workspace_id":                   types.StringType,
	"api_key_id":                     types.StringType,
	"account_id":                     types.StringType,
	"service_account_id":             types.StringType,
	"service_tier":                   types.StringType,
	"context_window":                 types.StringType,
	"inference_geo":                  types.StringType,
}

var attrTypesUsageBucket = map[string]attr.Type{
	"starting_at": types.StringType,
	"ending_at":   types.StringType,
	"results":     types.ListType{ElemType: types.ObjectType{AttrTypes: attrTypesUsageResult}},
}

func optionalStringList(desc string, validators ...validator.List) schema.ListAttribute {
	return schema.ListAttribute{MarkdownDescription: desc, ElementType: types.StringType, Optional: true, Validators: validators}
}

func (d *usageReportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_usage_report"
}

func (d *usageReportDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Token usage of the Messages API for a Console organization, bucketed over time and optionally " +
			"grouped by dimension. Requires `admin_api_key` or `oauth_token`." + reportNote,
		Attributes: map[string]schema.Attribute{
			"starting_at": schema.StringAttribute{MarkdownDescription: "Inclusive start, RFC 3339. Buckets snap to the start of the minute/hour/day in UTC.", Required: true},
			"ending_at":   schema.StringAttribute{MarkdownDescription: "Exclusive end, RFC 3339. Defaults to the API default window.", Optional: true},
			"bucket_width": schema.StringAttribute{
				MarkdownDescription: "`1d` (max 31 buckets), `1h` (max 168) or `1m` (max 1440). Defaults to `1d`.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("1d", "1h", "1m")},
			},
			"group_by": optionalStringList("Dimensions to group by: `account_id`, `api_key_id`, `context_window`, `inference_geo`, `model`, `service_account_id`, `service_tier`, `workspace_id`.",
				listvalidator.ValueStringsAre(stringvalidator.OneOf("account_id", "api_key_id", "context_window", "inference_geo", "model", "service_account_id", "service_tier", "workspace_id"))),
			"api_key_ids":         optionalStringList("Restrict to these API key ids."),
			"workspace_ids":       optionalStringList("Restrict to these workspace ids."),
			"models":              optionalStringList("Restrict to these models."),
			"service_tiers":       optionalStringList("Restrict to these service tiers (`standard`, `batch`, `priority`, `priority_on_demand`, `flex`, `flex_discount`)."),
			"context_windows":     optionalStringList("Restrict to these context windows (`0-200k`, `200k-1M`)."),
			"inference_geos":      optionalStringList("Restrict to these inference geographies (`global`, `us`, `not_available`)."),
			"service_account_ids": optionalStringList("Restrict to these service account ids."),
			"account_ids":         optionalStringList("Restrict to these user account ids."),
			"buckets": schema.ListNestedAttribute{
				MarkdownDescription: "Time buckets, oldest first. Buckets with no usage have an empty `results` list.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"starting_at": dsString("Bucket start (inclusive)."),
					"ending_at":   dsString("Bucket end (exclusive)."),
					"results": schema.ListNestedAttribute{
						MarkdownDescription: "One row per group within the bucket.",
						Computed:            true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"uncached_input_tokens":          dsInt64("Uncached input tokens."),
							"cache_read_input_tokens":        dsInt64("Input tokens read from cache."),
							"cache_creation_1h_input_tokens": dsInt64("Input tokens written to the 1 hour cache."),
							"cache_creation_5m_input_tokens": dsInt64("Input tokens written to the 5 minute cache."),
							"output_tokens":                  dsInt64("Output tokens."),
							"web_search_requests":            dsInt64("Server-side web search requests."),
							"model":                          dsString("Model; null unless grouped by model."),
							"workspace_id":                   dsString("Workspace id; null unless grouped, and null for the default workspace."),
							"api_key_id":                     dsString("API key id; null unless grouped, and null for Console usage."),
							"account_id":                     dsString("User account id; null unless grouped."),
							"service_account_id":             dsString("Service account id; null unless grouped."),
							"service_tier":                   dsString("Service tier; null unless grouped."),
							"context_window":                 dsString("Context window; null unless grouped."),
							"inference_geo":                  dsString("Inference geography; null unless grouped."),
						}},
					},
				}},
			},
			"total_uncached_input_tokens":   dsInt64("Sum of uncached input tokens across every row."),
			"total_cache_read_input_tokens": dsInt64("Sum of cache-read input tokens across every row."),
			"total_output_tokens":           dsInt64("Sum of output tokens across every row."),
		},
	}
}

func (d *usageReportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *usageReportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg usageReportModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := client.UsageReportParams{
		StartingAt:        cfg.StartingAt.ValueString(),
		EndingAt:          cfg.EndingAt.ValueString(),
		BucketWidth:       cfg.BucketWidth.ValueString(),
		GroupBy:           listStrings(&resp.Diagnostics, cfg.GroupBy),
		APIKeyIDs:         listStrings(&resp.Diagnostics, cfg.APIKeyIDs),
		WorkspaceIDs:      listStrings(&resp.Diagnostics, cfg.WorkspaceIDs),
		Models:            listStrings(&resp.Diagnostics, cfg.Models),
		ServiceTiers:      listStrings(&resp.Diagnostics, cfg.ServiceTiers),
		ContextWindows:    listStrings(&resp.Diagnostics, cfg.ContextWindows),
		InferenceGeos:     listStrings(&resp.Diagnostics, cfg.InferenceGeos),
		ServiceAccountIDs: listStrings(&resp.Diagnostics, cfg.ServiceAccountIDs),
		AccountIDs:        listStrings(&resp.Diagnostics, cfg.AccountIDs),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	buckets, err := d.client.GetUsageReport(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading usage report", err)
		return
	}

	var totalUncached, totalCacheRead, totalOutput int64
	bucketVals := make([]map[string]attr.Value, 0, len(buckets))
	for _, b := range buckets {
		rows := make([]map[string]attr.Value, 0, len(b.Results))
		for _, r := range b.Results {
			totalUncached += r.UncachedInputTokens
			totalCacheRead += r.CacheReadInputTokens
			totalOutput += r.OutputTokens
			rows = append(rows, map[string]attr.Value{
				"uncached_input_tokens":          types.Int64Value(r.UncachedInputTokens),
				"cache_read_input_tokens":        types.Int64Value(r.CacheReadInputTokens),
				"cache_creation_1h_input_tokens": types.Int64Value(r.CacheCreation.Ephemeral1hInputTokens),
				"cache_creation_5m_input_tokens": types.Int64Value(r.CacheCreation.Ephemeral5mInputTokens),
				"output_tokens":                  types.Int64Value(r.OutputTokens),
				"web_search_requests":            types.Int64Value(r.ServerToolUse.WebSearchRequests),
				"model":                          stringFromPtr(r.Model),
				"workspace_id":                   stringFromPtr(r.WorkspaceID),
				"api_key_id":                     stringFromPtr(r.APIKeyID),
				"account_id":                     stringFromPtr(r.AccountID),
				"service_account_id":             stringFromPtr(r.ServiceAccountID),
				"service_tier":                   stringFromPtr(r.ServiceTier),
				"context_window":                 stringFromPtr(r.ContextWindow),
				"inference_geo":                  stringFromPtr(r.InferenceGeo),
			})
		}
		bucketVals = append(bucketVals, map[string]attr.Value{
			"starting_at": types.StringValue(b.StartingAt),
			"ending_at":   types.StringValue(b.EndingAt),
			"results":     objectList(&resp.Diagnostics, attrTypesUsageResult, rows),
		})
	}
	cfg.Buckets = objectList(&resp.Diagnostics, attrTypesUsageBucket, bucketVals)
	cfg.TotalUncachedInputTokens = types.Int64Value(totalUncached)
	cfg.TotalCacheReadInputTokens = types.Int64Value(totalCacheRead)
	cfg.TotalOutputTokens = types.Int64Value(totalOutput)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
