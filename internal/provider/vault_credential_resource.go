package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
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

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewVaultCredentialResource) }

var (
	_ resource.Resource                     = &vaultCredentialResource{}
	_ resource.ResourceWithConfigure        = &vaultCredentialResource{}
	_ resource.ResourceWithImportState      = &vaultCredentialResource{}
	_ resource.ResourceWithConfigValidators = &vaultCredentialResource{}
)

// NewVaultCredentialResource returns the anthropic_vault_credential resource.
func NewVaultCredentialResource() resource.Resource { return &vaultCredentialResource{} }

type vaultCredentialResource struct{ client *client.Client }

type vaultCredentialModel struct {
	ID                  types.String `tfsdk:"id"`
	VaultID             types.String `tfsdk:"vault_id"`
	DisplayName         types.String `tfsdk:"display_name"`
	Metadata            types.Map    `tfsdk:"metadata"`
	SecretVersion       types.String `tfsdk:"secret_version"`
	DeleteOnDestroy     types.Bool   `tfsdk:"delete_on_destroy"`
	StaticBearer        types.Object `tfsdk:"static_bearer"`
	EnvironmentVariable types.Object `tfsdk:"environment_variable"`
	MCPOAuth            types.Object `tfsdk:"mcp_oauth"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
	ArchivedAt          types.String `tfsdk:"archived_at"`
}

type staticBearerModel struct {
	MCPServerURL types.String `tfsdk:"mcp_server_url"`
	Token        types.String `tfsdk:"token"`
}

type envVarModel struct {
	SecretName     types.String `tfsdk:"secret_name"`
	SecretValue    types.String `tfsdk:"secret_value"`
	NetworkingType types.String `tfsdk:"networking_type"`
	AllowedHosts   types.List   `tfsdk:"allowed_hosts"`
	InjectHeader   types.Bool   `tfsdk:"inject_header"`
	InjectBody     types.Bool   `tfsdk:"inject_body"`
}

type mcpOAuthModel struct {
	MCPServerURL types.String `tfsdk:"mcp_server_url"`
	AccessToken  types.String `tfsdk:"access_token"`
	ExpiresAt    types.String `tfsdk:"expires_at"`
	Refresh      types.Object `tfsdk:"refresh"`
}

type oauthRefreshModel struct {
	TokenEndpoint         types.String `tfsdk:"token_endpoint"`
	ClientID              types.String `tfsdk:"client_id"`
	RefreshToken          types.String `tfsdk:"refresh_token"`
	Scope                 types.String `tfsdk:"scope"`
	Resource              types.String `tfsdk:"resource"`
	TokenEndpointAuthType types.String `tfsdk:"token_endpoint_auth_type"`
	ClientSecret          types.String `tfsdk:"client_secret"`
}

var (
	staticBearerAttrTypes = map[string]attr.Type{"mcp_server_url": types.StringType, "token": types.StringType}
	envVarAttrTypes       = map[string]attr.Type{"secret_name": types.StringType, "secret_value": types.StringType, "networking_type": types.StringType,
		"allowed_hosts": types.ListType{ElemType: types.StringType}, "inject_header": types.BoolType, "inject_body": types.BoolType}
	oauthRefreshAttrTypes = map[string]attr.Type{"token_endpoint": types.StringType, "client_id": types.StringType, "refresh_token": types.StringType,
		"scope": types.StringType, "resource": types.StringType, "token_endpoint_auth_type": types.StringType, "client_secret": types.StringType}
	mcpOAuthAttrTypes = map[string]attr.Type{"mcp_server_url": types.StringType, "access_token": types.StringType, "expires_at": types.StringType,
		"refresh": types.ObjectType{AttrTypes: oauthRefreshAttrTypes}}
)

func (r *vaultCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault_credential"
}

func writeOnlySecret(desc string, required bool) schema.StringAttribute {
	return schema.StringAttribute{MarkdownDescription: desc + " Write-only: sent to the API, never stored in state, never drift-detected. Requires Terraform 1.11 or newer.",
		Required: required, Optional: !required, Sensitive: true, WriteOnly: true}
}

func (r *vaultCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a credential inside a Managed Agents vault. Exactly one of `static_bearer`, `environment_variable` " +
			"or `mcp_oauth` must be set. Secret values are write-only attributes: they are sent to the API and never stored in state, " +
			"so Terraform cannot detect out-of-band rotation. Change `secret_version` to force the secrets to be re-sent (rotation). " +
			"After `terraform import` the first plan re-sends the configured secrets because state holds none." + managedAgentsNote + "\n\n" +
			"~> The credential's key (`mcp_server_url` or `secret_name`) and OAuth `token_endpoint` / `client_id` are immutable: " +
			"changing them replaces the credential.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Credential id (`vcrd_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vault_id": schema.StringAttribute{
				MarkdownDescription: "Vault the credential belongs to (`vlt_...`).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{MarkdownDescription: "Human-readable label (max 255).", Optional: true, Validators: []validator.String{stringvalidator.LengthAtMost(255)}},
			"metadata":     schema.MapAttribute{MarkdownDescription: "Key/value metadata (up to 16 pairs).", ElementType: types.StringType, Optional: true},
			"secret_version": schema.StringAttribute{
				MarkdownDescription: "Opaque rotation trigger. Change this value (for example a date) to re-send every write-only secret on the next apply.",
				Optional:            true,
			},
			"delete_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Hard-delete on destroy (default `true`). `false` archives the credential and purges its secret instead. Imported resources start with `false`, so removing one from configuration cannot destroy it until you opt in.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"static_bearer": schema.SingleNestedAttribute{
				MarkdownDescription: "A fixed bearer token presented to one MCP server.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"mcp_server_url": schema.StringAttribute{MarkdownDescription: "MCP server URL the token is sent to. Must use `https://`. Immutable.", Required: true,
						Validators:    []validator.String{httpsURL()},
						PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
					"token": writeOnlySecret("Bearer token (1 to 8192 characters).", true),
				},
			},
			"environment_variable": schema.SingleNestedAttribute{
				MarkdownDescription: "A secret exposed to the agent's sandbox as an environment variable and substituted at network egress.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"secret_name": schema.StringAttribute{MarkdownDescription: "Environment variable name (1 to 255). Immutable and unique within the vault.", Required: true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
					"secret_value":    writeOnlySecret("Secret value (1 to 4096 characters).", true),
					"networking_type": schema.StringAttribute{MarkdownDescription: "`unrestricted` or `limited` (then `allowed_hosts` applies).", Required: true, Validators: []validator.String{stringvalidator.OneOf("unrestricted", "limited")}},
					"allowed_hosts":   schema.ListAttribute{MarkdownDescription: "Hosts the secret may be sent to when `networking_type` is `limited` (hostnames, IPv4 or `*.wildcards`, at most 16).", ElementType: types.StringType, Optional: true},
					"inject_header":   schema.BoolAttribute{MarkdownDescription: "Allow substitution in request headers. Defaults to `true`.", Optional: true, Computed: true},
					"inject_body":     schema.BoolAttribute{MarkdownDescription: "Allow substitution in request bodies. Defaults to `true`.", Optional: true, Computed: true},
				},
			},
			"mcp_oauth": schema.SingleNestedAttribute{
				MarkdownDescription: "An OAuth access token (optionally refreshable) for one MCP server.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"mcp_server_url": schema.StringAttribute{MarkdownDescription: "MCP server URL the access token is sent to. Must use `https://`. Immutable.", Required: true,
						Validators:    []validator.String{httpsURL()},
						PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
					"access_token": writeOnlySecret("OAuth access token.", true),
					"expires_at":   schema.StringAttribute{MarkdownDescription: "Access token expiry (RFC 3339).", Optional: true},
					"refresh": schema.SingleNestedAttribute{
						MarkdownDescription: "Refresh configuration; when set the platform refreshes the access token itself.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"token_endpoint": schema.StringAttribute{MarkdownDescription: "OAuth token endpoint the refresh token is sent to. Must use `https://`. Immutable.", Required: true,
								Validators:    []validator.String{httpsURL()},
								PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
							"client_id": schema.StringAttribute{MarkdownDescription: "OAuth client id. Immutable.", Required: true,
								PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
							"refresh_token": writeOnlySecret("OAuth refresh token.", true),
							"scope":         schema.StringAttribute{MarkdownDescription: "Scope requested on refresh.", Optional: true},
							"resource":      schema.StringAttribute{MarkdownDescription: "Resource indicator sent on refresh.", Optional: true},
							"token_endpoint_auth_type": schema.StringAttribute{MarkdownDescription: "`none`, `client_secret_basic` or `client_secret_post`. Defaults to `none`.",
								Optional: true, Computed: true, Validators: []validator.String{stringvalidator.OneOf("none", "client_secret_basic", "client_secret_post")}},
							"client_secret": writeOnlySecret("Client secret for the `client_secret_*` auth types.", false),
						},
					},
				},
			},
			"created_at":  schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":  schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
			"archived_at": schema.StringAttribute{MarkdownDescription: "Archive timestamp; null while live.", Computed: true},
		},
	}
}

