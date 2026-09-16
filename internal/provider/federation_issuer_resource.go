package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewFederationIssuerResource) }

var (
	_ resource.Resource                   = &federationIssuerResource{}
	_ resource.ResourceWithConfigure      = &federationIssuerResource{}
	_ resource.ResourceWithImportState    = &federationIssuerResource{}
	_ resource.ResourceWithValidateConfig = &federationIssuerResource{}
)

// NewFederationIssuerResource returns the anthropic_federation_issuer resource.
func NewFederationIssuerResource() resource.Resource { return &federationIssuerResource{} }

type federationIssuerResource struct {
	client *client.Client
}

type federationIssuerModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	IssuerURL             types.String `tfsdk:"issuer_url"`
	CheckJTI              types.Bool   `tfsdk:"check_jti"`
	MaxJWTLifetimeSeconds types.Int64  `tfsdk:"max_jwt_lifetime_seconds"`
	JWKS                  types.Object `tfsdk:"jwks"`
	ArchiveOnDestroy      types.Bool   `tfsdk:"archive_on_destroy"`
	CreatedAt             types.String `tfsdk:"created_at"`
	UpdatedAt             types.String `tfsdk:"updated_at"`
	ArchivedAt            types.String `tfsdk:"archived_at"`
	JWKSPollingDisabledAt types.String `tfsdk:"jwks_polling_disabled_at"`
}

type jwksModel struct {
	Type          types.String `tfsdk:"type"`
	CACertPEM     types.String `tfsdk:"ca_cert_pem"`
	DiscoveryBase types.String `tfsdk:"discovery_base"`
	URL           types.String `tfsdk:"url"`
	Keys          types.List   `tfsdk:"keys"`
}

var attrTypesJWKS = map[string]attr.Type{
	"type":           types.StringType,
	"ca_cert_pem":    types.StringType,
	"discovery_base": types.StringType,
	"url":            types.StringType,
	"keys":           types.ListType{ElemType: types.StringType},
}

func (r *federationIssuerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_issuer"
}

func (r *federationIssuerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Registers an OpenID Connect token issuer for workload identity federation. Federation rules " +
			"(`anthropic_federation_rule`) reference an issuer to map its tokens to a service account.\n\n" +
			"Requires `oauth_token` (an `org:admin` OAuth token).\n\n" +
			"~> The Admin API cannot delete issuers. `terraform destroy` **archives** the issuer, which is irreversible and " +
			"fails while a live federation rule references it. Set `archive_on_destroy = false` to only remove it from state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Issuer id (`fdis_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique slug: lowercase letters, digits and hyphens.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(slugRegexp, slugMessage)},
			},
			"issuer_url": schema.StringAttribute{
				MarkdownDescription: "Exact `iss` claim value tokens must carry, for example `https://token.actions.githubusercontent.com`. Must use `https://`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(httpsRegexp, "must start with https://")},
			},
			"check_jti": schema.BoolAttribute{
				MarkdownDescription: "Reject replayed tokens by tracking the `jti` claim. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"max_jwt_lifetime_seconds": schema.Int64Attribute{
				MarkdownDescription: "Maximum accepted token lifetime in seconds (1 to 176400). Defaults to `3600`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(3600),
				Validators:          []validator.Int64{int64validator.Between(1, 176400)},
			},
			"jwks": schema.SingleNestedAttribute{
				MarkdownDescription: "Where the issuer's signing keys come from. Defaults to `{ type = \"discovery\" }` (OIDC discovery from `issuer_url`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "`discovery`, `explicit_url` or `inline`. Required when `jwks` is set.",
						Optional:            true,
						Computed:            true,
						Validators:          []validator.String{stringvalidator.OneOf("discovery", "explicit_url", "inline")},
					},
					"ca_cert_pem": schema.StringAttribute{
						MarkdownDescription: "PEM CA certificate used to verify the JWKS endpoint (`discovery` and `explicit_url`).",
						Optional:            true,
						Computed:            true,
					},
					"discovery_base": schema.StringAttribute{
						MarkdownDescription: "Alternative base URL for OIDC discovery (`discovery` only).",
						Optional:            true,
						Computed:            true,
					},
					"url": schema.StringAttribute{
						MarkdownDescription: "JWKS endpoint URL. Required when `type` is `explicit_url`.",
						Optional:            true,
						Computed:            true,
					},
					"keys": schema.ListAttribute{
						MarkdownDescription: "JWK objects as JSON strings, one per key. Required when `type` is `inline`.",
						Optional:            true,
						Computed:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"archive_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Archive the issuer when the resource is destroyed. Defaults to `true`. When `false`, destroy only removes the resource from state.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
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
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "Archive timestamp; null while live.",
				Computed:            true,
			},
			"jwks_polling_disabled_at": schema.StringAttribute{
				MarkdownDescription: "Set when the JWKS poller has been paused after repeated failures; null otherwise.",
				Computed:            true,
			},
		},
	}
}

