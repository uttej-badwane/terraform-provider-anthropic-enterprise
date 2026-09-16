package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewFederationRuleWorkspacesDataSource) }

var _ datasource.DataSourceWithConfigure = &federationRuleWorkspacesDataSource{}

// NewFederationRuleWorkspacesDataSource returns the anthropic_federation_rule_workspaces data source.
func NewFederationRuleWorkspacesDataSource() datasource.DataSource {
	return &federationRuleWorkspacesDataSource{}
}

type federationRuleWorkspacesDataSource struct{ client *client.Client }

type federationRuleWorkspacesModel struct {
	FederationRuleID types.String `tfsdk:"federation_rule_id"`
	Workspaces       types.List   `tfsdk:"workspaces"`
}

var attrTypesRuleWorkspace = map[string]attr.Type{
	"workspace_id":   types.StringType,
	"workspace_name": types.StringType,
	"created_at":     types.StringType,
}

func (d *federationRuleWorkspacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rule_workspaces"
}

func (d *federationRuleWorkspacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the workspaces a workload identity federation rule is enabled for. Requires `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"federation_rule_id": schema.StringAttribute{MarkdownDescription: "Rule id (`fdrl_...`).", Required: true},
			"workspaces": schema.ListNestedAttribute{
				MarkdownDescription: "Workspace bindings.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"workspace_id":   dsString("Workspace id."),
					"workspace_name": dsString("Workspace name."),
					"created_at":     dsString("Timestamp the binding was created."),
				}},
			},
		},
	}
}

func (d *federationRuleWorkspacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredOAuth, &resp.Diagnostics)
}

func (d *federationRuleWorkspacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg federationRuleWorkspacesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListFederationRuleWorkspaces(ctx, cfg.FederationRuleID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing federation rule workspaces", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, b := range list {
		obj, diags := types.ObjectValue(attrTypesRuleWorkspace, map[string]attr.Value{
			"workspace_id":   types.StringValue(b.WorkspaceID),
			"workspace_name": stringFromPtr(b.WorkspaceName),
			"created_at":     types.StringValue(b.CreatedAt),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesRuleWorkspace}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Workspaces = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
