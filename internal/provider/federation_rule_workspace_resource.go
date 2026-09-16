package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewFederationRuleWorkspaceResource) }

var (
	_ resource.Resource                = &federationRuleWorkspaceResource{}
	_ resource.ResourceWithConfigure   = &federationRuleWorkspaceResource{}
	_ resource.ResourceWithImportState = &federationRuleWorkspaceResource{}
)

// NewFederationRuleWorkspaceResource returns the anthropic_federation_rule_workspace resource.
func NewFederationRuleWorkspaceResource() resource.Resource {
	return &federationRuleWorkspaceResource{}
}

type federationRuleWorkspaceResource struct {
	client *client.Client
}

type federationRuleWorkspaceModel struct {
	ID               types.String `tfsdk:"id"`
	FederationRuleID types.String `tfsdk:"federation_rule_id"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
	CreatedAt        types.String `tfsdk:"created_at"`
	WorkspaceName    types.String `tfsdk:"workspace_name"`
}

func (r *federationRuleWorkspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rule_workspace"
}

func (r *federationRuleWorkspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Enables a federation rule for an additional workspace. The rule's primary workspace is set " +
			"with `workspace_id` on `anthropic_federation_rule`; use this resource for every further workspace.\n\n" +
			"Requires `oauth_token` (an `org:admin` OAuth token). Import with `federation_rule_id/workspace_id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite id `federation_rule_id/workspace_id`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"federation_rule_id": schema.StringAttribute{
				MarkdownDescription: "Federation rule id (`fdrl_...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace id (`wrkspc_...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Binding creation timestamp (RFC 3339).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_name": schema.StringAttribute{
				MarkdownDescription: "Name of the workspace.",
				Computed:            true,
			},
		},
	}
}

func (r *federationRuleWorkspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredOAuth, &resp.Diagnostics)
}

func (r *federationRuleWorkspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan federationRuleWorkspaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.AddFederationRuleWorkspace(ctx, plan.FederationRuleID.ValueString(), plan.WorkspaceID.ValueString()); err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error enabling federation rule for workspace", err)
		return
	}
	// The add response has workspace_name null; read the binding back from the list.
	b, err := r.findBinding(ctx, plan.FederationRuleID.ValueString(), plan.WorkspaceID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading federation rule workspace", err)
		return
	}
	if b == nil {
		resp.Diagnostics.AddError("Binding not found after create", "the API did not return the new binding")
		return
	}
	state := plan
	flattenRuleWorkspace(b, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *federationRuleWorkspaceResource) findBinding(ctx context.Context, ruleID, workspaceID string) (*client.FederationRuleWorkspace, error) {
	list, err := r.client.ListFederationRuleWorkspaces(ctx, ruleID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].WorkspaceID == workspaceID {
			return &list[i], nil
		}
	}
	return nil, nil
}

func (r *federationRuleWorkspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state federationRuleWorkspaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.findBinding(ctx, state.FederationRuleID.ValueString(), state.WorkspaceID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading federation rule workspace", err)
		return
	}
	if b == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	flattenRuleWorkspace(b, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *federationRuleWorkspaceResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected update", "anthropic_federation_rule_workspace has no updatable attributes")
}

func (r *federationRuleWorkspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state federationRuleWorkspaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.RemoveFederationRuleWorkspace(ctx, state.FederationRuleID.ValueString(), state.WorkspaceID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error disabling federation rule for workspace", err)
	}
}

func (r *federationRuleWorkspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, "federation_rule_id/workspace_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(req.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("federation_rule_id"), types.StringValue(parts[0]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), types.StringValue(parts[1]))...)
}

func flattenRuleWorkspace(b *client.FederationRuleWorkspace, m *federationRuleWorkspaceModel) {
	m.ID = types.StringValue(compositeID(b.FederationRuleID, b.WorkspaceID))
	m.FederationRuleID = types.StringValue(b.FederationRuleID)
	m.WorkspaceID = types.StringValue(b.WorkspaceID)
	m.CreatedAt = types.StringValue(b.CreatedAt)
	m.WorkspaceName = stringFromPtr(b.WorkspaceName)
}
