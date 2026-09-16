package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewRBACGroupMembersDataSource) }

var _ datasource.DataSourceWithConfigure = &rbacGroupMembersDataSource{}

// NewRBACGroupMembersDataSource returns the anthropic_rbac_group_members data source.
func NewRBACGroupMembersDataSource() datasource.DataSource { return &rbacGroupMembersDataSource{} }

type rbacGroupMembersDataSource struct{ client *client.Client }

type rbacGroupMembersModel struct {
	GroupID types.String `tfsdk:"group_id"`
	Members types.List   `tfsdk:"members"`
}

var attrTypesRBACGroupMember = map[string]attr.Type{
	"user_id":    types.StringType,
	"email":      types.StringType,
	"created_at": types.StringType,
}

func (d *rbacGroupMembersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbac_group_members"
}

func (d *rbacGroupMembersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the members of an RBAC group in a Claude Enterprise organization. Requires `enterprise_api_key`.",
		Attributes: map[string]schema.Attribute{
			"group_id": schema.StringAttribute{MarkdownDescription: "Group id (`rbac_group_...`).", Required: true},
			"members": schema.ListNestedAttribute{
				MarkdownDescription: "Members.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"user_id":    dsString("User id."),
					"email":      dsString("Email address."),
					"created_at": dsString("Timestamp the user was added."),
				}},
			},
		},
	}
}

func (d *rbacGroupMembersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *rbacGroupMembersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg rbacGroupMembersModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListRBACGroupMembers(ctx, cfg.GroupID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing RBAC group members", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, m := range list {
		obj, diags := types.ObjectValue(attrTypesRBACGroupMember, map[string]attr.Value{
			"user_id":    types.StringValue(m.UserID),
			"email":      types.StringValue(m.Email),
			"created_at": types.StringValue(m.CreatedAt),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesRBACGroupMember}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Members = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
