package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewRBACGroupResource) }

var (
	_ resource.Resource                = &rbacGroupResource{}
	_ resource.ResourceWithConfigure   = &rbacGroupResource{}
	_ resource.ResourceWithImportState = &rbacGroupResource{}
)

// NewRBACGroupResource returns the anthropic_rbac_group resource.
func NewRBACGroupResource() resource.Resource { return &rbacGroupResource{} }

type rbacGroupResource struct {
	client *client.Client
}

type rbacGroupModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	SourceType types.String `tfsdk:"source_type"`
	Roles      types.List   `tfsdk:"roles"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

func (r *rbacGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbac_group"
}

func (r *rbacGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an RBAC group in a Claude Enterprise organization. Requires `enterprise_api_key`.\n\n" +
			"Groups provisioned through SCIM (`source_type = \"scim\"`) are read-only: they can be imported and read, " +
			"but renaming, deleting or changing their membership through the API fails.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Group id (`rbac_group_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Group name, 1 to 255 characters. Uniqueness is not enforced by the API.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 255)},
			},
			"source_type": schema.StringAttribute{
				MarkdownDescription: "How the group is managed: `direct` (API/console) or `scim`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"roles": schema.ListAttribute{
				MarkdownDescription: "RBAC role ids assigned to the group (read-only). Null when the API reports the list as temporarily unavailable.",
				ElementType:         types.StringType,
				Computed:            true,
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

func (r *rbacGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (r *rbacGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan rbacGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.CreateRBACGroup(ctx, client.RBACGroupWrite{Name: plan.Name.ValueString()})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating RBAC group", err)
		return
	}
	state := plan
	resp.Diagnostics.Append(flattenRBACGroup(ctx, g, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *rbacGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state rbacGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.GetRBACGroup(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading RBAC group", err)
		return
	}
	resp.Diagnostics.Append(flattenRBACGroup(ctx, g, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *rbacGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state rbacGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.SourceType.ValueString() == "scim" {
		resp.Diagnostics.AddError("SCIM-managed group is read-only",
			"This group is provisioned through SCIM and cannot be modified via the Admin API. Change it in your identity provider instead.")
		return
	}
	g, err := r.client.UpdateRBACGroup(ctx, state.ID.ValueString(), client.RBACGroupWrite{Name: plan.Name.ValueString()})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating RBAC group", err)
		return
	}
	newState := plan
	resp.Diagnostics.Append(flattenRBACGroup(ctx, g, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *rbacGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state rbacGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteRBACGroup(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		if state.SourceType.ValueString() == "scim" {
			resp.Diagnostics.AddError("SCIM-managed group cannot be deleted via the API",
				"Remove the group in your identity provider, or use `terraform state rm` to stop managing it. API error: "+err.Error())
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error deleting RBAC group", err)
	}
}

func (r *rbacGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenRBACGroup(ctx context.Context, g *client.RBACGroup, m *rbacGroupModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(g.ID)
	m.Name = types.StringValue(g.Name)
	m.SourceType = types.StringValue(g.SourceType)
	m.CreatedAt = types.StringValue(g.CreatedAt)
	m.UpdatedAt = types.StringValue(g.UpdatedAt)
	roles, d := stringList(ctx, g.Roles)
	diags.Append(d...)
	m.Roles = roles
	return diags
}

var attrTypesRBACGroup = map[string]attr.Type{
	"id":          types.StringType,
	"name":        types.StringType,
	"source_type": types.StringType,
	"roles":       types.ListType{ElemType: types.StringType},
	"created_at":  types.StringType,
	"updated_at":  types.StringType,
}

func rbacGroupObject(ctx context.Context, g *client.RBACGroup) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	roles, d := stringList(ctx, g.Roles)
	diags.Append(d...)
	obj, d := types.ObjectValue(attrTypesRBACGroup, map[string]attr.Value{
		"id":          types.StringValue(g.ID),
		"name":        types.StringValue(g.Name),
		"source_type": types.StringValue(g.SourceType),
		"roles":       roles,
		"created_at":  types.StringValue(g.CreatedAt),
		"updated_at":  types.StringValue(g.UpdatedAt),
	})
	diags.Append(d...)
	return obj, diags
}
