package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewAnalyticsUserUsageReportDataSource)
	registerDataSource(NewAnalyticsUserCostReportDataSource)
}

// analyticsUserReportModel is shared by the per-user usage and cost reports.
type analyticsUserReportModel struct {
	analyticsReportInputsModel
	Order               types.String `tfsdk:"order"`
	OrderBy             types.String `tfsdk:"order_by"`
	ExcludeDeletedUsers types.Bool   `tfsdk:"exclude_deleted_users"`
	LimitRows           types.Int64  `tfsdk:"limit_rows"`
	Rows                types.List   `tfsdk:"rows"`
	DataRefreshedAt     types.String `tfsdk:"data_refreshed_at"`
	OrganizationID      types.String `tfsdk:"organization_id"`
}

func (m *analyticsUserReportModel) reportParams(diags *diag.Diagnostics) client.AnalyticsReportParams {
	p := m.params(diags)
	p.Order = m.Order.ValueString()
	p.OrderBy = m.OrderBy.ValueString()
	p.ExcludeDeletedUsers = m.ExcludeDeletedUsers.ValueBool()
	return p
}

func userReportInputs(groupBy []string, orderBy []string) map[string]schema.Attribute {
	attrs := analyticsReportInputs(groupBy, " When set, rows carry `starting_at`/`ending_at` and one user may span several rows; `1m` allows at most a 24 hour range.")
	attrs["order"] = orderAttr
	attrs["order_by"] = schema.StringAttribute{MarkdownDescription: "Metric to rank users by.", Optional: true,
		Validators: []validator.String{stringvalidator.OneOf(orderBy...)}}
	attrs["exclude_deleted_users"] = schema.BoolAttribute{MarkdownDescription: "Omit rows for deleted users.", Optional: true}
	attrs["limit_rows"] = limitRowsAttr
	attrs["data_refreshed_at"] = dsString("When the underlying export was refreshed; data after it may be incomplete.")
	attrs["organization_id"] = dsString("Organization id.")
	return attrs
}

func actorAttrs(m map[string]schema.Attribute) map[string]schema.Attribute {
	m["user_id"] = dsString("User id.")
	m["email"] = dsString("User email; null for deleted users.")
	m["name"] = dsString("User name; null for deleted users.")
	m["deleted"] = dsBool("Whether the user has been removed from the organization.")
	m["starting_at"] = dsString("Bucket start; null unless `bucket_width` is set.")
	m["ending_at"] = dsString("Bucket end; null unless `bucket_width` is set.")
	return m
}

func actorValues(m map[string]attr.Value, a client.AnalyticsActor, start, end *string) map[string]attr.Value {
	m["user_id"] = types.StringValue(a.UserID)
	m["email"] = stringFromPtr(a.Email)
	m["name"] = stringFromPtr(a.Name)
	m["deleted"] = types.BoolValue(a.Deleted)
	m["starting_at"] = stringFromPtr(start)
	m["ending_at"] = stringFromPtr(end)
	return m
}

// --- user usage --------------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &analyticsUserUsageReportDataSource{}

// NewAnalyticsUserUsageReportDataSource returns the anthropic_analytics_user_usage_report data source.
func NewAnalyticsUserUsageReportDataSource() datasource.DataSource {
	return &analyticsUserUsageReportDataSource{}
}

type analyticsUserUsageReportDataSource struct{ client *client.Client }

func userUsageRowAttrs() map[string]schema.Attribute {
	m := usageMetricAttrs(actorAttrs(map[string]schema.Attribute{}))
	m["total_tokens"] = dsInt64("Sum of uncached input, cache read and output tokens.")
	return m
}

func (d *analyticsUserUsageReportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_user_usage_report"
}

func (d *analyticsUserUsageReportDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := userReportInputs(usageGroupBy, []string{"total_tokens", "uncached_input_tokens", "output_tokens", "requests"})
	attrs["rows"] = schema.ListNestedAttribute{MarkdownDescription: "One row per user (times buckets and group dimensions), ranked by `order_by`.", Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: userUsageRowAttrs()}}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Per-user token usage for a Claude Enterprise organization, ranked by a consumption metric. Only seat " +
			"users are attributed." + analyticsNote,
		Attributes: attrs,
	}
}

func (d *analyticsUserUsageReportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsUserUsageReportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsUserReportModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := cfg.reportParams(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	rep, err := d.client.ListAnalyticsUserUsage(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics user usage report", err)
		return
	}
	rows := capRows(rep.Data, cfg.LimitRows)
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		v := usageMetricValues(actorValues(map[string]attr.Value{}, r.Actor, r.StartingAt, r.EndingAt), &r.AnalyticsUsageResult)
		v["total_tokens"] = types.Int64Value(r.TotalTokens)
		vals = append(vals, v)
	}
	cfg.Rows = objectList(&resp.Diagnostics, attrTypesFromSchema(userUsageRowAttrs()), vals)
	cfg.DataRefreshedAt = stringFromPtr(rep.DataRefreshedAt)
	cfg.OrganizationID = types.StringValue(rep.OrganizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- user cost ---------------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &analyticsUserCostReportDataSource{}

// NewAnalyticsUserCostReportDataSource returns the anthropic_analytics_user_cost_report data source.
func NewAnalyticsUserCostReportDataSource() datasource.DataSource {
	return &analyticsUserCostReportDataSource{}
}

type analyticsUserCostReportDataSource struct{ client *client.Client }

func userCostRowAttrs() map[string]schema.Attribute {
	return costMetricAttrs(actorAttrs(map[string]schema.Attribute{}))
}

func (d *analyticsUserCostReportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_user_cost_report"
}

func (d *analyticsUserCostReportDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := userReportInputs(costGroupBy, []string{"amount", "list_amount"})
	attrs["rows"] = schema.ListNestedAttribute{MarkdownDescription: "One row per user (times buckets and group dimensions), ranked by `order_by`.", Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: userCostRowAttrs()}}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Per-user cost for a Claude Enterprise organization, ranked by post-discount or list amount. Amounts are " +
			"fractional cents as decimal strings." + analyticsNote,
		Attributes: attrs,
	}
}

func (d *analyticsUserCostReportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsUserCostReportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsUserReportModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := cfg.reportParams(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	rep, err := d.client.ListAnalyticsUserCost(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics user cost report", err)
		return
	}
	rows := capRows(rep.Data, cfg.LimitRows)
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		vals = append(vals, costMetricValues(actorValues(map[string]attr.Value{}, r.Actor, r.StartingAt, r.EndingAt), &r.AnalyticsCostResult))
	}
	cfg.Rows = objectList(&resp.Diagnostics, attrTypesFromSchema(userCostRowAttrs()), vals)
	cfg.DataRefreshedAt = stringFromPtr(rep.DataRefreshedAt)
	cfg.OrganizationID = types.StringValue(rep.OrganizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
