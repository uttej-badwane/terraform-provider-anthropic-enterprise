package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewAPIKeyResource) }

var (
	_ resource.Resource                = &apiKeyResource{}
	_ resource.ResourceWithConfigure   = &apiKeyResource{}
	_ resource.ResourceWithImportState = &apiKeyResource{}
)

// NewAPIKeyResource returns the anthropic_api_key resource.
func NewAPIKeyResource() resource.Resource { return &apiKeyResource{} }

type apiKeyResource struct {
	client *client.Client
}

type apiKeyModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Status              types.String `tfsdk:"status"`
	DeactivateOnDestroy types.Bool   `tfsdk:"deactivate_on_destroy"`
	CreatedAt           types.String `tfsdk:"created_at"`
	ExpiresAt           types.String `tfsdk:"expires_at"`
	PartialKeyHint      types.String `tfsdk:"partial_key_hint"`
	ScopeType           types.String `tfsdk:"scope_type"`
	WorkspaceID         types.String `tfsdk:"workspace_id"`
	PrincipalType       types.String `tfsdk:"principal_type"`
	PrincipalID         types.String `tfsdk:"principal_id"`
	CreatedByID         types.String `tfsdk:"created_by_id"`
	CreatedByType       types.String `tfsdk:"created_by_type"`
}

const apiKeyCreateError = "API keys cannot be created through the Admin API. Create the key in the Claude Console, " +
	"then import it: terraform import anthropic_api_key.<name> apikey_..."

