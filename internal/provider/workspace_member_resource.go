package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewWorkspaceMemberResource) }

var (
	_ resource.Resource                = &workspaceMemberResource{}
	_ resource.ResourceWithConfigure   = &workspaceMemberResource{}
	_ resource.ResourceWithImportState = &workspaceMemberResource{}
)

// NewWorkspaceMemberResource returns the anthropic_workspace_member resource.
func NewWorkspaceMemberResource() resource.Resource { return &workspaceMemberResource{} }

type workspaceMemberResource struct {
	client *client.Client
}

type workspaceMemberModel struct {
	ID            types.String `tfsdk:"id"`
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	UserID        types.String `tfsdk:"user_id"`
	WorkspaceRole types.String `tfsdk:"workspace_role"`
}

var workspaceMemberRoles = []string{"workspace_admin", "workspace_developer", "workspace_restricted_developer", "workspace_user"}

func (r *workspaceMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_member"
}

func (r *workspaceMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Grants an organization member a role in a workspace.\n\n" +
			"Requires `admin_api_key` or `oauth_token`.\n\n" +
			"Organization admins and billing members hold their workspace access implicitly and cannot be managed here. " +
			"`workspace_billing` is inherited from the organization role and cannot be assigned.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite id `workspace_id/user_id`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace id (`wrkspc_...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_id": schema.StringAttribute{
				MarkdownDescription: "User id (`user_...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"workspace_role": schema.StringAttribute{
				MarkdownDescription: "One of `workspace_admin`, `workspace_developer`, `workspace_restricted_developer`, `workspace_user`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(workspaceMemberRoles...)},
			},
		},
	}
}

func (r *workspaceMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAdmin, &resp.Diagnostics)
}

func (r *workspaceMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceMemberModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.AddWorkspaceMember(ctx, plan.WorkspaceID.ValueString(), client.WorkspaceMemberAdd{
		UserID:        plan.UserID.ValueString(),
		WorkspaceRole: plan.WorkspaceRole.ValueString(),
	})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error adding workspace member", err)
		return
	}
	r.waitForRole(ctx, m.WorkspaceID, m.UserID, m.WorkspaceRole)
	flattenWorkspaceMember(m, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workspaceMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceMemberModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.FindWorkspaceMember(ctx, state.WorkspaceID.ValueString(), state.UserID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading workspace member", err)
		return
	}
	flattenWorkspaceMember(m, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workspaceMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspaceMemberModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := retryUntilVisible(ctx, func() (*client.WorkspaceMember, error) {
		return r.client.UpdateWorkspaceMember(ctx, plan.WorkspaceID.ValueString(), plan.UserID.ValueString(),
			client.WorkspaceRoleUpdate{WorkspaceRole: plan.WorkspaceRole.ValueString()})
	})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating workspace member", err)
		return
	}
	r.waitForRole(ctx, m.WorkspaceID, m.UserID, m.WorkspaceRole)
	flattenWorkspaceMember(m, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workspaceMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceMemberModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := retryUntilVisible(ctx, func() (struct{}, error) {
		return struct{}{}, r.client.RemoveWorkspaceMember(ctx, state.WorkspaceID.ValueString(), state.UserID.ValueString())
	})
	if err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error removing workspace member", err)
	}
}

// waitForRole blocks until a read of the membership reflects the role just
// written, so the refresh that follows an apply sees the new value.
func (r *workspaceMemberResource) waitForRole(ctx context.Context, workspaceID, userID, role string) {
	waitUntil(ctx, func() bool {
		m, err := r.client.FindWorkspaceMember(ctx, workspaceID, userID)
		return err == nil && m.WorkspaceRole == role
	})
}

func (r *workspaceMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, "workspace_id/user_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(req.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), types.StringValue(parts[0]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), types.StringValue(parts[1]))...)
}

func flattenWorkspaceMember(m *client.WorkspaceMember, model *workspaceMemberModel) {
	model.ID = types.StringValue(compositeID(m.WorkspaceID, m.UserID))
	model.WorkspaceID = types.StringValue(m.WorkspaceID)
	model.UserID = types.StringValue(m.UserID)
	model.WorkspaceRole = types.StringValue(m.WorkspaceRole)
}
