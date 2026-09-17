package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
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

func init() { registerResource(NewWorkspaceServiceAccountResource) }

var (
	_ resource.Resource                = &workspaceServiceAccountResource{}
	_ resource.ResourceWithConfigure   = &workspaceServiceAccountResource{}
	_ resource.ResourceWithImportState = &workspaceServiceAccountResource{}
)

// assignableWorkspaceRoles are the roles accepted when adding a principal to a workspace.
var assignableWorkspaceRoles = []string{"workspace_admin", "workspace_developer", "workspace_restricted_developer", "workspace_user"}

// NewWorkspaceServiceAccountResource returns the anthropic_workspace_service_account resource.
func NewWorkspaceServiceAccountResource() resource.Resource {
	return &workspaceServiceAccountResource{}
}

type workspaceServiceAccountResource struct {
	client *client.Client
}

type workspaceServiceAccountModel struct {
	ID               types.String   `tfsdk:"id"`
	WorkspaceID      types.String   `tfsdk:"workspace_id"`
	ServiceAccountID types.String   `tfsdk:"service_account_id"`
	WorkspaceRole    types.String   `tfsdk:"workspace_role"`
	Implicit         types.Bool     `tfsdk:"implicit"`
	CreatedByActorID types.String   `tfsdk:"created_by_actor_id"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *workspaceServiceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_service_account"
}

func (r *workspaceServiceAccountResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Grants a service account a role in a workspace.\n\n" +
			"Requires `oauth_token` (an `org:admin` OAuth token). Import with `workspace_id/service_account_id`.",
		Blocks: map[string]schema.Block{
			// The Admin API is eventually consistent here, so update and delete
			// wait for the change to become visible. These bound that wait.
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite id `workspace_id/service_account_id`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace id (`wrkspc_...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"service_account_id": schema.StringAttribute{
				MarkdownDescription: "Service account id (`svac_...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"workspace_role": schema.StringAttribute{
				MarkdownDescription: "Role in the workspace: `workspace_admin`, `workspace_developer`, `workspace_restricted_developer` or `workspace_user`. Service accounts cannot hold `workspace_billing`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(assignableWorkspaceRoles...)},
			},
			"implicit": schema.BoolAttribute{
				MarkdownDescription: "True for the implicit default-workspace membership every service account has.",
				Computed:            true,
			},
			"created_by_actor_id": schema.StringAttribute{
				MarkdownDescription: "Actor that created the membership (`user_...` or `svac_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *workspaceServiceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredOAuth, &resp.Diagnostics)
}

func (r *workspaceServiceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceServiceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wait, tdiags := plan.Timeouts.Create(ctx, consistencyTimeout)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.AddWorkspaceServiceAccount(ctx, plan.WorkspaceID.ValueString(), client.ServiceAccountWorkspaceMemberAdd{
		ServiceAccountID: plan.ServiceAccountID.ValueString(),
		WorkspaceRole:    plan.WorkspaceRole.ValueString(),
	})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error adding service account to workspace", err)
		return
	}
	r.waitForRole(ctx, wait, m.WorkspaceID, m.ServiceAccountID, m.WorkspaceRole)
	state := plan
	flattenWorkspaceServiceAccount(m, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workspaceServiceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceServiceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.FindWorkspaceServiceAccount(ctx, state.WorkspaceID.ValueString(), state.ServiceAccountID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading workspace service account", err)
		return
	}
	flattenWorkspaceServiceAccount(m, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workspaceServiceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state workspaceServiceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wait, tdiags := plan.Timeouts.Update(ctx, consistencyTimeout)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := retryUntilVisibleFor(ctx, wait, consistencyInterval, func() (*client.ServiceAccountWorkspaceMember, error) {
		return r.client.UpdateWorkspaceServiceAccount(ctx, state.WorkspaceID.ValueString(), state.ServiceAccountID.ValueString(),
			client.WorkspaceRoleUpdate{WorkspaceRole: plan.WorkspaceRole.ValueString()})
	})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating workspace service account", err)
		return
	}
	r.waitForRole(ctx, wait, m.WorkspaceID, m.ServiceAccountID, m.WorkspaceRole)
	newState := plan
	flattenWorkspaceServiceAccount(m, &newState)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *workspaceServiceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceServiceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wait, tdiags := state.Timeouts.Delete(ctx, consistencyTimeout)
	resp.Diagnostics.Append(tdiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := retryUntilVisibleFor(ctx, wait, consistencyInterval, func() (struct{}, error) {
		return struct{}{}, r.client.RemoveWorkspaceServiceAccount(ctx, state.WorkspaceID.ValueString(), state.ServiceAccountID.ValueString())
	})
	if err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error removing service account from workspace", err)
	}
}

func (r *workspaceServiceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, "workspace_id/service_account_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(req.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), types.StringValue(parts[0]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_account_id"), types.StringValue(parts[1]))...)
}

func flattenWorkspaceServiceAccount(m *client.ServiceAccountWorkspaceMember, s *workspaceServiceAccountModel) {
	s.ID = types.StringValue(compositeID(m.WorkspaceID, m.ServiceAccountID))
	s.WorkspaceID = types.StringValue(m.WorkspaceID)
	s.ServiceAccountID = types.StringValue(m.ServiceAccountID)
	s.WorkspaceRole = types.StringValue(m.WorkspaceRole)
	s.Implicit = types.BoolValue(m.Implicit != nil && *m.Implicit)
	s.CreatedByActorID = stringFromPtr(m.CreatedByActorID)
}

// waitForRole blocks until a read of the membership reflects the role just
// written, so the refresh that follows an apply sees the new value.
func (r *workspaceServiceAccountResource) waitForRole(ctx context.Context, timeout time.Duration, workspaceID, serviceAccountID, role string) {
	waitUntilFor(ctx, timeout, consistencyInterval, func() bool {
		m, err := r.client.FindWorkspaceServiceAccount(ctx, workspaceID, serviceAccountID)
		return err == nil && m.WorkspaceRole == role
	})
}
