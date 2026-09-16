package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewInviteResource) }

var (
	_ resource.Resource                = &inviteResource{}
	_ resource.ResourceWithConfigure   = &inviteResource{}
	_ resource.ResourceWithImportState = &inviteResource{}
)

// NewInviteResource returns the anthropic_invite resource.
func NewInviteResource() resource.Resource { return &inviteResource{} }

type inviteResource struct {
	client *client.Client
}

type inviteModel struct {
	ID           types.String `tfsdk:"id"`
	Email        types.String `tfsdk:"email"`
	Role         types.String `tfsdk:"role"`
	RBACGroupIDs types.List   `tfsdk:"rbac_group_ids"`
	Status       types.String `tfsdk:"status"`
	InvitedAt    types.String `tfsdk:"invited_at"`
	ExpiresAt    types.String `tfsdk:"expires_at"`
	AcceptedAt   types.String `tfsdk:"accepted_at"`
}

func (r *inviteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_invite"
}

func (r *inviteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Invites a person to the organization by email. Works with `admin_api_key`/`oauth_token` (Console " +
			"organizations) and `enterprise_api_key` (Claude Enterprise organizations).\n\n" +
			"Invites have no update endpoint, so every change forces a new invite. Once accepted or expired the invite stays " +
			"in state with its final `status`; destroy is a no-op for anything other than a pending invite. To resend an " +
			"expired invite, taint or replace the resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Invite id (`invite_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Email address to invite.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "Organization role: `user`, `developer`, `billing`, `claude_code_user` (Console) or `user`, `managed` (Claude Enterprise).",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(writableOrgRoles...)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"rbac_group_ids": schema.ListAttribute{
				MarkdownDescription: "RBAC groups to add the user to on acceptance. Claude Enterprise only.",
				ElementType:         types.StringType,
				Optional:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"status":      schema.StringAttribute{MarkdownDescription: "`pending`, `accepted`, `expired` or `deleted`.", Computed: true},
			"invited_at":  computedString("Creation timestamp (RFC 3339)."),
			"expires_at":  computedString("Expiry timestamp (RFC 3339)."),
			"accepted_at": schema.StringAttribute{MarkdownDescription: "Acceptance timestamp; null until accepted.", Computed: true},
		},
	}
}

func (r *inviteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = memberClientFromResource(req, &resp.Diagnostics)
}

func (r *inviteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan inviteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.InviteCreate{Email: plan.Email.ValueString(), Role: plan.Role.ValueString()}
	if !plan.RBACGroupIDs.IsNull() && !plan.RBACGroupIDs.IsUnknown() {
		resp.Diagnostics.Append(plan.RBACGroupIDs.ElementsAs(ctx, &in.RBACGroupIDs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	inv, err := r.client.CreateInvite(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating invite", err)
		return
	}
	resp.Diagnostics.Append(flattenInvite(ctx, inv, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *inviteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state inviteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	inv, err := r.client.GetInvite(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading invite", err)
		return
	}
	resp.Diagnostics.Append(flattenInvite(ctx, inv, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *inviteResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Invites cannot be updated", "every attribute forces replacement; this is a provider bug")
}

func (r *inviteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state inviteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.Status.ValueString() != "pending" {
		tflog.Info(ctx, "invite is not pending; nothing to delete", map[string]any{"id": state.ID.ValueString(), "status": state.Status.ValueString()})
		return
	}
	if err := r.client.DeleteInvite(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) && !client.IsBadRequest(err) {
		apiErrorDiag(&resp.Diagnostics, "Error deleting invite", err)
	}
}

func (r *inviteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenInvite(ctx context.Context, inv *client.Invite, m *inviteModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(inv.ID)
	m.Email = types.StringValue(inv.Email)
	m.Role = types.StringValue(inv.Role)
	m.Status = types.StringValue(inv.Status)
	m.InvitedAt = types.StringValue(inv.InvitedAt)
	m.ExpiresAt = types.StringValue(inv.ExpiresAt)
	m.AcceptedAt = stringFromPtr(inv.AcceptedAt)
	if len(inv.RBACGroupIDs) == 0 && (m.RBACGroupIDs.IsNull() || m.RBACGroupIDs.IsUnknown()) {
		m.RBACGroupIDs = types.ListNull(types.StringType)
	} else {
		l, d := types.ListValueFrom(ctx, types.StringType, inv.RBACGroupIDs)
		diags.Append(d...)
		m.RBACGroupIDs = l
	}
	return diags
}
