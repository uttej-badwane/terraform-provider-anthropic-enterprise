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

func init() { registerDataSource(NewServiceAccountDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &serviceAccountDataSource{}
	_ datasource.DataSourceWithConfigValidators = &serviceAccountDataSource{}
)

// NewServiceAccountDataSource returns the anthropic_service_account data source.
func NewServiceAccountDataSource() datasource.DataSource { return &serviceAccountDataSource{} }

type serviceAccountDataSource struct{ client *client.Client }

func (d *serviceAccountDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (d *serviceAccountDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up one service account by id or name. Requires `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{MarkdownDescription: "Service account id (`svac_...`). Exactly one of `id` or `name` must be set.", Optional: true, Computed: true},
			"name":              schema.StringAttribute{MarkdownDescription: "Slug name; only live (unarchived) service accounts are matched.", Optional: true, Computed: true},
			"description":       dsString("Description; null when empty."),
			"organization_role": dsString("Organization role (`developer` or `admin`)."),
			"created_at":        dsString("Creation timestamp."),
			"updated_at":        dsString("Last update timestamp."),
			"archived_at":       dsString("Archive timestamp; null while live."),
		},
	}
}

func (d *serviceAccountDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name"))}
}

func (d *serviceAccountDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredOAuth, &resp.Diagnostics)
}

func (d *serviceAccountDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var sa *client.ServiceAccount
	if !id.IsNull() {
		got, err := d.client.GetServiceAccount(ctx, id.ValueString())
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error reading service account", err)
			return
		}
		sa = got
	} else {
		list, err := d.client.ListServiceAccounts(ctx, false)
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error listing service accounts", err)
			return
		}
		for i := range list {
			if list[i].Name == name.ValueString() {
				if sa != nil {
					resp.Diagnostics.AddError("Ambiguous service account name", fmt.Sprintf("more than one live service account is named %q; use id", name.ValueString()))
					return
				}
				sa = &list[i]
			}
		}
		if sa == nil {
			resp.Diagnostics.AddError("Service account not found", fmt.Sprintf("no live service account named %q", name.ValueString()))
			return
		}
	}
	obj, diags := serviceAccountObject(sa)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
