package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewUserDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &userDataSource{}
	_ datasource.DataSourceWithConfigValidators = &userDataSource{}
)

// NewUserDataSource returns the anthropic_user data source.
func NewUserDataSource() datasource.DataSource { return &userDataSource{} }

type userDataSource struct{ client *client.Client }

func (d *userDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up one organization member by id or email. Works with any admin or enterprise credential.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{MarkdownDescription: "User id. Exactly one of `id` or `email` must be set.", Optional: true, Computed: true},
			"email":    schema.StringAttribute{MarkdownDescription: "Email address (case-insensitive).", Optional: true, Computed: true},
			"name":     dsString("Display name."),
			"role":     dsString("Organization role."),
			"added_at": dsString("Timestamp the member joined."),
		},
	}
}

func (d *userDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("email"))}
}

func (d *userDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = memberClientFromDataSource(req, &resp.Diagnostics)
}

func (d *userDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, email types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("email"), &email)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var u *client.User
	if !id.IsNull() {
		got, err := d.client.GetUser(ctx, id.ValueString())
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error reading user", err)
			return
		}
		u = got
	} else {
		list, err := d.client.ListUsers(ctx, client.UserListOptions{Email: email.ValueString()})
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error listing users", err)
			return
		}
		if len(list) == 0 {
			resp.Diagnostics.AddError("User not found", fmt.Sprintf("no member with email %q", email.ValueString()))
			return
		}
		u = &list[0]
	}
	obj, diags := userObject(u)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
