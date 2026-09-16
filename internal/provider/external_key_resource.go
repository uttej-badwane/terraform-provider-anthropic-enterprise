package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewExternalKeyResource) }

var (
	_ resource.Resource                   = &externalKeyResource{}
	_ resource.ResourceWithConfigure      = &externalKeyResource{}
	_ resource.ResourceWithImportState    = &externalKeyResource{}
	_ resource.ResourceWithValidateConfig = &externalKeyResource{}
)

// NewExternalKeyResource returns the anthropic_external_key resource.
func NewExternalKeyResource() resource.Resource { return &externalKeyResource{} }

type externalKeyResource struct {
	client *client.Client
}

type externalKeyModel struct {
	ID               types.String `tfsdk:"id"`
	DisplayName      types.String `tfsdk:"display_name"`
	Geo              types.String `tfsdk:"geo"`
	ProviderConfig   types.Object `tfsdk:"provider_config"`
	ValidateOnCreate types.Bool   `tfsdk:"validate_on_create"`
	AttachmentType   types.String `tfsdk:"attachment_type"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

type providerConfigModel struct {
	Type     types.String `tfsdk:"type"`
	KMSARN   types.String `tfsdk:"kms_arn"`
	Region   types.String `tfsdk:"region"`
	KeyName  types.String `tfsdk:"key_name"`
	TenantID types.String `tfsdk:"tenant_id"`
	VaultURI types.String `tfsdk:"vault_uri"`
	ClientID types.String `tfsdk:"client_id"`
}

var attrTypesProviderConfig = map[string]attr.Type{
	"type":      types.StringType,
	"kms_arn":   types.StringType,
	"region":    types.StringType,
	"key_name":  types.StringType,
	"tenant_id": types.StringType,
	"vault_uri": types.StringType,
	"client_id": types.StringType,
}

func (r *externalKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_key"
}

func (r *externalKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Registers a customer-managed encryption key (CMEK) held in AWS KMS, Google Cloud KMS or Azure Key " +
			"Vault. Attach it to a workspace with `external_key_id` on `anthropic_workspace`.\n\n" +
			"Requires `admin_api_key` or `oauth_token`.\n\n" +
			"~> A key can only be deleted while unattached, and `provider_config` cannot change once attached. " +
			"Changing `provider_config` or `geo` forces a new registration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "External key id (`ekey_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name. Removing it clears the name.",
				Optional:            true,
			},
			"geo": schema.StringAttribute{
				MarkdownDescription: "Geography the key serves. Only `us` today. Defaults to `us`; changing it forces a new registration.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("us"),
				Validators:          []validator.String{stringvalidator.OneOf("us")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"provider_config": schema.SingleNestedAttribute{
				MarkdownDescription: "Where the key lives. Changing any field forces a new registration.",
				Required:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.RequiresReplace()},
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "`aws`, `gcp` or `azure`.",
						Required:            true,
						Validators:          []validator.String{stringvalidator.OneOf("aws", "gcp", "azure")},
					},
					"kms_arn": schema.StringAttribute{
						MarkdownDescription: "AWS KMS key ARN. Required for `aws`.",
						Optional:            true,
					},
					"region": schema.StringAttribute{
						MarkdownDescription: "AWS region of the key. Derived from `kms_arn` when omitted.",
						Optional:            true,
						Computed:            true,
					},
					"key_name": schema.StringAttribute{
						MarkdownDescription: "Google Cloud KMS key resource name, or Azure Key Vault key name. Required for `gcp` and `azure`.",
						Optional:            true,
					},
					"tenant_id": schema.StringAttribute{
						MarkdownDescription: "Azure tenant id. Required for `azure`.",
						Optional:            true,
					},
					"vault_uri": schema.StringAttribute{
						MarkdownDescription: "Azure Key Vault or Managed HSM URI. Required for `azure`.",
						Optional:            true,
					},
					"client_id": schema.StringAttribute{
						MarkdownDescription: "Azure client id used to access the vault.",
						Optional:            true,
					},
				},
			},
			"validate_on_create": schema.BoolAttribute{
				MarkdownDescription: "Call the validate endpoint after registering the key and fail if the key is not reachable. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"attachment_type": schema.StringAttribute{
				MarkdownDescription: "`attached` when a workspace uses the key, otherwise `unattached`.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp (RFC 3339).",
				Computed:            true,
			},
		},
	}
}