func (r *vaultCredentialResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{resourcevalidator.ExactlyOneOf(path.MatchRoot("static_bearer"), path.MatchRoot("environment_variable"), path.MatchRoot("mcp_oauth"))}
}

func (r *vaultCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAPIKey, &resp.Diagnostics)
}

// buildAuth assembles the auth union from the configuration (secrets come from
// req.Config because write-only values are absent from plan and state).
func (r *vaultCredentialResource) buildAuth(ctx context.Context, cfg vaultCredentialModel, diags *diag.Diagnostics) json.RawMessage {
	auth := map[string]any{}
	switch {
	case !cfg.StaticBearer.IsNull():
		var b staticBearerModel
		diags.Append(cfg.StaticBearer.As(ctx, &b, basetypesObjectAsOptions)...)
		auth["type"] = "static_bearer"
		auth["mcp_server_url"] = b.MCPServerURL.ValueString()
		if !b.Token.IsNull() {
			auth["token"] = b.Token.ValueString()
		}
	case !cfg.EnvironmentVariable.IsNull():
		var e envVarModel
		diags.Append(cfg.EnvironmentVariable.As(ctx, &e, basetypesObjectAsOptions)...)
		auth["type"] = "environment_variable"
		auth["secret_name"] = e.SecretName.ValueString()
		if !e.SecretValue.IsNull() {
			auth["secret_value"] = e.SecretValue.ValueString()
		}
		net := map[string]any{"type": e.NetworkingType.ValueString()}
		if !e.AllowedHosts.IsNull() && !e.AllowedHosts.IsUnknown() {
			net["allowed_hosts"] = listStrings(diags, e.AllowedHosts)
		}
		auth["networking"] = net
		if !e.InjectHeader.IsUnknown() && !e.InjectBody.IsUnknown() && (!e.InjectHeader.IsNull() || !e.InjectBody.IsNull()) {
			h, b := true, true
			if !e.InjectHeader.IsNull() {
				h = e.InjectHeader.ValueBool()
			}
			if !e.InjectBody.IsNull() {
				b = e.InjectBody.ValueBool()
			}
			auth["injection_location"] = map[string]any{"header": h, "body": b}
		}
	case !cfg.MCPOAuth.IsNull():
		var o mcpOAuthModel
		diags.Append(cfg.MCPOAuth.As(ctx, &o, basetypesObjectAsOptions)...)
		auth["type"] = "mcp_oauth"
		auth["mcp_server_url"] = o.MCPServerURL.ValueString()
		if !o.AccessToken.IsNull() {
			auth["access_token"] = o.AccessToken.ValueString()
		}
		if !o.ExpiresAt.IsNull() {
			auth["expires_at"] = o.ExpiresAt.ValueString()
		}
		if !o.Refresh.IsNull() {
			var rf oauthRefreshModel
			diags.Append(o.Refresh.As(ctx, &rf, basetypesObjectAsOptions)...)
			ref := map[string]any{"token_endpoint": rf.TokenEndpoint.ValueString(), "client_id": rf.ClientID.ValueString()}
			if !rf.RefreshToken.IsNull() {
				ref["refresh_token"] = rf.RefreshToken.ValueString()
			}
			if !rf.Scope.IsNull() {
				ref["scope"] = rf.Scope.ValueString()
			}
			if !rf.Resource.IsNull() {
				ref["resource"] = rf.Resource.ValueString()
			}
			authType := "none"
			if !rf.TokenEndpointAuthType.IsNull() && !rf.TokenEndpointAuthType.IsUnknown() {
				authType = rf.TokenEndpointAuthType.ValueString()
			}
			tea := map[string]any{"type": authType}
			if authType != "none" && !rf.ClientSecret.IsNull() {
				tea["client_secret"] = rf.ClientSecret.ValueString()
			}
			ref["token_endpoint_auth"] = tea
			auth["refresh"] = ref
		}
	}
	b, _ := json.Marshal(auth)
	return b
}

