package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewRBACRolesDataSource)
	registerDataSource(NewRBACRoleDataSource)
}

var attrTypesRBACRole = map[string]attr.Type{
	"id":         types.StringType,
	"name":       types.StringType,
	"created_at": types.StringType,
	"updated_at": types.StringType,
}

func rbacRoleObject(r *client.RBACRole) (types.Object, diag.Diagnostics) {
	return types.ObjectValue(attrTypesRBACRole, map[string]attr.Value{
		"id":         types.StringValue(r.ID),
		"name":       types.StringValue(r.Name),
		"created_at": types.StringValue(r.CreatedAt),
		"updated_at": types.StringValue(r.UpdatedAt),
	})
}

// --- anthropic_rbac_roles ---------------------------------------------------

var _ datasource.DataSourceWithConfigure = &rbacRolesDataSource{}

// NewRBACRolesDataSource returns the anthropic_rbac_roles data source.
func NewRBACRolesDataSource() datasource.DataSource { return &rbacRolesDataSource{} }

type rbacRolesDataSource struct{ client *client.Client }

type rbacRolesModel struct {
	Roles types.List `tfsdk:"roles"`
}

func (d *rbacRolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbac_roles"
}

func (d *rbacRolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists RBAC roles in a Claude Enterprise organization. Roles are read-only through the API. Requires `enterprise_api_key`.",
		Attributes: map[string]schema.Attribute{
			"roles": schema.ListNestedAttribute{
				MarkdownDescription: "Roles.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":         dsString("Role id (`rbac_role_...`)."),
					"name":       dsString("Role name."),
					"created_at": dsString("Creation timestamp."),
					"updated_at": dsString("Last update timestamp."),
				}},
			},
		},
	}
}

func (d *rbacRolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *rbacRolesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.ListRBACRoles(ctx)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing RBAC roles", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for i := range list {
		obj, diags := rbacRoleObject(&list[i])
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesRBACRole}, objs)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &rbacRolesModel{Roles: l})...)
}

// --- anthropic_rbac_role ----------------------------------------------------

var _ datasource.DataSourceWithConfigure = &rbacRoleDataSource{}

// NewRBACRoleDataSource returns the anthropic_rbac_role data source.
func NewRBACRoleDataSource() datasource.DataSource { return &rbacRoleDataSource{} }

type rbacRoleDataSource struct{ client *client.Client }

type rbacRoleModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	Permissions types.List   `tfsdk:"permissions"`
}

var attrTypesRBACPermission = map[string]attr.Type{
	"action":        types.StringType,
	"resource_type": types.StringType,
	"resource_id":   types.StringType,
}

func (d *rbacRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbac_role"
}

func (d *rbacRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one RBAC role and its permissions in a Claude Enterprise organization. Requires `enterprise_api_key`.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{MarkdownDescription: "Role id (`rbac_role_...`).", Required: true},
			"name":       dsString("Role name."),
			"created_at": dsString("Creation timestamp."),
			"updated_at": dsString("Last update timestamp."),
			"permissions": schema.ListNestedAttribute{
				MarkdownDescription: "Permissions granted by the role.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"action":        dsString("Permitted action (open vocabulary, e.g. `chat`, `use`)."),
					"resource_type": dsString("Resource type: `organization`, `connector`, `connector_tool`, `connector_scope` or `all_connectors`."),
					"resource_id":   dsString("Organization id or connector id when the resource type carries one; null otherwise."),
				}},
			},
		},
	}
}

func (d *rbacRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *rbacRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg rbacRoleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	role, err := d.client.GetRBACRole(ctx, cfg.ID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading RBAC role", err)
		return
	}
	perms, err := d.client.ListRBACRolePermissions(ctx, role.ID)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing RBAC role permissions", err)
		return
	}
	objs := make([]attr.Value, 0, len(perms))
	for _, p := range perms {
		rt, _ := p.Resource["type"].(string)
		rid := types.StringNull()
		for _, k := range []string{"organization_id", "connector_id"} {
			if v, ok := p.Resource[k].(string); ok && v != "" {
				rid = types.StringValue(v)
				break
			}
		}
		obj, diags := types.ObjectValue(attrTypesRBACPermission, map[string]attr.Value{
			"action":        types.StringValue(p.Action),
			"resource_type": types.StringValue(rt),
			"resource_id":   rid,
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesRBACPermission}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Name = types.StringValue(role.Name)
	cfg.CreatedAt = types.StringValue(role.CreatedAt)
	cfg.UpdatedAt = types.StringValue(role.UpdatedAt)
	cfg.Permissions = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
