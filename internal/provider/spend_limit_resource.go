package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewSpendLimitResource) }

var (
	_ resource.Resource                = &spendLimitResource{}
	_ resource.ResourceWithConfigure   = &spendLimitResource{}
	_ resource.ResourceWithImportState = &spendLimitResource{}
)

// NewSpendLimitResource returns the anthropic_spend_limit resource.
func NewSpendLimitResource() resource.Resource { return &spendLimitResource{} }

type spendLimitResource struct {
	client *client.Client
}

type spendLimitModel struct {
	ID        types.String `tfsdk:"id"`
	UserID    types.String `tfsdk:"user_id"`
	Period    types.String `tfsdk:"period"`
	Amount    types.String `tfsdk:"amount"`
	Currency  types.String `tfsdk:"currency"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
}

var amountRe = regexp.MustCompile(`^[0-9]+$`)

func (r *spendLimitResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_spend_limit"
}

func (r *spendLimitResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a per-user spend limit override in a Claude Enterprise organization. " +
			"Requires `enterprise_api_key` with the spend limit scopes.\n\n" +
			"The API upserts on `(user_id, period)`: creating a second resource for the same user and period " +
			"overwrites the first. Only user-scoped limits are writable; seat-tier, group and organization limits are read-only.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Spend limit id (`spl_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "User the limit applies to (`user_...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"period": schema.StringAttribute{
				MarkdownDescription: "Budget period: `daily`, `weekly` or `monthly`. Defaults to `monthly`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("monthly"),
				Validators:          []validator.String{stringvalidator.OneOf("daily", "weekly", "monthly")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"amount": schema.StringAttribute{
				MarkdownDescription: "Cap in minor currency units (cents) as a decimal string, e.g. `\"50000\"` is 500.00. " +
					"Omit for an explicit no-cap override for this period.",
				Optional:   true,
				Validators: []validator.String{stringvalidator.RegexMatches(amountRe, "must be a non-negative integer string in cents")},
			},
			"currency": schema.StringAttribute{
				MarkdownDescription: "ISO 4217 currency code.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp (RFC 3339).",
				Computed:            true,
			},
		},
	}
}

func (r *spendLimitResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (r *spendLimitResource) upsert(ctx context.Context, plan *spendLimitModel, resp *resource.CreateResponse, uresp *resource.UpdateResponse) {
	in := client.SpendLimitCreate{
		Amount: stringPtr(plan.Amount),
		Scope:  client.SpendScope{Type: "user", UserID: stringPtr(plan.UserID)},
		Period: stringPtr(plan.Period),
	}
	sl, err := r.client.UpsertSpendLimit(ctx, in)
	if err != nil {
		if resp != nil {
			apiErrorDiag(&resp.Diagnostics, "Error creating spend limit", err)
		} else {
			apiErrorDiag(&uresp.Diagnostics, "Error updating spend limit", err)
		}
		return
	}
	flattenSpendLimit(sl, plan)
	if resp != nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	} else {
		uresp.Diagnostics.Append(uresp.State.Set(ctx, plan)...)
	}
}

func (r *spendLimitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan spendLimitModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, &plan, resp, nil)
}

func (r *spendLimitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state spendLimitModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sl, err := r.client.GetSpendLimit(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading spend limit", err)
		return
	}
	flattenSpendLimit(sl, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *spendLimitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan spendLimitModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.upsert(ctx, &plan, nil, resp)
}

func (r *spendLimitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state spendLimitModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSpendLimit(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error deleting spend limit", err)
	}
}

func (r *spendLimitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenSpendLimit(sl *client.SpendLimit, m *spendLimitModel) {
	m.ID = types.StringValue(sl.ID)
	m.Period = types.StringValue(sl.Period)
	m.Amount = stringFromPtr(sl.Amount)
	m.Currency = types.StringValue(sl.Currency)
	m.CreatedAt = types.StringValue(sl.CreatedAt)
	m.UpdatedAt = types.StringValue(sl.UpdatedAt)
	if sl.Scope.UserID != nil {
		m.UserID = types.StringValue(*sl.Scope.UserID)
	}
}