// flatten maps the sanitized API object onto the model; write-only secrets stay null.
func (r *vaultCredentialResource) flatten(ctx context.Context, c *client.VaultCredential, m *vaultCredentialModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(c.ID)
	m.VaultID = types.StringValue(c.VaultID)
	m.DisplayName = stringFromPtr(c.DisplayName)
	m.Metadata = metadataToMap(ctx, c.Metadata, m.Metadata, diags)
	m.CreatedAt = types.StringValue(c.CreatedAt)
	m.UpdatedAt = types.StringValue(c.UpdatedAt)
	m.ArchivedAt = stringFromPtr(c.ArchivedAt)
	if m.DeleteOnDestroy.IsNull() || m.DeleteOnDestroy.IsUnknown() {
		m.DeleteOnDestroy = types.BoolValue(true)
	}
	var auth struct {
		Type         string  `json:"type"`
		MCPServerURL string  `json:"mcp_server_url"`
		SecretName   string  `json:"secret_name"`
		ExpiresAt    *string `json:"expires_at"`
		Networking   *struct {
			Type         string   `json:"type"`
			AllowedHosts []string `json:"allowed_hosts"`
		} `json:"networking"`
		InjectionLocation *struct {
			Header bool `json:"header"`
			Body   bool `json:"body"`
		} `json:"injection_location"`
		Refresh *struct {
			TokenEndpoint     string  `json:"token_endpoint"`
			ClientID          string  `json:"client_id"`
			Scope             *string `json:"scope"`
			Resource          *string `json:"resource"`
			TokenEndpointAuth *struct {
				Type string `json:"type"`
			} `json:"token_endpoint_auth"`
		} `json:"refresh"`
	}
	if err := json.Unmarshal(c.Auth, &auth); err != nil {
		diags.AddError("Unexpected credential auth payload", err.Error())
		return
	}
	m.StaticBearer = types.ObjectNull(staticBearerAttrTypes)
	m.EnvironmentVariable = types.ObjectNull(envVarAttrTypes)
	m.MCPOAuth = types.ObjectNull(mcpOAuthAttrTypes)
	switch auth.Type {
	case "static_bearer":
		obj, d := types.ObjectValue(staticBearerAttrTypes, map[string]attr.Value{"mcp_server_url": types.StringValue(auth.MCPServerURL), "token": types.StringNull()})
		diags.Append(d...)
		m.StaticBearer = obj
	case "environment_variable":
		hosts := types.ListNull(types.StringType)
		netType := "unrestricted"
		if auth.Networking != nil {
			netType = auth.Networking.Type
			if auth.Networking.AllowedHosts != nil {
				l, d := types.ListValueFrom(ctx, types.StringType, auth.Networking.AllowedHosts)
				diags.Append(d...)
				hosts = l
			}
		}
		h, b := true, true
		if auth.InjectionLocation != nil {
			h, b = auth.InjectionLocation.Header, auth.InjectionLocation.Body
		}
		obj, d := types.ObjectValue(envVarAttrTypes, map[string]attr.Value{"secret_name": types.StringValue(auth.SecretName), "secret_value": types.StringNull(),
			"networking_type": types.StringValue(netType), "allowed_hosts": hosts, "inject_header": types.BoolValue(h), "inject_body": types.BoolValue(b)})
		diags.Append(d...)
		m.EnvironmentVariable = obj
	case "mcp_oauth":
		refresh := types.ObjectNull(oauthRefreshAttrTypes)
		if auth.Refresh != nil {
			authType := "none"
			if auth.Refresh.TokenEndpointAuth != nil && auth.Refresh.TokenEndpointAuth.Type != "" {
				authType = auth.Refresh.TokenEndpointAuth.Type
			}
			obj, d := types.ObjectValue(oauthRefreshAttrTypes, map[string]attr.Value{"token_endpoint": types.StringValue(auth.Refresh.TokenEndpoint), "client_id": types.StringValue(auth.Refresh.ClientID),
				"refresh_token": types.StringNull(), "scope": stringFromPtr(auth.Refresh.Scope), "resource": stringFromPtr(auth.Refresh.Resource),
				"token_endpoint_auth_type": types.StringValue(authType), "client_secret": types.StringNull()})
			diags.Append(d...)
			refresh = obj
		}
		obj, d := types.ObjectValue(mcpOAuthAttrTypes, map[string]attr.Value{"mcp_server_url": types.StringValue(auth.MCPServerURL), "access_token": types.StringNull(),
			"expires_at": stringFromPtr(auth.ExpiresAt), "refresh": refresh})
		diags.Append(d...)
		m.MCPOAuth = obj
	default:
		diags.AddError("Unexpected credential auth type", fmt.Sprintf("API returned auth.type %q", auth.Type))
	}
}

