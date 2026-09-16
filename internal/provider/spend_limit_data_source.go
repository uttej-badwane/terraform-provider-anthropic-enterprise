package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewSpendLimitDataSource) }

var _ datasource.DataSourceWithConfigure = &spendLimitDataSource{}

// NewSpendLimitDataSource returns the anthropic_spend_limit data source.
func NewSpendLimitDataSource() datasource.DataSource { return &spendLimitDataSource{} }

type spendLimitDataSource struct{ client *client.Client }

type spendLimitDSModel struct {
	ID        types.String `tfsdk:"id"`
	UserID    types.String `tfsdk:"user_id"`
	Period    types.String `tfsdk:"period"`
	Amount    types.String `tfsdk:"amount"`
	Currency  types.String `tfsdk:"currency"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

func (d *spendLimitDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spend_limit"
}

func (d *spendLimitDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one spend limit row in a Claude Enterprise organization by id. Requires `enterprise_api_key`.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{MarkdownDescription: "Spend limit id (`spl_...`).", Required: true},
			"user_id":    dsString("User the limit applies to; null for non-user scopes."),
			"period":     dsString("`daily`, `weekly` or `monthly`."),
			"amount":     dsString("Cap in minor units (cents) as a string; null when unlimited."),
			"currency":   dsString("ISO 4217 currency code."),
			"created_at": dsString("Creation timestamp."),
			"updated_at": dsString("Last update timestamp."),
		},
	}
}

func (d *spendLimitDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *spendLimitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg spendLimitDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sl, err := d.client.GetSpendLimit(ctx, cfg.ID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading spend limit", err)
		return
	}
	cfg.UserID = stringFromPtr(sl.Scope.UserID)
	cfg.Period = types.StringValue(sl.Period)
	cfg.Amount = stringFromPtr(sl.Amount)
	cfg.Currency = types.StringValue(sl.Currency)
	cfg.CreatedAt = types.StringValue(sl.CreatedAt)
	cfg.UpdatedAt = types.StringValue(sl.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
