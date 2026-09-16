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

func init() { registerDataSource(NewRBACGroupDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &rbacGroupDataSource{}
	_ datasource.DataSourceWithConfigValidators = &rbacGroupDataSource{}
)

// NewRBACGroupDataSource returns the anthropic_rbac_group data source.
func NewRBACGroupDataSource() datasource.DataSource { return &rbacGroupDataSource{} }

type rbacGroupDataSource struct{ client *client.Client }

func (d *rbacGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbac_group"
}

func (d *rbacGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up one RBAC group in a Claude Enterprise organization by id or name. Requires `enterprise_api_key`.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{MarkdownDescription: "Group id (`rbac_group_...`). Exactly one of `id` or `name` must be set.", Optional: true, Computed: true},
			"name":        schema.StringAttribute{MarkdownDescription: "Group name. Names are not unique; the lookup fails when more than one group matches.", Optional: true, Computed: true},
			"source_type": dsString("`direct` or `scim`."),
			"roles":       dsStringList("RBAC role ids assigned to the group; null when temporarily unavailable."),
			"created_at":  dsString("Creation timestamp."),
			"updated_at":  dsString("Last update timestamp."),
		},
	}
}

func (d *rbacGroupDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name"))}
}

func (d *rbacGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredEnterprise, &resp.Diagnostics)
}

func (d *rbacGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var g *client.RBACGroup
	if !id.IsNull() {
		got, err := d.client.GetRBACGroup(ctx, id.ValueString())
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error reading RBAC group", err)
			return
		}
		g = got
	} else {
		list, err := d.client.ListRBACGroups(ctx)
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error listing RBAC groups", err)
			return
		}
		for i := range list {
			if list[i].Name == name.ValueString() {
				if g != nil {
					resp.Diagnostics.AddError("Ambiguous RBAC group name", fmt.Sprintf("more than one group is named %q; use id", name.ValueString()))
					return
				}
				g = &list[i]
			}
		}
		if g == nil {
			resp.Diagnostics.AddError("RBAC group not found", fmt.Sprintf("no group named %q", name.ValueString()))
			return
		}
	}
	obj, diags := rbacGroupObject(ctx, g)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
