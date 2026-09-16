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

func init() { registerDataSource(NewSpendLimitsDataSource) }

var _ datasource.DataSourceWithConfigure = &spendLimitsDataSource{}

// NewSpendLimitsDataSource returns the anthropic_spend_limits data source.
func NewSpendLimitsDataSource() datasource.DataSource { return &spendLimitsDataSource{} }

type spendLimitsDataSource struct{ client *client.Client }

type spendLimitsModel struct {
	Periods     types.List `tfsdk:"periods"`
	UserIDs     types.List `tfsdk:"user_ids"`
	SpendLimits types.List `tfsdk:"spend_limits"`
}

var attrTypesSpendSummary = map[string]attr.Type{
	"user_id":              types.StringType,
	"email":                types.StringType,
	"name":                 types.StringType,
	"period":               types.StringType,
	"amount":               types.StringType,
	"currency":             types.StringType,
	"period_to_date_spend": types.StringType,
	"source_type":          types.StringType,
	"source_id":            types.StringType,
	"spend_limit_id":       types.StringType,
}

func (d *spendLimitsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spend_limits"
}

func (d *spendLimitsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the effective per-user spend limits of a Claude Enterprise organization, " +
			"including limits inherited from seat tiers, groups or the organization. Requires `enterprise_api_key`.",
		Attributes: map[string]schema.Attribute{
			"periods": schema.ListAttribute{
				MarkdownDescription: "Periods to return (`daily`, `weekly`, `monthly`). Defaults to the API default (monthly).",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.List{listvalidator.SizeAtMost(3),
					listvalidator.ValueStringsAre(stringvalidator.OneOf("daily", "weekly", "monthly"))},
			},
			"user_ids": schema.ListAttribute{
				MarkdownDescription: "Restrict to these user ids (at most 100).",
				ElementType:         types.StringType,
				Optional:            true,
				Validators:          []validator.List{listvalidator.SizeAtMost(100)},
			},
			"spend_limits": schema.ListNestedAttribute{
				MarkdownDescription: "One row per user and period.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"user_id":              dsString("User id."),
					"email":                dsString("User email; null when unavailable."),
					"name":                 dsString("User name; null when unavailable."),
					"period":               dsString("`daily`, `weekly` or `monthly`."),
					"amount":               dsString("Effective cap in minor units (cents) as a string; null when unlimited."),
					"currency":             dsString("ISO 4217 currency code."),
					"period_to_date_spend": dsString("Spend so far in the current period, as a decimal string."),
					"source_type":          dsString("Where the limit comes from: `user`, `seat_tier`, `rbac_group`, `organization_service` or `organization`."),
					"source_id":            dsString("Identifier of the source (user id, seat tier, group id or service name); null for `organization`."),
					"spend_limit_id":       dsString("Id of the spend limit row the source resolves to."),
				}},
			},
		},
	}
}

func (d *spendLimitsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *spendLimitsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg spendLimitsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var opts client.SpendLimitListOptions
	if !cfg.Periods.IsNull() {
		resp.Diagnostics.Append(cfg.Periods.ElementsAs(ctx, &opts.Periods, false)...)
	}
	if !cfg.UserIDs.IsNull() {
		resp.Diagnostics.Append(cfg.UserIDs.ElementsAs(ctx, &opts.UserIDs, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	rows, err := d.client.ListEffectiveSpendLimits(ctx, opts)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing effective spend limits", err)
		return
	}
	objs := make([]attr.Value, 0, len(rows))
	for _, row := range rows {
		obj, diags := types.ObjectValue(attrTypesSpendSummary, map[string]attr.Value{
			"user_id":              types.StringValue(row.Actor.UserID),
			"email":                stringFromPtr(row.Actor.EmailAddress),
			"name":                 stringFromPtr(row.Actor.Name),
			"period":               types.StringValue(row.Period),
			"amount":               stringFromPtr(row.Amount),
			"currency":             types.StringValue(row.Currency),
			"period_to_date_spend": types.StringValue(row.PeriodToDateSpend),
			"source_type":          types.StringValue(row.Source.Type),
			"source_id":            spendScopeID(row.Source),
			"spend_limit_id":       types.StringValue(row.SpendLimitID),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesSpendSummary}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.SpendLimits = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

func spendScopeID(s client.SpendScope) types.String {
	switch s.Type {
	case "user":
		return stringFromPtr(s.UserID)
	case "seat_tier":
		return stringFromPtr(s.SeatTier)
	case "rbac_group":
		return stringFromPtr(s.RBACGroupID)
	case "organization_service":
		return stringFromPtr(s.Service)
	}
	return types.StringNull()
}
