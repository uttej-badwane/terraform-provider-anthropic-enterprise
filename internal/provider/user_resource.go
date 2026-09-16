package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewUserResource) }

var (
	_ resource.Resource                = &userResource{}
	_ resource.ResourceWithConfigure   = &userResource{}
	_ resource.ResourceWithImportState = &userResource{}
)

// NewUserResource returns the anthropic_user resource.
func NewUserResource() resource.Resource { return &userResource{} }

type userResource struct {
	client *client.Client
}

type userModel struct {
	ID              types.String `tfsdk:"id"`
	Role            types.String `tfsdk:"role"`
	RemoveOnDestroy types.Bool   `tfsdk:"remove_on_destroy"`
	Email           types.String `tfsdk:"email"`
	Name            types.String `tfsdk:"name"`
	AddedAt         types.String `tfsdk:"added_at"`
}

const userCreateError = "Users join an organization through an invite (anthropic_invite) or SSO; they cannot be created " +
	"through the Admin API. Import an existing member: terraform import anthropic_user.<name> user_..."

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the organization role of an existing member. Members join through invites or SSO, so this " +
			"resource is **import-only**: `terraform import anthropic_user.<name> user_...`. Applying a configuration for a " +
			"user that is not in state fails.\n\n" +
			"Works with `admin_api_key`/`oauth_token` (Console organizations) and `enterprise_api_key` (Claude Enterprise).\n\n" +
			"Admins and owners cannot be modified or removed through the API. Destroy only removes the member from the " +
			"organization when `remove_on_destroy = true`; by default it just drops the resource from state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "User id (`user_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "Organization role: `user`, `developer`, `billing`, `claude_code_user` (Console) or `user`, `managed` (Claude Enterprise).",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(writableOrgRoles...)},
			},
			"remove_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Remove the member from the organization when the resource is destroyed. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"email":    computedString("Email address."),
			"name":     computedString("Display name."),
			"added_at": computedString("Timestamp the member joined (RFC 3339)."),
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = memberClientFromResource(req, &resp.Diagnostics)
}

func (r *userResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError("Users cannot be created", userCreateError)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	u, err := r.client.GetUser(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading user", err)
		return
	}
	flattenUser(u, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var u *client.User
	var err error
	if plan.Role.Equal(state.Role) {
		u, err = r.client.GetUser(ctx, state.ID.ValueString())
	} else {
		u, err = r.client.UpdateUser(ctx, state.ID.ValueString(), client.UserUpdate{Role: plan.Role.ValueString()})
	}
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating user", err)
		return
	}
	newState := plan
	flattenUser(u, &newState)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.RemoveOnDestroy.ValueBool() {
		tflog.Info(ctx, "remove_on_destroy is false; leaving member in place", map[string]any{"id": state.ID.ValueString()})
		return
	}
	if err := r.client.RemoveUser(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error removing user", err)
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("remove_on_destroy"), types.BoolValue(false))...)
}

func flattenUser(u *client.User, m *userModel) {
	m.ID = types.StringValue(u.ID)
	m.Role = types.StringValue(u.Role)
	m.Email = types.StringValue(u.Email)
	m.Name = types.StringValue(u.Name)
	m.AddedAt = types.StringValue(u.AddedAt)
	if m.RemoveOnDestroy.IsNull() || m.RemoveOnDestroy.IsUnknown() {
		m.RemoveOnDestroy = types.BoolValue(false)
	}
}

var attrTypesUser = map[string]attr.Type{
	"id":       types.StringType,
	"email":    types.StringType,
	"name":     types.StringType,
	"role":     types.StringType,
	"added_at": types.StringType,
}

func userObject(u *client.User) (types.Object, diag.Diagnostics) {
	return types.ObjectValue(attrTypesUser, map[string]attr.Value{
		"id":       types.StringValue(u.ID),
		"email":    types.StringValue(u.Email),
		"name":     types.StringValue(u.Name),
		"role":     types.StringValue(u.Role),
		"added_at": types.StringValue(u.AddedAt),
	})
}
