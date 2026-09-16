package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewExternalKeyDataSource) }

var _ datasource.DataSourceWithConfigure = &externalKeyDataSource{}

// NewExternalKeyDataSource returns the anthropic_external_key data source.
func NewExternalKeyDataSource() datasource.DataSource { return &externalKeyDataSource{} }

type externalKeyDataSource struct{ client *client.Client }

func externalKeySummaryObject(k *client.ExternalKey) (types.Object, diag.Diagnostics) {
	return types.ObjectValue(attrTypesExternalKeySummary, map[string]attr.Value{
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
}

func (d *externalKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_key"
}

func (d *externalKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one customer-managed encryption key registration by id. Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{MarkdownDescription: "External key id (`ekey_...`).", Required: true},
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
		},
	}
}

func (d *externalKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *externalKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	k, err := d.client.GetExternalKey(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading external key", err)
		return
	}
	obj, diags := externalKeySummaryObject(k)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