func (r *apiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (r *apiKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the name and status of an existing API key. The Admin API cannot create keys, so this " +
			"resource is **import-only**: create the key in the Claude Console, then `terraform import anthropic_api_key.<name> apikey_...`. " +
			"Applying a configuration for a key that is not in state fails.\n\n" +
			"Requires `admin_api_key` or `oauth_token`. The key secret is never returned by the API and never stored in state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "API key id (`apikey_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "One of `active`, `inactive`, `archived`. Archiving is irreversible.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.OneOf("active", "inactive", "archived")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"deactivate_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Set the key to `inactive` when the resource is destroyed. Defaults to `false`, " +
					"in which case destroy only removes the key from state.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"created_at":       computedString("Creation timestamp (RFC 3339)."),
			"expires_at":       schema.StringAttribute{MarkdownDescription: "Expiry timestamp; null when the key does not expire.", Computed: true},
			"partial_key_hint": computedString("Redacted hint of the key value."),
			"scope_type":       computedString("`workspace` or `organization`."),
			"workspace_id":     schema.StringAttribute{MarkdownDescription: "Workspace the key is scoped to; null for organization-scoped keys.", Computed: true},
			"principal_type":   schema.StringAttribute{MarkdownDescription: "`user_actor` or `service_account_actor`; null for legacy keys.", Computed: true},
			"principal_id":     schema.StringAttribute{MarkdownDescription: "User or service account id the key acts as.", Computed: true},
			"created_by_id":    schema.StringAttribute{MarkdownDescription: "Id of the actor that created the key.", Computed: true},
			"created_by_type":  schema.StringAttribute{MarkdownDescription: "`user` or `service_account`.", Computed: true},
		},
	}
}

func computedString(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: desc,
		Computed:            true,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

func (r *apiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAdmin, &resp.Diagnostics)
}

func (r *apiKeyResource) Create(_ context.Context, _ resource.CreateRequest, resp *resource.CreateResponse) {
	resp.Diagnostics.AddError("API keys cannot be created", apiKeyCreateError)
}

func (r *apiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	k, err := r.client.GetAPIKey(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading API key", err)
		return
	}
	flattenAPIKey(k, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *apiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state apiKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.APIKeyUpdate{}
	if !plan.Name.IsUnknown() && !plan.Name.IsNull() && !plan.Name.Equal(state.Name) {
		in.Name = stringPtr(plan.Name)
	}
	if !plan.Status.IsUnknown() && !plan.Status.IsNull() && !plan.Status.Equal(state.Status) {
		in.Status = stringPtr(plan.Status)
	}
	var (
		k   *client.APIKey
		err error
	)
	if in.Name != nil || in.Status != nil {
		k, err = r.client.UpdateAPIKey(ctx, state.ID.ValueString(), in)
	} else {
		k, err = r.client.GetAPIKey(ctx, state.ID.ValueString())
	}
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating API key", err)
		return
	}
	newState := plan
	flattenAPIKey(k, &newState)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *apiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.DeactivateOnDestroy.ValueBool() {
		tflog.Info(ctx, "deactivate_on_destroy is false; leaving API key in place", map[string]any{"id": state.ID.ValueString()})
		return
	}
	inactive := "inactive"
	if _, err := r.client.UpdateAPIKey(ctx, state.ID.ValueString(), client.APIKeyUpdate{Status: &inactive}); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error deactivating API key", err)
	}
}

func (r *apiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deactivate_on_destroy"), types.BoolValue(false))...)
}

func flattenAPIKey(k *client.APIKey, m *apiKeyModel) {
	m.ID = types.StringValue(k.ID)
	m.Name = types.StringValue(k.Name)
	m.Status = types.StringValue(k.Status)
	if m.DeactivateOnDestroy.IsNull() || m.DeactivateOnDestroy.IsUnknown() {
		m.DeactivateOnDestroy = types.BoolValue(false)
	}
	m.CreatedAt = types.StringValue(k.CreatedAt)
	m.ExpiresAt = stringFromPtr(k.ExpiresAt)
	m.PartialKeyHint = stringFromPtr(k.PartialKeyHint)
	m.ScopeType = types.StringValue(k.Scope.Type)
	m.WorkspaceID = stringFromPtr(k.Scope.WorkspaceID)
	m.PrincipalType, m.PrincipalID = types.StringNull(), types.StringNull()
	if k.Principal != nil {
		m.PrincipalType = types.StringValue(k.Principal.Type)
		switch {
		case k.Principal.UserID != nil:
			m.PrincipalID = types.StringValue(*k.Principal.UserID)
		case k.Principal.ServiceAccountID != nil:
			m.PrincipalID = types.StringValue(*k.Principal.ServiceAccountID)
		}
	}
	m.CreatedByID, m.CreatedByType = types.StringNull(), types.StringNull()
	if k.CreatedBy != nil {
		m.CreatedByID = types.StringValue(k.CreatedBy.ID)
		m.CreatedByType = types.StringValue(k.CreatedBy.Type)
	}
}

var attrTypesAPIKey = map[string]attr.Type{
	"id":               types.StringType,
	"name":             types.StringType,
	"status":           types.StringType,
	"created_at":       types.StringType,
	"expires_at":       types.StringType,
	"partial_key_hint": types.StringType,
	"scope_type":       types.StringType,
	"workspace_id":     types.StringType,
	"principal_type":   types.StringType,
	"principal_id":     types.StringType,
	"created_by_id":    types.StringType,
	"created_by_type":  types.StringType,
}

func apiKeyObject(k *client.APIKey) (types.Object, diag.Diagnostics) {
	var m apiKeyModel
	flattenAPIKey(k, &m)
	return types.ObjectValue(attrTypesAPIKey, map[string]attr.Value{
		"id":               m.ID,
		"name":             m.Name,
		"status":           m.Status,
		"created_at":       m.CreatedAt,
		"expires_at":       m.ExpiresAt,
		"partial_key_hint": m.PartialKeyHint,
		"scope_type":       m.ScopeType,
		"workspace_id":     m.WorkspaceID,
		"principal_type":   m.PrincipalType,
		"principal_id":     m.PrincipalID,
		"created_by_id":    m.CreatedByID,
		"created_by_type":  m.CreatedByType,
	})
}