func (r *vaultCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg vaultCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.VaultCredentialCreate{DisplayName: stringPtr(plan.DisplayName), Metadata: metadataFromMap(ctx, plan.Metadata, &resp.Diagnostics), Auth: r.buildAuth(ctx, cfg, &resp.Diagnostics)}
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.client.CreateVaultCredential(ctx, plan.VaultID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating vault credential", err)
		return
	}
	r.flatten(ctx, c, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vaultCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vaultCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.client.GetVaultCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading vault credential", err)
		return
	}
	if c.ArchivedAt != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	r.flatten(ctx, c, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vaultCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, cfg vaultCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.VaultCredentialUpdate{Metadata: metadataPatch(ctx, plan.Metadata, state.Metadata, &resp.Diagnostics)}
	if !plan.DisplayName.Equal(state.DisplayName) {
		if plan.DisplayName.IsNull() {
			in.DisplayName = client.Null[string]()
		} else {
			in.DisplayName = client.Some(plan.DisplayName.ValueString())
		}
	}
	// Any change to the auth block or the rotation trigger re-sends the secrets.
	if !plan.SecretVersion.Equal(state.SecretVersion) || !plan.StaticBearer.Equal(state.StaticBearer) ||
		!plan.EnvironmentVariable.Equal(state.EnvironmentVariable) || !plan.MCPOAuth.Equal(state.MCPOAuth) {
		in.Auth = r.buildAuth(ctx, cfg, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.client.UpdateVaultCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating vault credential", err)
		return
	}
	r.flatten(ctx, c, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vaultCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vaultCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var err error
	if state.DeleteOnDestroy.ValueBool() {
		err = r.client.DeleteVaultCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString())
	} else {
		_, err = r.client.ArchiveVaultCredential(ctx, state.VaultID.ValueString(), state.ID.ValueString())
	}
	if err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error deleting vault credential", err)
	}
}

func (r *vaultCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, "vault_id/credential_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vault_id"), types.StringValue(parts[0]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(parts[1]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("delete_on_destroy"), types.BoolValue(false))...)
}