func (r *federationIssuerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg federationIssuerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.JWKS.IsNull() || cfg.JWKS.IsUnknown() {
		return
	}
	var j jwksModel
	resp.Diagnostics.Append(cfg.JWKS.As(ctx, &j, basetypesObjectAsOptions)...)
	if resp.Diagnostics.HasError() || j.Type.IsUnknown() {
		return
	}
	if j.Type.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("jwks").AtName("type"), "Missing jwks.type", "jwks.type is required when jwks is set.")
		return
	}
	switch j.Type.ValueString() {
	case "explicit_url":
		if j.URL.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("jwks").AtName("url"), "Missing jwks.url", "jwks.url is required when jwks.type is \"explicit_url\".")
		}
	case "inline":
		if j.Keys.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("jwks").AtName("keys"), "Missing jwks.keys", "jwks.keys is required when jwks.type is \"inline\".")
		}
	}
}

func (r *federationIssuerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredOAuth, &resp.Diagnostics)
}

func (r *federationIssuerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan federationIssuerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.FederationIssuerCreate{
		IssuerURL:             plan.IssuerURL.ValueString(),
		Name:                  plan.Name.ValueString(),
		CheckJTI:              boolPtr(plan.CheckJTI),
		MaxJWTLifetimeSeconds: int64Ptr(plan.MaxJWTLifetimeSeconds),
	}
	in.JWKS = expandJWKS(ctx, plan.JWKS, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	is, err := r.client.CreateFederationIssuer(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating federation issuer", err)
		return
	}
	tflog.Trace(ctx, "created federation issuer", map[string]any{"id": is.ID})
	state := plan
	resp.Diagnostics.Append(flattenFederationIssuer(is, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *federationIssuerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state federationIssuerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	is, err := r.client.GetFederationIssuer(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading federation issuer", err)
		return
	}
	if is.ArchivedAt != nil {
		tflog.Info(ctx, "federation issuer archived out of band; removing from state", map[string]any{"id": is.ID})
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(flattenFederationIssuer(is, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *federationIssuerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state federationIssuerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.FederationIssuerUpdate{}
	if !plan.Name.Equal(state.Name) {
		in.Name = stringPtr(plan.Name)
	}
	if !plan.IssuerURL.Equal(state.IssuerURL) {
		in.IssuerURL = stringPtr(plan.IssuerURL)
	}
	if !plan.CheckJTI.IsUnknown() && !plan.CheckJTI.Equal(state.CheckJTI) {
		in.CheckJTI = boolPtr(plan.CheckJTI)
	}
	if !plan.MaxJWTLifetimeSeconds.IsUnknown() && !plan.MaxJWTLifetimeSeconds.Equal(state.MaxJWTLifetimeSeconds) {
		in.MaxJWTLifetimeSeconds = int64Ptr(plan.MaxJWTLifetimeSeconds)
	}
	if !plan.JWKS.IsUnknown() && !plan.JWKS.IsNull() && !plan.JWKS.Equal(state.JWKS) {
		in.JWKS = expandJWKS(ctx, plan.JWKS, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	is, err := r.client.UpdateFederationIssuer(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating federation issuer", err)
		return
	}
	newState := plan
	resp.Diagnostics.Append(flattenFederationIssuer(is, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *federationIssuerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state federationIssuerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.ArchiveOnDestroy.ValueBool() {
		tflog.Info(ctx, "archive_on_destroy is false; leaving federation issuer in place", map[string]any{"id": state.ID.ValueString()})
		return
	}
	if _, err := r.client.ArchiveFederationIssuer(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error archiving federation issuer", err)
	}
}

func (r *federationIssuerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("archive_on_destroy"), types.BoolValue(true))...)
}

// expandJWKS converts the jwks object into the API union; nil when unset.
func expandJWKS(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.JWKS {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	var j jwksModel
	diags.Append(obj.As(ctx, &j, basetypesObjectAsOptions)...)
	if diags.HasError() {
		return nil
	}
	out := &client.JWKS{
		Type:          j.Type.ValueString(),
		CACertPEM:     stringPtr(j.CACertPEM),
		DiscoveryBase: stringPtr(j.DiscoveryBase),
		URL:           stringPtr(j.URL),
	}
	if !j.Keys.IsNull() && !j.Keys.IsUnknown() {
		var raw []string
		diags.Append(j.Keys.ElementsAs(ctx, &raw, false)...)
		for i, s := range raw {
			var m map[string]any
			if err := json.Unmarshal([]byte(s), &m); err != nil {
				diags.AddAttributeError(path.Root("jwks").AtName("keys").AtListIndex(i), "Invalid JWK", fmt.Sprintf("element %d is not a JSON object: %s", i, err))
				continue
			}
			out.Keys = append(out.Keys, m)
		}
	}
	return out
}

func flattenJWKS(j client.JWKS) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	keys := types.ListNull(types.StringType)
	if j.Keys != nil {
		vals := make([]attr.Value, 0, len(j.Keys))
		for _, k := range j.Keys {
			b, err := json.Marshal(k)
			if err != nil {
				diags.AddError("Error encoding JWK", err.Error())
				continue
			}
			vals = append(vals, types.StringValue(string(b)))
		}
		l, d := types.ListValue(types.StringType, vals)
		diags.Append(d...)
		keys = l
	}
	obj, d := types.ObjectValue(attrTypesJWKS, map[string]attr.Value{
		"type":           types.StringValue(j.Type),
		"ca_cert_pem":    stringFromPtr(j.CACertPEM),
		"discovery_base": stringFromPtr(j.DiscoveryBase),
		"url":            stringFromPtr(j.URL),
		"keys":           keys,
	})
	diags.Append(d...)
	return obj, diags
}

func flattenFederationIssuer(is *client.FederationIssuer, m *federationIssuerModel) diag.Diagnostics {
	m.ID = types.StringValue(is.ID)
	m.Name = types.StringValue(is.Name)
	m.IssuerURL = types.StringValue(is.IssuerURL)
	m.CheckJTI = types.BoolValue(is.CheckJTI)
	m.MaxJWTLifetimeSeconds = types.Int64Value(is.MaxJWTLifetimeSeconds)
	m.CreatedAt = types.StringValue(is.CreatedAt)
	m.UpdatedAt = types.StringValue(is.UpdatedAt)
	m.ArchivedAt = stringFromPtr(is.ArchivedAt)
	m.JWKSPollingDisabledAt = stringFromPtr(is.JWKSPollingDisabledAt)
	if m.ArchiveOnDestroy.IsNull() || m.ArchiveOnDestroy.IsUnknown() {
		m.ArchiveOnDestroy = types.BoolValue(true)
	}
	obj, diags := flattenJWKS(is.JWKS)
	m.JWKS = obj
	return diags
}
