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
	registerDataSource(NewWorkspaceMembersDataSource)
	registerDataSource(NewWorkspaceMemberDataSource)
}

var attrTypesWorkspaceMember = map[string]attr.Type{
	"user_id":        types.StringType,
	"workspace_role": types.StringType,
}

// --- anthropic_workspace_members -------------------------------------------

var _ datasource.DataSourceWithConfigure = &workspaceMembersDataSource{}

// NewWorkspaceMembersDataSource returns the anthropic_workspace_members data source.
func NewWorkspaceMembersDataSource() datasource.DataSource { return &workspaceMembersDataSource{} }

type workspaceMembersDataSource struct{ client *client.Client }

type workspaceMembersModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Members     types.List   `tfsdk:"members"`
}

func (d *workspaceMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_members"
}

func (d *workspaceMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the members of a workspace. Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{MarkdownDescription: "Workspace id (`wrkspc_...`).", Required: true},
			"members": schema.ListNestedAttribute{
				MarkdownDescription: "Members.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"user_id":        dsString("User id."),
					"workspace_role": dsString("Workspace role."),
				}},
			},
		},
	}
}

func (d *workspaceMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *workspaceMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg workspaceMembersModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListWorkspaceMembers(ctx, cfg.WorkspaceID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing workspace members", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, m := range list {
		obj, diags := types.ObjectValue(attrTypesWorkspaceMember, map[string]attr.Value{
			"user_id":        types.StringValue(m.UserID),
			"workspace_role": types.StringValue(m.WorkspaceRole),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesWorkspaceMember}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Members = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_workspace_member --------------------------------------------

var _ datasource.DataSourceWithConfigure = &workspaceMemberDataSource{}

// NewWorkspaceMemberDataSource returns the anthropic_workspace_member data source.
func NewWorkspaceMemberDataSource() datasource.DataSource { return &workspaceMemberDataSource{} }

type workspaceMemberDataSource struct{ client *client.Client }

type workspaceMemberDSModel struct {
	WorkspaceID   types.String `tfsdk:"workspace_id"`
	UserID        types.String `tfsdk:"user_id"`
	WorkspaceRole types.String `tfsdk:"workspace_role"`
}

func (d *workspaceMemberDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_member"
}

func (d *workspaceMemberDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one workspace membership. Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"workspace_id":   schema.StringAttribute{MarkdownDescription: "Workspace id (`wrkspc_...`).", Required: true},
			"user_id":        schema.StringAttribute{MarkdownDescription: "User id (`user_...`).", Required: true},
			"workspace_role": dsString("Workspace role."),
		},
	}
}

func (d *workspaceMemberDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *workspaceMemberDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg workspaceMemberDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := d.client.FindWorkspaceMember(ctx, cfg.WorkspaceID.ValueString(), cfg.UserID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading workspace member", err)
		return
	}
	cfg.WorkspaceRole = types.StringValue(m.WorkspaceRole)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
