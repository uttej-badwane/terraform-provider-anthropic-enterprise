package provider

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func init() { registerDataSource(NewComplianceEffectiveSettingsDataSource) }

var _ datasource.DataSourceWithConfigure = &complianceEffectiveSettingsDataSource{}

// NewComplianceEffectiveSettingsDataSource returns the anthropic_compliance_effective_settings data source.
func NewComplianceEffectiveSettingsDataSource() datasource.DataSource {
	return &complianceEffectiveSettingsDataSource{}
}

type complianceEffectiveSettingsDataSource struct{ complianceBase }

type complianceEffectiveSettingsModel struct {
	OrganizationUUID types.String `tfsdk:"organization_uuid"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	Settings         types.List   `tfsdk:"settings"`
	SettingsMap      types.Map    `tfsdk:"settings_map"`
	APIKeys          types.List   `tfsdk:"api_keys"`
}

var attrTypesEffectiveSetting = map[string]attr.Type{
	"name": types.StringType, "type": types.StringType, "value_json": types.StringType,
}

var attrTypesComplianceAPIKey = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "scopes": types.ListType{ElemType: types.StringType},
	"is_active": types.BoolType, "created_at": types.StringType, "created_by_id": types.StringType, "expires_at": types.StringType,
}

func (d *complianceEffectiveSettingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_effective_settings"
}

func (d *complianceEffectiveSettingsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the settings actually in force for one linked organization (retention, redaction, SSO " +
			"provisioning mode, IP allowlist, session duration, capability toggles) after policy and dependency rules are applied. " +
			"Use it to attest a security baseline in policy checks.\n\n" +
			"~> The parent organization itself is not a valid target and returns not found. A setting that is missing from the " +
			"response is not controllable by the organization's administrators; it does not mean the setting is off." + complianceNote,
		Attributes: map[string]schema.Attribute{
			"organization_uuid": schema.StringAttribute{MarkdownDescription: "UUID of a linked organization (not the parent).", Required: true},
			"organization_id":   dsString("Organization UUID echoed by the API."),
			"settings": schema.ListNestedAttribute{
				MarkdownDescription: "Typed setting rows.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":       dsString("Setting name, for example `content_redaction_enabled`."),
					"type":       dsString("`boolean`, `integer`, `string_list`, `provisioning_mode` or `data_retention`."),
					"value_json": dsString("The value as compact JSON (for example `true`, `28800`, `[\"10.0.0.0/8\"]`). Decode with `jsondecode`."),
				}},
			},
			"settings_map": schema.MapAttribute{
				MarkdownDescription: "The same rows keyed by setting name; values are compact JSON strings.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"api_keys": schema.ListNestedAttribute{
				MarkdownDescription: "Every Compliance Access Key configured for the parent organization (secrets are never returned).",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":            dsString("Key id."),
					"name":          dsString("Key name."),
					"scopes":        dsStringList("Scopes carried by the key."),
					"is_active":     dsBool("Whether the key is active."),
					"created_at":    dsString("Creation timestamp."),
					"created_by_id": dsString("Id of the creating user; may be null."),
					"expires_at":    dsString("Expiry timestamp; null when the key does not expire."),
				}},
			},
		},
	}
}

func compactJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

func (d *complianceEffectiveSettingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg complianceEffectiveSettingsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	es, err := d.client.GetEffectiveOrganizationSettings(ctx, cfg.OrganizationUUID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading effective organization settings", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(es.Settings))
	byName := map[string]string{}
	for _, s := range es.Settings {
		v := compactJSON(s.Value)
		byName[s.Name] = v
		rows = append(rows, map[string]attr.Value{
			"name": types.StringValue(s.Name), "type": types.StringValue(s.Type), "value_json": types.StringValue(v),
		})
	}
	keys := make([]map[string]attr.Value, 0, len(es.APIKeys))
	for _, k := range es.APIKeys {
		scopes, diags := types.ListValueFrom(ctx, types.StringType, nonNilStrings(k.Scopes))
		resp.Diagnostics.Append(diags...)
		keys = append(keys, map[string]attr.Value{
			"id": types.StringValue(k.ID), "name": types.StringValue(k.Name), "scopes": scopes, "is_active": types.BoolValue(k.IsActive),
			"created_at": types.StringValue(k.CreatedAt), "created_by_id": stringFromPtr(k.CreatedByID), "expires_at": stringFromPtr(k.ExpiresAt),
		})
	}
	m, diags := types.MapValueFrom(ctx, types.StringType, byName)
	resp.Diagnostics.Append(diags...)
	cfg.OrganizationID = types.StringValue(es.OrganizationID)
	cfg.Settings = objectList(&resp.Diagnostics, attrTypesEffectiveSetting, rows)
	cfg.SettingsMap = m
	cfg.APIKeys = objectList(&resp.Diagnostics, attrTypesComplianceAPIKey, keys)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