func (r *externalKeyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg externalKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.ProviderConfig.IsNull() || cfg.ProviderConfig.IsUnknown() {
		return
	}
	var pc providerConfigModel
	resp.Diagnostics.Append(cfg.ProviderConfig.As(ctx, &pc, basetypesObjectAsOptions)...)
	if resp.Diagnostics.HasError() || pc.Type.IsUnknown() {
		return
	}
	need := func(v types.String, name string) {
		if v.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("provider_config").AtName(name), "Missing provider_config."+name,
				fmt.Sprintf("provider_config.%s is required when provider_config.type is %q.", name, pc.Type.ValueString()))
		}
	}
	switch pc.Type.ValueString() {
	case "aws":
		need(pc.KMSARN, "kms_arn")
	case "gcp":
		need(pc.KeyName, "key_name")
	case "azure":
		need(pc.KeyName, "key_name")
		need(pc.TenantID, "tenant_id")
		need(pc.VaultURI, "vault_uri")
	}
}

func (r *externalKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAdmin, &resp.Diagnostics)
}

func (r *externalKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan externalKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pc := expandProviderConfig(ctx, plan.ProviderConfig, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	key, err := r.client.CreateExternalKey(ctx, client.ExternalKeyCreate{
		ProviderConfig: pc,
		DisplayName:    stringPtr(plan.DisplayName),
		Geo:            stringPtr(plan.Geo),
	})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error registering external key", err)
		return
	}
	tflog.Trace(ctx, "created external key", map[string]any{"id": key.ID})

	state := plan
	resp.Diagnostics.Append(flattenExternalKey(key, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	if plan.ValidateOnCreate.ValueBool() {
		v, err := r.client.ValidateExternalKey(ctx, key.ID)
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error validating external key", err)
			return
		}
		if v.Status != "success" {
			msg := "the key could not be validated"
			if v.Error != nil {
				msg = *v.Error
			}
			resp.Diagnostics.AddError("External key validation failed",
				fmt.Sprintf("The key was registered as %s but validation failed: %s. Fix the key policy and re-run, or destroy the resource.", key.ID, msg))
		}
	}
}

func (r *externalKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state externalKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	key, err := r.client.GetExternalKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading external key", err)
		return
	}
	resp.Diagnostics.Append(flattenExternalKey(key, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *externalKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state externalKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.ExternalKeyUpdate{}
	if !plan.DisplayName.Equal(state.DisplayName) {
		if plan.DisplayName.IsNull() {
			in.DisplayName = client.Null[string]()
		} else {
			in.DisplayName = client.Some(plan.DisplayName.ValueString())
		}
	}
	key, err := r.client.UpdateExternalKey(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating external key", err)
		return
	}
	newState := plan
	resp.Diagnostics.Append(flattenExternalKey(key, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *externalKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state externalKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteExternalKey(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error deleting external key", err)
	}
}

func (r *externalKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("validate_on_create"), types.BoolValue(false))...)
}

func expandProviderConfig(ctx context.Context, obj types.Object, diags *diag.Diagnostics) client.ProviderConfig {
	var pc providerConfigModel
	diags.Append(obj.As(ctx, &pc, basetypesObjectAsOptions)...)
	return client.ProviderConfig{
		Type:     pc.Type.ValueString(),
		KMSARN:   stringPtr(pc.KMSARN),
		Region:   stringPtr(pc.Region),
		KeyName:  stringPtr(pc.KeyName),
		TenantID: stringPtr(pc.TenantID),
		VaultURI: stringPtr(pc.VaultURI),
		ClientID: stringPtr(pc.ClientID),
	}
}

func flattenProviderConfig(pc client.ProviderConfig) (types.Object, diag.Diagnostics) {
	return types.ObjectValue(attrTypesProviderConfig, map[string]attr.Value{
		"type":      types.StringValue(pc.Type),
		"kms_arn":   stringFromPtr(pc.KMSARN),
		"region":    stringFromPtr(pc.Region),
		"key_name":  stringFromPtr(pc.KeyName),
		"tenant_id": stringFromPtr(pc.TenantID),
		"vault_uri": stringFromPtr(pc.VaultURI),
		"client_id": stringFromPtr(pc.ClientID),
	})
}

func flattenExternalKey(key *client.ExternalKey, m *externalKeyModel) diag.Diagnostics {
	m.ID = types.StringValue(key.ID)
	m.DisplayName = stringFromPtr(key.DisplayName)
	m.Geo = types.StringValue(key.Geo)
	m.AttachmentType = types.StringValue(key.Attachment.Type)
	m.CreatedAt = types.StringValue(key.CreatedAt)
	m.UpdatedAt = types.StringValue(key.UpdatedAt)
	if m.ValidateOnCreate.IsNull() || m.ValidateOnCreate.IsUnknown() {
		m.ValidateOnCreate = types.BoolValue(false)
	}
	obj, diags := flattenProviderConfig(key.ProviderConfig)
	m.ProviderConfig = obj
	return diags
}
