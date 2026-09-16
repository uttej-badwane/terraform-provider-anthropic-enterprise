package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewComplianceOrganizationsDataSource)
	registerDataSource(NewComplianceOrganizationUsersDataSource)
	registerDataSource(NewComplianceRolesDataSource)
	registerDataSource(NewComplianceRoleDataSource)
	registerDataSource(NewComplianceGroupsDataSource)
	registerDataSource(NewComplianceGroupMembersDataSource)
}

const complianceNote = " Requires `compliance_api_key` (a Compliance Access Key) or an `enterprise_api_key` carrying the " +
	"`read:compliance_org_data` scope; Admin API keys are rejected."

// complianceBase holds the shared Configure for compliance data sources.
type complianceBase struct{ client *client.Client }

func (b *complianceBase) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	b.client = clientFromDataSource(req, client.CredCompliance, &resp.Diagnostics)
}

// --- anthropic_compliance_organizations -------------------------------------

var _ datasource.DataSourceWithConfigure = &complianceOrganizationsDataSource{}

// NewComplianceOrganizationsDataSource returns the anthropic_compliance_organizations data source.
func NewComplianceOrganizationsDataSource() datasource.DataSource {
	return &complianceOrganizationsDataSource{}
}

type complianceOrganizationsDataSource struct{ complianceBase }

var attrTypesComplianceOrg = map[string]attr.Type{
	"uuid": types.StringType, "name": types.StringType, "created_at": types.StringType,
}

func (d *complianceOrganizationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_organizations"
}

func (d *complianceOrganizationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every organization linked under the Claude Enterprise parent the key is bound to." + complianceNote,
		Attributes: map[string]schema.Attribute{
			"organizations": schema.ListNestedAttribute{
				MarkdownDescription: "Linked organizations, oldest first.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"uuid":       dsString("Organization UUID; the path parameter for the per-organization data sources."),
					"name":       dsString("Organization name."),
					"created_at": dsString("Creation timestamp."),
				}},
			},
		},
	}
}

func (d *complianceOrganizationsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	orgs, err := d.client.ListComplianceOrganizations(ctx)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing compliance organizations", err)
		return
	}
	vals := make([]map[string]attr.Value, 0, len(orgs))
	for _, o := range orgs {
		vals = append(vals, map[string]attr.Value{
			"uuid": types.StringValue(o.UUID), "name": types.StringValue(o.Name), "created_at": types.StringValue(o.CreatedAt),
		})
	}
	l := objectList(&resp.Diagnostics, attrTypesComplianceOrg, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &struct {
		Organizations types.List `tfsdk:"organizations"`
	}{l})...)
}

// --- anthropic_compliance_organization_users --------------------------------

var _ datasource.DataSourceWithConfigure = &complianceUsersDataSource{}

// NewComplianceOrganizationUsersDataSource returns the anthropic_compliance_organization_users data source.
func NewComplianceOrganizationUsersDataSource() datasource.DataSource {
	return &complianceUsersDataSource{}
}

type complianceUsersDataSource struct{ complianceBase }

type complianceUsersModel struct {
	OrganizationUUID types.String `tfsdk:"organization_uuid"`
	Users            types.List   `tfsdk:"users"`
}

var attrTypesComplianceUser = map[string]attr.Type{
	"id": types.StringType, "full_name": types.StringType, "email": types.StringType,
	"organization_role": types.StringType, "created_at": types.StringType,
}

func (d *complianceUsersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_organization_users"
}

func (d *complianceUsersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the active members of one linked organization from the compliance directory. " +
			"Removed users disappear immediately. Requires the `read:compliance_user_data` scope." + complianceNote,
		Attributes: map[string]schema.Attribute{
			"organization_uuid": schema.StringAttribute{MarkdownDescription: "Organization UUID (see `anthropic_compliance_organizations`).", Required: true},
			"users": schema.ListNestedAttribute{
				MarkdownDescription: "Members sorted by join date.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":                dsString("User id (`user_...`)."),
					"full_name":         dsString("Full name."),
					"email":             dsString("Email address."),
					"organization_role": dsString("Built-in membership role (`user`, `admin`, `owner`, ...), independent of RBAC roles."),
					"created_at":        dsString("Join timestamp."),
				}},
			},
		},
	}
}

