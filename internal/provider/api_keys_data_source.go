package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAPIKeysDataSource) }

var _ datasource.DataSourceWithConfigure = &apiKeysDataSource{}

// NewAPIKeysDataSource returns the anthropic_api_keys data source.
func NewAPIKeysDataSource() datasource.DataSource { return &apiKeysDataSource{} }

type apiKeysDataSource struct{ client *client.Client }

type apiKeysModel struct {
	Status          types.String `tfsdk:"status"`
	WorkspaceID     types.String `tfsdk:"workspace_id"`
	CreatedByUserID types.String `tfsdk:"created_by_user_id"`
	APIKeys         types.List   `tfsdk:"api_keys"`
}

func (d *apiKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_keys"
}

func (d *apiKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists API key records (never the secrets). Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"status":             schema.StringAttribute{MarkdownDescription: "Filter by status: `active`, `inactive`, `archived`, `expired`.", Optional: true},
			"workspace_id":       schema.StringAttribute{MarkdownDescription: "Filter by workspace id.", Optional: true},
			"created_by_user_id": schema.StringAttribute{MarkdownDescription: "Filter by creating user id.", Optional: true},
			"api_keys": schema.ListNestedAttribute{
				MarkdownDescription: "API keys.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":               dsString("API key id."),
					"name":             dsString("Display name."),
					"status":           dsString("Status."),
					"created_at":       dsString("Creation timestamp."),
					"expires_at":       dsString("Expiry timestamp, if any."),
					"partial_key_hint": dsString("Redacted key hint."),
					"scope_type":       dsString("`workspace` or `organization`."),
					"workspace_id":     dsString("Scoped workspace id, if any."),
					"principal_type":   dsString("`user_actor` or `service_account_actor`."),
					"principal_id":     dsString("Principal id."),
					"created_by_id":    dsString("Creator id."),
					"created_by_type":  dsString("Creator type."),
				}},
			},
		},
	}
}

func (d *apiKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *apiKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg apiKeysModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListAPIKeys(ctx, client.APIKeyListOptions{
		Status:          cfg.Status.ValueString(),
		WorkspaceID:     cfg.WorkspaceID.ValueString(),
		CreatedByUserID: cfg.CreatedByUserID.ValueString(),
	})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing API keys", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for i := range list {
		obj, diags := apiKeyObject(&list[i])
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesAPIKey}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.APIKeys = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
