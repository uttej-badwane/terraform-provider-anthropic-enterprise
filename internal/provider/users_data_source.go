package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewUsersDataSource) }

var _ datasource.DataSourceWithConfigure = &usersDataSource{}

// NewUsersDataSource returns the anthropic_users data source.
func NewUsersDataSource() datasource.DataSource { return &usersDataSource{} }

type usersDataSource struct{ client *client.Client }

type usersModel struct {
	Email types.String `tfsdk:"email"`
	Roles types.List   `tfsdk:"roles"`
	Users types.List   `tfsdk:"users"`
}

func (d *usersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists organization members. Works with any admin or enterprise credential.",
		Attributes: map[string]schema.Attribute{
			"email": schema.StringAttribute{MarkdownDescription: "Filter by email address.", Optional: true},
			"roles": schema.ListAttribute{MarkdownDescription: "Filter by organization roles (any match).", Optional: true, ElementType: types.StringType},
			"users": schema.ListNestedAttribute{
				MarkdownDescription: "Members.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":       dsString("User id."),
					"email":    dsString("Email address."),
					"name":     dsString("Display name."),
					"role":     dsString("Organization role."),
					"added_at": dsString("Timestamp the member joined."),
				}},
			},
		},
	}
}

func (d *usersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = memberClientFromDataSource(req, &resp.Diagnostics)
}

func (d *usersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg usersModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts := client.UserListOptions{Email: cfg.Email.ValueString()}
	if !cfg.Roles.IsNull() {
		resp.Diagnostics.Append(cfg.Roles.ElementsAs(ctx, &opts.Roles, false)...)
	}
	list, err := d.client.ListUsers(ctx, opts)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing users", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for i := range list {
		obj, diags := userObject(&list[i])
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesUser}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Users = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