func (d *complianceUsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg complianceUsersModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	users, err := d.client.ListComplianceUsers(ctx, cfg.OrganizationUUID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing compliance organization users", err)
		return
	}
	vals := make([]map[string]attr.Value, 0, len(users))
	for _, u := range users {
		vals = append(vals, map[string]attr.Value{
			"id": types.StringValue(u.ID), "full_name": types.StringValue(u.FullName), "email": types.StringValue(u.Email),
			"organization_role": types.StringValue(u.OrganizationRole), "created_at": types.StringValue(u.CreatedAt),
		})
	}
	cfg.Users = objectList(&resp.Diagnostics, attrTypesComplianceUser, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_compliance_roles / anthropic_compliance_role -----------------

var attrTypesComplianceRole = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "description": types.StringType,
	"created_at": types.StringType, "updated_at": types.StringType,
}

func complianceRoleValues(r *client.ComplianceRole) map[string]attr.Value {
	return map[string]attr.Value{
		"id": types.StringValue(r.ID), "name": types.StringValue(r.Name), "description": stringFromPtr(r.Description),
		"created_at": types.StringValue(r.CreatedAt), "updated_at": types.StringValue(r.UpdatedAt),
	}
}

var _ datasource.DataSourceWithConfigure = &complianceRolesDataSource{}

// NewComplianceRolesDataSource returns the anthropic_compliance_roles data source.
func NewComplianceRolesDataSource() datasource.DataSource { return &complianceRolesDataSource{} }

type complianceRolesDataSource struct{ complianceBase }

type complianceRolesModel struct {
	OrganizationUUID types.String `tfsdk:"organization_uuid"`
	Roles            types.List   `tfsdk:"roles"`
}

func (d *complianceRolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_roles"
}

func (d *complianceRolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the custom RBAC roles defined on one linked organization." + complianceNote,
		Attributes: map[string]schema.Attribute{
			"organization_uuid": schema.StringAttribute{MarkdownDescription: "Organization UUID.", Required: true},
			"roles": schema.ListNestedAttribute{
				MarkdownDescription: "Roles.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":          dsString("Role id (`rbac_role_...`)."),
					"name":        dsString("Role name."),
					"description": dsString("Role description; may be null."),
					"created_at":  dsString("Creation timestamp."),
					"updated_at":  dsString("Last update timestamp."),
				}},
			},
		},
	}
}

func (d *complianceRolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg complianceRolesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	roles, err := d.client.ListComplianceRoles(ctx, cfg.OrganizationUUID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing compliance roles", err)
		return
	}
	vals := make([]map[string]attr.Value, 0, len(roles))
	for i := range roles {
		vals = append(vals, complianceRoleValues(&roles[i]))
	}
	cfg.Roles = objectList(&resp.Diagnostics, attrTypesComplianceRole, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

var _ datasource.DataSourceWithConfigure = &complianceRoleDataSource{}

// NewComplianceRoleDataSource returns the anthropic_compliance_role data source.
func NewComplianceRoleDataSource() datasource.DataSource { return &complianceRoleDataSource{} }

type complianceRoleDataSource struct{ complianceBase }

type complianceRoleModel struct {
	OrganizationUUID types.String `tfsdk:"organization_uuid"`
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
	Permissions      types.List   `tfsdk:"permissions"`
}

func (d *complianceRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_role"
}

func (d *complianceRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one custom RBAC role of a linked organization together with its permissions." + complianceNote,
		Attributes: map[string]schema.Attribute{
			"organization_uuid": schema.StringAttribute{MarkdownDescription: "Organization UUID.", Required: true},
			"id":                schema.StringAttribute{MarkdownDescription: "Role id (`rbac_role_...`).", Required: true},
			"name":              dsString("Role name."),
			"description":       dsString("Role description; may be null."),
			"created_at":        dsString("Creation timestamp."),
			"updated_at":        dsString("Last update timestamp."),
			"permissions": schema.ListNestedAttribute{
				MarkdownDescription: "Permissions granted by the role.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"action":        dsString("Permitted action (open vocabulary)."),
					"resource_type": dsString("Resource type (`organization`, `connector`, `connector_tool`, `connector_scope`, `all_connectors`)."),
					"resource_id":   dsString("Organization id or connector id when present; null otherwise."),
				}},
			},
		},
	}
}

func permissionValues(perms []client.RBACRolePermission) []map[string]attr.Value {
	vals := make([]map[string]attr.Value, 0, len(perms))
	for _, p := range perms {
		rt, _ := p.Resource["type"].(string)
		rid := types.StringNull()
		for _, k := range []string{"organization_id", "connector_id"} {
			if v, ok := p.Resource[k].(string); ok && v != "" {
				rid = types.StringValue(v)
				break
			}
		}
		vals = append(vals, map[string]attr.Value{
			"action": types.StringValue(p.Action), "resource_type": types.StringValue(rt), "resource_id": rid,
		})
	}
	return vals
}

