package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewVaultDataSource)
	registerDataSource(NewVaultsDataSource)
	registerDataSource(NewVaultCredentialsDataSource)
}

var vaultAttrTypes = map[string]attr.Type{
	"id": types.StringType, "display_name": types.StringType, "metadata": types.MapType{ElemType: types.StringType},
	"created_at": types.StringType, "updated_at": types.StringType, "archived_at": types.StringType,
}

func vaultRow(ctx context.Context, v *client.Vault, diags *diag.Diagnostics) map[string]attr.Value {
	meta, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(v.Metadata))
	diags.Append(d...)
	return map[string]attr.Value{
		"id": types.StringValue(v.ID), "display_name": types.StringValue(v.DisplayName), "metadata": meta,
		"created_at": types.StringValue(v.CreatedAt), "updated_at": types.StringValue(v.UpdatedAt), "archived_at": stringFromPtr(v.ArchivedAt),
	}
}

// --- anthropic_vault ----------------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &vaultDataSource{}

// NewVaultDataSource returns the anthropic_vault data source.
func NewVaultDataSource() datasource.DataSource { return &vaultDataSource{} }

type vaultDataSource struct{ client *client.Client }

func (d *vaultDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

func (d *vaultDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one Managed Agents vault." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{MarkdownDescription: "Vault id (`vlt_...`).", Required: true},
			"display_name": dsString("Display name."),
			"metadata":     schema.MapAttribute{MarkdownDescription: "Metadata.", Computed: true, ElementType: types.StringType},
			"created_at":   dsString("Creation timestamp."),
			"updated_at":   dsString("Last update timestamp."),
			"archived_at":  dsString("Archive timestamp; null while live."),
		},
	}
}

func (d *vaultDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *vaultDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	v, err := d.client.GetVault(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading vault", err)
		return
	}
	obj, diags := types.ObjectValue(vaultAttrTypes, vaultRow(ctx, v, &resp.Diagnostics))
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}

// --- anthropic_vaults ---------------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &vaultsDataSource{}

// NewVaultsDataSource returns the anthropic_vaults data source.
func NewVaultsDataSource() datasource.DataSource { return &vaultsDataSource{} }

type vaultsDataSource struct{ client *client.Client }

type vaultsModel struct {
	IncludeArchived types.Bool `tfsdk:"include_archived"`
	Vaults          types.List `tfsdk:"vaults"`
}

func (d *vaultsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vaults"
}

func (d *vaultsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Managed Agents vaults." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived vaults. Defaults to `false`.", Optional: true},
			"vaults": schema.ListNestedAttribute{MarkdownDescription: "Vaults.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"id": dsString("Vault id."), "display_name": dsString("Display name."),
				"metadata":   schema.MapAttribute{MarkdownDescription: "Metadata.", Computed: true, ElementType: types.StringType},
				"created_at": dsString("Creation timestamp."), "updated_at": dsString("Last update timestamp."), "archived_at": dsString("Archive timestamp."),
			}}},
		},
	}
}

func (d *vaultsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *vaultsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg vaultsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListVaults(ctx, cfg.IncludeArchived.ValueBool())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing vaults", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for i := range list {
		rows = append(rows, vaultRow(ctx, &list[i], &resp.Diagnostics))
	}
	cfg.Vaults = objectList(&resp.Diagnostics, vaultAttrTypes, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_vault_credentials -----------------------------------------------------

var _ datasource.DataSourceWithConfigure = &vaultCredentialsDataSource{}

// NewVaultCredentialsDataSource returns the anthropic_vault_credentials data source.
func NewVaultCredentialsDataSource() datasource.DataSource { return &vaultCredentialsDataSource{} }

type vaultCredentialsDataSource struct{ client *client.Client }

type vaultCredentialsModel struct {
	VaultID         types.String `tfsdk:"vault_id"`
	IncludeArchived types.Bool   `tfsdk:"include_archived"`
	Credentials     types.List   `tfsdk:"credentials"`
}

var vaultCredentialAttrTypes = map[string]attr.Type{
	"id": types.StringType, "display_name": types.StringType, "auth_type": types.StringType, "mcp_server_url": types.StringType,
	"secret_name": types.StringType, "created_at": types.StringType, "updated_at": types.StringType, "archived_at": types.StringType,
}

func (d *vaultCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault_credentials"
}

func (d *vaultCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the credentials stored in a vault. Secret values are never returned by the API and never appear here." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"vault_id":         schema.StringAttribute{MarkdownDescription: "Vault id (`vlt_...`).", Required: true},
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived credentials. Defaults to `false`.", Optional: true},
			"credentials": schema.ListNestedAttribute{MarkdownDescription: "Credentials.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"id": dsString("Credential id (`vcrd_...`)."), "display_name": dsString("Display name."),
				"auth_type":      dsString("`static_bearer`, `environment_variable` or `mcp_oauth`."),
				"mcp_server_url": dsString("MCP server URL (bearer and OAuth credentials)."),
				"secret_name":    dsString("Environment variable name (environment_variable credentials)."),
				"created_at":     dsString("Creation timestamp."), "updated_at": dsString("Last update timestamp."), "archived_at": dsString("Archive timestamp."),
			}}},
		},
	}
}

func (d *vaultCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *vaultCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg vaultCredentialsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListVaultCredentials(ctx, cfg.VaultID.ValueString(), cfg.IncludeArchived.ValueBool())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing vault credentials", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for _, c := range list {
		var auth struct {
			Type         string  `json:"type"`
			MCPServerURL *string `json:"mcp_server_url"`
			SecretName   *string `json:"secret_name"`
		}
		_ = json.Unmarshal(c.Auth, &auth)
		rows = append(rows, map[string]attr.Value{
			"id": types.StringValue(c.ID), "display_name": stringFromPtr(c.DisplayName), "auth_type": types.StringValue(auth.Type),
			"mcp_server_url": stringFromPtr(auth.MCPServerURL), "secret_name": stringFromPtr(auth.SecretName),
			"created_at": types.StringValue(c.CreatedAt), "updated_at": types.StringValue(c.UpdatedAt), "archived_at": stringFromPtr(c.ArchivedAt),
		})
	}
	cfg.Credentials = objectList(&resp.Diagnostics, vaultCredentialAttrTypes, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
