package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAnalyticsUsageReportDataSource) }

var _ datasource.DataSourceWithConfigure = &analyticsUsageReportDataSource{}

// NewAnalyticsUsageReportDataSource returns the anthropic_analytics_usage_report data source.
func NewAnalyticsUsageReportDataSource() datasource.DataSource {
	return &analyticsUsageReportDataSource{}
}

type analyticsUsageReportDataSource struct{ client *client.Client }

type analyticsUsageReportModel struct {
	analyticsReportInputsModel
	Buckets                   types.List   `tfsdk:"buckets"`
	DataRefreshedAt           types.String `tfsdk:"data_refreshed_at"`
	OrganizationID            types.String `tfsdk:"organization_id"`
	TotalUncachedInputTokens  types.Int64  `tfsdk:"total_uncached_input_tokens"`
	TotalCacheReadInputTokens types.Int64  `tfsdk:"total_cache_read_input_tokens"`
	TotalOutputTokens         types.Int64  `tfsdk:"total_output_tokens"`
	TotalRequests             types.Int64  `tfsdk:"total_requests"`
}

func (d *analyticsUsageReportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_usage_report"
}

func usageBucketAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"starting_at": dsString("Bucket start (inclusive)."),
		"ending_at":   dsString("Bucket end (exclusive)."),
		"results": schema.ListNestedAttribute{MarkdownDescription: "One row per group within the bucket.", Computed: true,
			NestedObject: schema.NestedAttributeObject{Attributes: usageMetricAttrs(map[string]schema.Attribute{})}},
	}
}

func (d *analyticsUsageReportDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := analyticsReportInputs(usageGroupBy, "")
	attrs["buckets"] = schema.ListNestedAttribute{MarkdownDescription: "Time buckets, oldest first.", Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: usageBucketAttrs()}}
	attrs["data_refreshed_at"] = dsString("When the underlying export was refreshed; data after it may be incomplete.")
	attrs["organization_id"] = dsString("Organization id.")
	attrs["total_uncached_input_tokens"] = dsInt64("Sum over every row.")
	attrs["total_cache_read_input_tokens"] = dsInt64("Sum over every row.")
	attrs["total_output_tokens"] = dsInt64("Sum over every row.")
	attrs["total_requests"] = dsInt64("Sum over every row (null requests count as zero).")
	resp.Schema = schema.Schema{
		MarkdownDescription: "Token usage over time across Claude products for a Claude Enterprise organization (usage-based plans; " +
			"seat-based plans report usage credits only)." + analyticsNote,
		Attributes: attrs,
	}
}

func (d *analyticsUsageReportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsUsageReportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsUsageReportModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := cfg.params(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	rep, err := d.client.GetAnalyticsUsageReport(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics usage report", err)
		return
	}
	resultTypes := attrTypesFromSchema(usageMetricAttrs(map[string]schema.Attribute{}))
	bucketTypes := map[string]attr.Type{"starting_at": types.StringType, "ending_at": types.StringType, "results": types.ListType{ElemType: types.ObjectType{AttrTypes: resultTypes}}}
	var tUncached, tCache, tOut, tReq int64
	buckets := make([]map[string]attr.Value, 0, len(rep.Data))
	for _, b := range rep.Data {
		rows := make([]map[string]attr.Value, 0, len(b.Results))
		for i := range b.Results {
			r := &b.Results[i]
			tUncached += r.UncachedInputTokens
			tCache += r.CacheReadInputTokens
			tOut += r.OutputTokens
			if r.Requests != nil {
				tReq += *r.Requests
			}
			rows = append(rows, usageMetricValues(map[string]attr.Value{}, r))
		}
		buckets = append(buckets, map[string]attr.Value{
			"starting_at": types.StringValue(b.StartingAt), "ending_at": types.StringValue(b.EndingAt),
			"results": objectList(&resp.Diagnostics, resultTypes, rows),
		})
	}
	cfg.Buckets = objectList(&resp.Diagnostics, bucketTypes, buckets)
	cfg.DataRefreshedAt = stringFromPtr(rep.DataRefreshedAt)
	cfg.OrganizationID = types.StringValue(rep.OrganizationID)
	cfg.TotalUncachedInputTokens = types.Int64Value(tUncached)
	cfg.TotalCacheReadInputTokens = types.Int64Value(tCache)
	cfg.TotalOutputTokens = types.Int64Value(tOut)
	cfg.TotalRequests = types.Int64Value(tReq)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