func (d *complianceRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg complianceRoleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	org, id := cfg.OrganizationUUID.ValueString(), cfg.ID.ValueString()
	role, err := d.client.GetComplianceRole(ctx, org, id)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading compliance role", err)
		return
	}
	perms, err := d.client.ListComplianceRolePermissions(ctx, org, id)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing compliance role permissions", err)
		return
	}
	cfg.Name = types.StringValue(role.Name)
	cfg.Description = stringFromPtr(role.Description)
	cfg.CreatedAt = types.StringValue(role.CreatedAt)
	cfg.UpdatedAt = types.StringValue(role.UpdatedAt)
	cfg.Permissions = objectList(&resp.Diagnostics, attrTypesRBACPermission, permissionValues(perms))
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_compliance_groups / anthropic_compliance_group_members -------

var _ datasource.DataSourceWithConfigure = &complianceGroupsDataSource{}

// NewComplianceGroupsDataSource returns the anthropic_compliance_groups data source.
func NewComplianceGroupsDataSource() datasource.DataSource { return &complianceGroupsDataSource{} }

type complianceGroupsDataSource struct{ complianceBase }

var attrTypesComplianceGroup = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "description": types.StringType, "source_type": types.StringType,
	"roles": types.ListType{ElemType: types.StringType}, "created_at": types.StringType, "updated_at": types.StringType,
}

func (d *complianceGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_groups"
}

func (d *complianceGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists RBAC and SCIM-provisioned groups across the parent tree, including the role ids attached to " +
			"each group (which the Admin API `anthropic_rbac_groups` data source does not expose)." + complianceNote,
		Attributes: map[string]schema.Attribute{
			"groups": schema.ListNestedAttribute{
				MarkdownDescription: "Groups.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":          dsString("Group id (`rbac_group_...`)."),
					"name":        dsString("Group name."),
					"description": dsString("Group description; may be null."),
					"source_type": dsString("`direct` (created in claude.ai) or `scim`."),
					"roles":       dsStringList("Role ids assigned to the group."),
					"created_at":  dsString("Creation timestamp."),
					"updated_at":  dsString("Last update timestamp."),
				}},
			},
		},
	}
}

func (d *complianceGroupsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	groups, err := d.client.ListComplianceGroups(ctx)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing compliance groups", err)
		return
	}
	vals := make([]map[string]attr.Value, 0, len(groups))
	for _, g := range groups {
		roles, diags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(g.Roles))
		resp.Diagnostics.Append(diags...)
		vals = append(vals, map[string]attr.Value{
			"id": types.StringValue(g.ID), "name": types.StringValue(g.Name), "description": stringFromPtr(g.Description),
			"source_type": types.StringValue(g.SourceType), "roles": roles,
			"created_at": types.StringValue(g.CreatedAt), "updated_at": types.StringValue(g.UpdatedAt),
		})
	}
	l := objectList(&resp.Diagnostics, attrTypesComplianceGroup, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &struct {
		Groups types.List `tfsdk:"groups"`
	}{l})...)
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

var _ datasource.DataSourceWithConfigure = &complianceGroupMembersDataSource{}

// NewComplianceGroupMembersDataSource returns the anthropic_compliance_group_members data source.
func NewComplianceGroupMembersDataSource() datasource.DataSource {
	return &complianceGroupMembersDataSource{}
}

type complianceGroupMembersDataSource struct{ complianceBase }

type complianceGroupMembersModel struct {
	GroupID types.String `tfsdk:"group_id"`
	Members types.List   `tfsdk:"members"`
}

var attrTypesComplianceGroupMember = map[string]attr.Type{
	"user_id": types.StringType, "email": types.StringType, "created_at": types.StringType, "updated_at": types.StringType,
}

func (d *complianceGroupMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_group_members"
}

func (d *complianceGroupMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the members of one group from the compliance directory. Requires the " +
			"`read:compliance_user_data` scope." + complianceNote,
		Attributes: map[string]schema.Attribute{
			"group_id": schema.StringAttribute{MarkdownDescription: "Group id (`rbac_group_...`).", Required: true},
			"members": schema.ListNestedAttribute{
				MarkdownDescription: "Members.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"user_id":    dsString("User id."),
					"email":      dsString("Email address."),
					"created_at": dsString("Membership creation timestamp."),
					"updated_at": dsString("Membership update timestamp."),
				}},
			},
		},
	}
}

func (d *complianceGroupMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg complianceGroupMembersModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	members, err := d.client.ListComplianceGroupMembers(ctx, cfg.GroupID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing compliance group members", err)
		return
	}
	vals := make([]map[string]attr.Value, 0, len(members))
	for _, m := range members {
		vals = append(vals, map[string]attr.Value{
			"user_id": types.StringValue(m.UserID), "email": types.StringValue(m.Email),
			"created_at": types.StringValue(m.CreatedAt), "updated_at": types.StringValue(m.UpdatedAt),
		})
	}
	cfg.Members = objectList(&resp.Diagnostics, attrTypesComplianceGroupMember, vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
