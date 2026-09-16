package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewRBACGroupsDataSource) }

var _ datasource.DataSourceWithConfigure = &rbacGroupsDataSource{}

// NewRBACGroupsDataSource returns the anthropic_rbac_groups data source.
func NewRBACGroupsDataSource() datasource.DataSource { return &rbacGroupsDataSource{} }

type rbacGroupsDataSource struct{ client *client.Client }

type rbacGroupsModel struct {
	Groups types.List `tfsdk:"groups"`
}

func (d *rbacGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbac_groups"
}

func (d *rbacGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists RBAC groups in a Claude Enterprise organization. Requires `enterprise_api_key`.",
		Attributes: map[string]schema.Attribute{
			"groups": schema.ListNestedAttribute{
				MarkdownDescription: "Groups.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":          dsString("Group id."),
					"name":        dsString("Group name."),
					"source_type": dsString("`direct` or `scim`."),
					"roles":       dsStringList("RBAC role ids assigned to the group; null when temporarily unavailable."),
					"created_at":  dsString("Creation timestamp."),
					"updated_at":  dsString("Last update timestamp."),
				}},
			},
		},
	}
}

func (d *rbacGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *rbacGroupsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.ListRBACGroups(ctx)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing RBAC groups", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for i := range list {
		obj, diags := rbacGroupObject(ctx, &list[i])
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesRBACGroup}, objs)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &rbacGroupsModel{Groups: l})...)
}
