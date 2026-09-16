package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewWorkspaceServiceAccountsDataSource) }

var _ datasource.DataSourceWithConfigure = &workspaceServiceAccountsDataSource{}

// NewWorkspaceServiceAccountsDataSource returns the anthropic_workspace_service_accounts data source.
func NewWorkspaceServiceAccountsDataSource() datasource.DataSource {
	return &workspaceServiceAccountsDataSource{}
}

type workspaceServiceAccountsDataSource struct{ client *client.Client }

type workspaceServiceAccountsModel struct {
	WorkspaceID     types.String `tfsdk:"workspace_id"`
	ServiceAccounts types.List   `tfsdk:"service_accounts"`
}

var attrTypesWorkspaceServiceAccount = map[string]attr.Type{
	"service_account_id":  types.StringType,
	"workspace_role":      types.StringType,
	"implicit":            types.BoolType,
	"created_by_actor_id": types.StringType,
}

func (d *workspaceServiceAccountsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_service_accounts"
}

func (d *workspaceServiceAccountsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the service accounts that are members of a workspace. Requires `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"workspace_id": schema.StringAttribute{MarkdownDescription: "Workspace id (`wrkspc_...`).", Required: true},
			"service_accounts": schema.ListNestedAttribute{
				MarkdownDescription: "Service-account memberships.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"service_account_id":  dsString("Service account id."),
					"workspace_role":      dsString("Workspace role."),
					"implicit":            dsBool("True for the implicit default-workspace membership."),
					"created_by_actor_id": dsString("Actor that created the membership; null when implicit."),
				}},
			},
		},
	}
}

func (d *workspaceServiceAccountsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredOAuth, &resp.Diagnostics)
}

func (d *workspaceServiceAccountsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg workspaceServiceAccountsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListWorkspaceServiceAccounts(ctx, cfg.WorkspaceID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing workspace service accounts", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, m := range list {
		implicit := types.BoolValue(false)
		if m.Implicit != nil {
			implicit = types.BoolValue(*m.Implicit)
		}
		obj, diags := types.ObjectValue(attrTypesWorkspaceServiceAccount, map[string]attr.Value{
			"service_account_id":  types.StringValue(m.ServiceAccountID),
			"workspace_role":      types.StringValue(m.WorkspaceRole),
			"implicit":            implicit,
			"created_by_actor_id": stringFromPtr(m.CreatedByActorID),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesWorkspaceServiceAccount}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.ServiceAccounts = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
