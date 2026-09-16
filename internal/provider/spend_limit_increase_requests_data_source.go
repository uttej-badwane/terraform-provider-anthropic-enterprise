package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewSpendLimitIncreaseRequestsDataSource)
	registerDataSource(NewSpendLimitIncreaseRequestDataSource)
}

var attrTypesIncreaseRequest = map[string]attr.Type{
	"id":                   types.StringType,
	"status":               types.StringType,
	"period":               types.StringType,
	"created_at":           types.StringType,
	"resolved_at":          types.StringType,
	"actor_user_id":        types.StringType,
	"actor_email":          types.StringType,
	"actor_name":           types.StringType,
	"resolved_by_type":     types.StringType,
	"resolved_by_id":       types.StringType,
	"current_amount":       types.StringType,
	"currency":             types.StringType,
	"period_to_date_spend": types.StringType,
}

var increaseRequestAttributes = map[string]schema.Attribute{
	"id":                   dsString("Request id (`slir_...`)."),
	"status":               dsString("`pending`, `approved` or `denied`."),
	"period":               dsString("`daily`, `weekly` or `monthly`."),
	"created_at":           dsString("Creation timestamp."),
	"resolved_at":          dsString("Resolution timestamp; null while pending."),
	"actor_user_id":        dsString("Id of the requesting user."),
	"actor_email":          dsString("Email of the requesting user; null when unavailable."),
	"actor_name":           dsString("Name of the requesting user; null when unavailable."),
	"resolved_by_type":     dsString("`user_actor` or `scoped_api_key_actor`; null while pending."),
	"resolved_by_id":       dsString("User id or scoped API key id of the resolver; null while pending."),
	"current_amount":       dsString("Current effective cap in minor units (cents) at request time; null when unlimited or after resolution."),
	"currency":             dsString("Currency of the spend summary; null after resolution."),
	"period_to_date_spend": dsString("Spend so far in the period at request time; null after resolution."),
}

func increaseRequestValues(r *client.SpendLimitIncreaseRequest) map[string]attr.Value {
	v := map[string]attr.Value{
		"id":                   types.StringValue(r.ID),
		"status":               types.StringValue(r.Status),
		"period":               types.StringValue(r.Period),
		"created_at":           types.StringValue(r.CreatedAt),
		"resolved_at":          stringFromPtr(r.ResolvedAt),
		"actor_user_id":        stringFromPtr(r.Actor.UserID),
		"actor_email":          stringFromPtr(r.Actor.EmailAddress),
		"actor_name":           stringFromPtr(r.Actor.Name),
		"resolved_by_type":     types.StringNull(),
		"resolved_by_id":       types.StringNull(),
		"current_amount":       types.StringNull(),
		"currency":             types.StringNull(),
		"period_to_date_spend": types.StringNull(),
	}
	if r.ResolvedBy != nil {
		v["resolved_by_type"] = types.StringValue(r.ResolvedBy.Type)
		if r.ResolvedBy.UserID != nil {
			v["resolved_by_id"] = types.StringValue(*r.ResolvedBy.UserID)
		} else if r.ResolvedBy.ScopedAPIKeyID != nil {
			v["resolved_by_id"] = types.StringValue(*r.ResolvedBy.ScopedAPIKeyID)
		}
	}
	if r.SpendSummary != nil {
		v["current_amount"] = stringFromPtr(r.SpendSummary.Amount)
		v["currency"] = types.StringValue(r.SpendSummary.Currency)
		v["period_to_date_spend"] = types.StringValue(r.SpendSummary.PeriodToDateSpend)
	}
	return v
}

// --- anthropic_spend_limit_increase_requests --------------------------------

var _ datasource.DataSourceWithConfigure = &increaseRequestsDataSource{}

// NewSpendLimitIncreaseRequestsDataSource returns the anthropic_spend_limit_increase_requests data source.
func NewSpendLimitIncreaseRequestsDataSource() datasource.DataSource {
	return &increaseRequestsDataSource{}
}

type increaseRequestsDataSource struct{ client *client.Client }

type increaseRequestsModel struct {
	Statuses types.List `tfsdk:"statuses"`
	ActorIDs types.List `tfsdk:"actor_ids"`
	Requests types.List `tfsdk:"requests"`
}

func (d *increaseRequestsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spend_limit_increase_requests"
}

func (d *increaseRequestsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists members' requests for a higher spend limit in a Claude Enterprise organization. " +
			"Approving or denying is a one-shot action outside Terraform. Requires `enterprise_api_key` with `read:spend_limits`.",
		Attributes: map[string]schema.Attribute{
			"statuses": schema.ListAttribute{
				MarkdownDescription: "Restrict to these statuses (`pending`, `approved`, `denied`).",
				ElementType:         types.StringType,
				Optional:            true,
				Validators:          []validator.List{listvalidator.ValueStringsAre(stringvalidator.OneOf("pending", "approved", "denied"))},
			},
			"actor_ids": schema.ListAttribute{
				MarkdownDescription: "Restrict to requests made by these user ids.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"requests": schema.ListNestedAttribute{
				MarkdownDescription: "Increase requests.",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: increaseRequestAttributes},
			},
		},
	}
}

func (d *increaseRequestsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *increaseRequestsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg increaseRequestsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts := client.IncreaseRequestListOptions{
		Statuses: listStrings(&resp.Diagnostics, cfg.Statuses),
		ActorIDs: listStrings(&resp.Diagnostics, cfg.ActorIDs),
	}
	rows, err := d.client.ListSpendLimitIncreaseRequests(ctx, opts)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing spend limit increase requests", err)
		return
	}
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		vals = append(vals, increaseRequestValues(&rows[i]))
	}
	cfg.Requests = objectList(&resp.Diagnostics, attrTypesIncreaseRequest, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_spend_limit_increase_request ---------------------------------

var _ datasource.DataSourceWithConfigure = &increaseRequestDataSource{}

// NewSpendLimitIncreaseRequestDataSource returns the anthropic_spend_limit_increase_request data source.
func NewSpendLimitIncreaseRequestDataSource() datasource.DataSource {
	return &increaseRequestDataSource{}
}

type increaseRequestDataSource struct{ client *client.Client }

func (d *increaseRequestDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spend_limit_increase_request"
}

func (d *increaseRequestDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{}
	for k, v := range increaseRequestAttributes {
		attrs[k] = v
	}
	attrs["id"] = schema.StringAttribute{MarkdownDescription: "Request id (`slir_...`).", Required: true}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one spend limit increase request. Requires `enterprise_api_key` with `read:spend_limits`.",
		Attributes:          attrs,
	}
}

func (d *increaseRequestDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *increaseRequestDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r, err := d.client.GetSpendLimitIncreaseRequest(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading spend limit increase request", err)
		return
	}
	obj, diags := types.ObjectValue(attrTypesIncreaseRequest, increaseRequestValues(r))
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
