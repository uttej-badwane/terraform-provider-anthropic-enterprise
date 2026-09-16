package provider

import (
	"context"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAnalyticsCostReportDataSource) }

var _ datasource.DataSourceWithConfigure = &analyticsCostReportDataSource{}

// NewAnalyticsCostReportDataSource returns the anthropic_analytics_cost_report data source.
func NewAnalyticsCostReportDataSource() datasource.DataSource {
	return &analyticsCostReportDataSource{}
}

type analyticsCostReportDataSource struct{ client *client.Client }

type analyticsCostReportModel struct {
	analyticsReportInputsModel
	Buckets         types.List   `tfsdk:"buckets"`
	DataRefreshedAt types.String `tfsdk:"data_refreshed_at"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	TotalAmount     types.String `tfsdk:"total_amount"`
	TotalListAmount types.String `tfsdk:"total_list_amount"`
}

func (d *analyticsCostReportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_cost_report"
}

func (d *analyticsCostReportDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := analyticsReportInputs(costGroupBy, "")
	attrs["buckets"] = schema.ListNestedAttribute{MarkdownDescription: "Time buckets, oldest first.", Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"starting_at": dsString("Bucket start (inclusive)."),
			"ending_at":   dsString("Bucket end (exclusive)."),
			"results": schema.ListNestedAttribute{MarkdownDescription: "One row per group within the bucket.", Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: costMetricAttrs(map[string]schema.Attribute{})}},
		}}}
	attrs["data_refreshed_at"] = dsString("When the underlying export was refreshed; data after it may be incomplete.")
	attrs["organization_id"] = dsString("Organization id.")
	attrs["total_amount"] = dsString("Sum of every `amount`, fractional cents with six decimals.")
	attrs["total_list_amount"] = dsString("Sum of every `list_amount`, fractional cents with six decimals.")
	resp.Schema = schema.Schema{
		MarkdownDescription: "Cost over time across Claude products for a Claude Enterprise organization (usage-based plans; " +
			"seat-based plans report usage credits only). Amounts are fractional cents as decimal strings." + analyticsNote,
		Attributes: attrs,
	}
}

func (d *analyticsCostReportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsCostReportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsCostReportModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := cfg.params(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	rep, err := d.client.GetAnalyticsCostReport(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics cost report", err)
		return
	}
	resultTypes := attrTypesFromSchema(costMetricAttrs(map[string]schema.Attribute{}))
	bucketTypes := map[string]attr.Type{"starting_at": types.StringType, "ending_at": types.StringType, "results": types.ListType{ElemType: types.ObjectType{AttrTypes: resultTypes}}}
	total, totalList := new(big.Rat), new(big.Rat)
	buckets := make([]map[string]attr.Value, 0, len(rep.Data))
	for _, b := range rep.Data {
		rows := make([]map[string]attr.Value, 0, len(b.Results))
		for i := range b.Results {
			r := &b.Results[i]
			addRat(&resp.Diagnostics, total, r.Amount, "amount")
			addRat(&resp.Diagnostics, totalList, r.ListAmount, "list_amount")
			rows = append(rows, costMetricValues(map[string]attr.Value{}, r))
		}
		buckets = append(buckets, map[string]attr.Value{
			"starting_at": types.StringValue(b.StartingAt), "ending_at": types.StringValue(b.EndingAt),
			"results": objectList(&resp.Diagnostics, resultTypes, rows),
		})
	}
	cfg.Buckets = objectList(&resp.Diagnostics, bucketTypes, buckets)
	cfg.DataRefreshedAt = stringFromPtr(rep.DataRefreshedAt)
	cfg.OrganizationID = types.StringValue(rep.OrganizationID)
	cfg.TotalAmount = types.StringValue(formatCents(total))
	cfg.TotalListAmount = types.StringValue(formatCents(totalList))
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
