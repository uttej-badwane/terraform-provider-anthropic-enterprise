package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewExternalKeysDataSource) }

var _ datasource.DataSourceWithConfigure = &externalKeysDataSource{}

// NewExternalKeysDataSource returns the anthropic_external_keys data source.
func NewExternalKeysDataSource() datasource.DataSource { return &externalKeysDataSource{} }

type externalKeysDataSource struct{ client *client.Client }

type externalKeysModel struct {
	ExternalKeys types.List `tfsdk:"external_keys"`
}

var attrTypesExternalKeySummary = map[string]attr.Type{
	"id":              types.StringType,
	"display_name":    types.StringType,
	"geo":             types.StringType,
	"provider_type":   types.StringType,
	"kms_arn":         types.StringType,
	"region":          types.StringType,
	"key_name":        types.StringType,
	"vault_uri":       types.StringType,
	"tenant_id":       types.StringType,
	"attachment_type": types.StringType,
	"created_at":      types.StringType,
}

func (d *externalKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_keys"
}

func (d *externalKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists customer-managed encryption key registrations. Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"external_keys": schema.ListNestedAttribute{
				MarkdownDescription: "External keys.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":              dsString("External key id."),
					"display_name":    dsString("Display name."),
					"geo":             dsString("Geography."),
					"provider_type":   dsString("`aws`, `gcp` or `azure`."),
					"kms_arn":         dsString("AWS KMS key ARN (aws)."),
					"region":          dsString("AWS region (aws)."),
					"key_name":        dsString("Key name (gcp, azure)."),
					"vault_uri":       dsString("Key Vault URI (azure)."),
					"tenant_id":       dsString("Tenant id (azure)."),
					"attachment_type": dsString("`attached` or `unattached`."),
					"created_at":      dsString("Creation timestamp."),
				}},
			},
		},
	}
}

func (d *externalKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *externalKeysDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, err := d.client.ListExternalKeys(ctx)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing external keys", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for _, k := range list {
		obj, diags := types.ObjectValue(attrTypesExternalKeySummary, map[string]attr.Value{
			"id":              types.StringValue(k.ID),
			"display_name":    stringFromPtr(k.DisplayName),
			"geo":             types.StringValue(k.Geo),
			"provider_type":   types.StringValue(k.ProviderConfig.Type),
			"kms_arn":         stringFromPtr(k.ProviderConfig.KMSARN),
			"region":          stringFromPtr(k.ProviderConfig.Region),
			"key_name":        stringFromPtr(k.ProviderConfig.KeyName),
			"vault_uri":       stringFromPtr(k.ProviderConfig.VaultURI),
			"tenant_id":       stringFromPtr(k.ProviderConfig.TenantID),
			"attachment_type": types.StringValue(k.Attachment.Type),
			"created_at":      types.StringValue(k.CreatedAt),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesExternalKeySummary}, objs)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &externalKeysModel{ExternalKeys: l})...)
}
