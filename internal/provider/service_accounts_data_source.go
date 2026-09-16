package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewServiceAccountsDataSource) }

var _ datasource.DataSourceWithConfigure = &serviceAccountsDataSource{}

// NewServiceAccountsDataSource returns the anthropic_service_accounts data source.
func NewServiceAccountsDataSource() datasource.DataSource { return &serviceAccountsDataSource{} }

type serviceAccountsDataSource struct{ client *client.Client }

type serviceAccountsModel struct {
	IncludeArchived types.Bool `tfsdk:"include_archived"`
	ServiceAccounts types.List `tfsdk:"service_accounts"`
}

func (d *serviceAccountsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_accounts"
}

func (d *serviceAccountsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists service accounts in the organization. Requires `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived service accounts. Defaults to `false`.", Optional: true},
			"service_accounts": schema.ListNestedAttribute{
				MarkdownDescription: "Service accounts.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":                dsString("Service account id."),
					"name":              dsString("Slug name."),
					"description":       dsString("Description; null when empty."),
					"organization_role": dsString("Organization role (`developer` or `admin`)."),
					"created_at":        dsString("Creation timestamp."),
					"updated_at":        dsString("Last update timestamp."),
					"archived_at":       dsString("Archive timestamp; null while live."),
				}},
			},
		},
	}
}

func (d *serviceAccountsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredOAuth, &resp.Diagnostics)
}

func (d *serviceAccountsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg serviceAccountsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListServiceAccounts(ctx, cfg.IncludeArchived.ValueBool())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing service accounts", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for i := range list {
		obj, diags := serviceAccountObject(&list[i])
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesServiceAccount}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.ServiceAccounts = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
