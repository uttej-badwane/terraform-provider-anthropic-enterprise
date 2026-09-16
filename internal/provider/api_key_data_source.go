package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAPIKeyDataSource) }

var _ datasource.DataSourceWithConfigure = &apiKeyDataSource{}

// NewAPIKeyDataSource returns the anthropic_api_key data source.
func NewAPIKeyDataSource() datasource.DataSource { return &apiKeyDataSource{} }

type apiKeyDataSource struct{ client *client.Client }

func (d *apiKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (d *apiKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one API key record by id. The secret is never returned. Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"id":               schema.StringAttribute{MarkdownDescription: "API key id (`apikey_...`).", Required: true},
			"name":             dsString("Display name."),
			"status":           dsString("`active`, `inactive`, `archived` or `expired`."),
			"created_at":       dsString("Creation timestamp."),
			"expires_at":       dsString("Expiry timestamp, if any."),
			"partial_key_hint": dsString("Redacted key hint."),
			"scope_type":       dsString("`workspace` or `organization`."),
			"workspace_id":     dsString("Scoped workspace id, if any."),
			"principal_type":   dsString("`user_actor` or `service_account_actor`."),
			"principal_id":     dsString("Principal id."),
			"created_by_id":    dsString("Creator id."),
			"created_by_type":  dsString("Creator type (`user` or `service_account`)."),
		},
	}
}

func (d *apiKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *apiKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	k, err := d.client.GetAPIKey(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading API key", err)
		return
	}
	obj, diags := apiKeyObject(k)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
