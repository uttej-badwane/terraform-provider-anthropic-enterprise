package provider

import (
	"context"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewFederationRuleResource) }

var (
	_ resource.Resource                   = &federationRuleResource{}
	_ resource.ResourceWithConfigure      = &federationRuleResource{}
	_ resource.ResourceWithImportState    = &federationRuleResource{}
	_ resource.ResourceWithValidateConfig = &federationRuleResource{}
)

// NewFederationRuleResource returns the anthropic_federation_rule resource.
func NewFederationRuleResource() resource.Resource { return &federationRuleResource{} }

type federationRuleResource struct {
	client *client.Client
}

type federationRuleModel struct {
	ID                     types.String `tfsdk:"id"`
	IssuerID               types.String `tfsdk:"issuer_id"`
	Name                   types.String `tfsdk:"name"`
	OAuthScope             types.String `tfsdk:"oauth_scope"`
	ServiceAccountID       types.String `tfsdk:"service_account_id"`
	Match                  types.Object `tfsdk:"match"`
	Description            types.String `tfsdk:"description"`
	TokenLifetimeSeconds   types.Int64  `tfsdk:"token_lifetime_seconds"`
	AppliesToAllWorkspaces types.Bool   `tfsdk:"applies_to_all_workspaces"`
	WorkspaceID            types.String `tfsdk:"workspace_id"`
	ArchiveOnDestroy       types.Bool   `tfsdk:"archive_on_destroy"`
	WorkspaceIDs           types.List   `tfsdk:"workspace_ids"`
	IssuerName             types.String `tfsdk:"issuer_name"`
	ServiceAccountName     types.String `tfsdk:"service_account_name"`
	CreatedAt              types.String `tfsdk:"created_at"`
	UpdatedAt              types.String `tfsdk:"updated_at"`
	ArchivedAt             types.String `tfsdk:"archived_at"`
}

type ruleMatchModel struct {
	SubjectPrefix types.String `tfsdk:"subject_prefix"`
	Claims        types.Map    `tfsdk:"claims"`
	Condition     types.String `tfsdk:"condition"`
	Audience      types.String `tfsdk:"audience"`
}

var attrTypesRuleMatch = map[string]attr.Type{
	"subject_prefix": types.StringType,
	"claims":         types.MapType{ElemType: types.StringType},
	"condition":      types.StringType,
	"audience":       types.StringType,
}

func (r *federationRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federation_rule"
}

func (r *federationRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Maps tokens from a federation issuer to a service account, producing short-lived credentials " +
			"scoped to one or more workspaces.\n\n" +
			"Requires `oauth_token` (an `org:admin` OAuth token). OAuth callers can only create rules with `oauth_scope` " +
			"`workspace:developer` or `workspace:inference`; `org:admin` rules must be created in the Console.\n\n" +
			"~> The Admin API cannot delete rules. `terraform destroy` **archives** the rule, which is irreversible. " +
			"Set `archive_on_destroy = false` to only remove it from state. Bind additional workspaces with " +
			"`anthropic_federation_rule_workspace`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Rule id (`fdrl_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"issuer_id": schema.StringAttribute{
				MarkdownDescription: "Federation issuer id (`fdis_...`). Changing it forces a new rule.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique slug: lowercase letters, digits and hyphens.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(slugRegexp, slugMessage)},
			},
			"oauth_scope": schema.StringAttribute{
				MarkdownDescription: "Scope of the minted token: `workspace:developer` or `workspace:inference`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("workspace:developer", "workspace:inference")},
			},
			"service_account_id": schema.StringAttribute{
				MarkdownDescription: "Service account (`svac_...`) the rule mints tokens for.",
				Required:            true,
			},
			"match": schema.SingleNestedAttribute{
				MarkdownDescription: "Which tokens the rule accepts. At least one of `subject_prefix`, `claims` or `condition` is required; `audience` alone is not sufficient.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"subject_prefix": schema.StringAttribute{
						MarkdownDescription: "Exact `sub` match, or a prefix when it ends with `*`.",
						Optional:            true,
					},
					"claims": schema.MapAttribute{
						MarkdownDescription: "Claims that must match exactly.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"condition": schema.StringAttribute{
						MarkdownDescription: "CEL expression over `claims`. Constant-true expressions are rejected by the API.",
						Optional:            true,
					},
					"audience": schema.StringAttribute{
						MarkdownDescription: "Exact `aud` match.",
						Optional:            true,
					},
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-text description. Removing it clears the description.",
				Optional:            true,
			},
			"token_lifetime_seconds": schema.Int64Attribute{
				MarkdownDescription: "Lifetime of minted tokens in seconds (60 to 86400). Defaults to `3600`.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(3600),
				Validators:          []validator.Int64{int64validator.Between(60, 86400)},
			},
			"applies_to_all_workspaces": schema.BoolAttribute{
				MarkdownDescription: "Enable the rule for every workspace. Defaults to `false`, in which case `workspace_id` is required.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"workspace_id": schema.StringAttribute{
				MarkdownDescription: "Workspace (`wrkspc_...`) the rule is enabled for. Required unless `applies_to_all_workspaces` is `true`.",
				Optional:            true,
			},
			"archive_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Archive the rule when the resource is destroyed. Defaults to `true`. When `false`, destroy only removes the resource from state.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"workspace_ids": schema.ListAttribute{
				MarkdownDescription: "All workspaces the rule is enabled for, including bindings added with `anthropic_federation_rule_workspace`.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"issuer_name": schema.StringAttribute{
				MarkdownDescription: "Name of the issuer.",
				Computed:            true,
			},
			"service_account_name": schema.StringAttribute{
				MarkdownDescription: "Name of the target service account.",
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
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "Archive timestamp; null while live.",
				Computed:            true,
			},
		},
	}
}

func (r *federationRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg federationRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !cfg.Match.IsNull() && !cfg.Match.IsUnknown() {
		var m ruleMatchModel
		resp.Diagnostics.Append(cfg.Match.As(ctx, &m, basetypesObjectAsOptions)...)
		known := func(v attr.Value) bool { return v.IsUnknown() }
		if !known(m.SubjectPrefix) && !known(m.Claims) && !known(m.Condition) &&
			m.SubjectPrefix.IsNull() && (m.Claims.IsNull() || len(m.Claims.Elements()) == 0) && m.Condition.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("match"), "Incomplete match",
				"At least one of match.subject_prefix, match.claims or match.condition must be set.")
		}
	}
	if cfg.AppliesToAllWorkspaces.IsUnknown() || cfg.WorkspaceID.IsUnknown() {
		return
	}
	all := !cfg.AppliesToAllWorkspaces.IsNull() && cfg.AppliesToAllWorkspaces.ValueBool()
	if all && !cfg.WorkspaceID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("workspace_id"), "Conflicting workspace binding",
			"workspace_id cannot be set when applies_to_all_workspaces is true.")
	}
	if !all && cfg.WorkspaceID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("workspace_id"), "Missing workspace binding",
			"workspace_id is required unless applies_to_all_workspaces is true.")
	}
}

func (r *federationRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredOAuth, &resp.Diagnostics)
}

func (r *federationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan federationRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	match := expandRuleMatch(ctx, plan.Match, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.FederationRuleCreate{
		IssuerID:               plan.IssuerID.ValueString(),
		Match:                  match,
		Name:                   plan.Name.ValueString(),
		OAuthScope:             plan.OAuthScope.ValueString(),
		Target:                 client.RuleTarget{Type: "service_account", ServiceAccountID: plan.ServiceAccountID.ValueString()},
		AppliesToAllWorkspaces: boolPtr(plan.AppliesToAllWorkspaces),
		Description:            stringPtr(plan.Description),
		TokenLifetimeSeconds:   int64Ptr(plan.TokenLifetimeSeconds),
		WorkspaceID:            stringPtr(plan.WorkspaceID),
	}
	rule, err := r.client.CreateFederationRule(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating federation rule", err)
		return
	}
	tflog.Trace(ctx, "created federation rule", map[string]any{"id": rule.ID})
	rule = r.rereadRule(ctx, rule)
	state := plan
	resp.Diagnostics.Append(flattenFederationRule(ctx, rule, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *federationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state federationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.GetFederationRule(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading federation rule", err)
		return
	}
	if rule.ArchivedAt != nil {
		tflog.Info(ctx, "federation rule archived out of band; removing from state", map[string]any{"id": rule.ID})
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(flattenFederationRule(ctx, rule, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *federationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state federationRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.FederationRuleUpdate{}
	if !plan.Name.Equal(state.Name) {
		in.Name = stringPtr(plan.Name)
	}
	if !plan.OAuthScope.Equal(state.OAuthScope) {
		in.OAuthScope = stringPtr(plan.OAuthScope)
	}
	if !plan.ServiceAccountID.Equal(state.ServiceAccountID) {
		in.Target = &client.RuleTarget{Type: "service_account", ServiceAccountID: plan.ServiceAccountID.ValueString()}
	}
	if !plan.Match.Equal(state.Match) {
		m := expandRuleMatch(ctx, plan.Match, &resp.Diagnostics)
		in.Match = &m
	}
	if !plan.Description.Equal(state.Description) {
		if plan.Description.IsNull() {
			in.Description = client.Null[string]()
		} else {
			in.Description = client.Some(plan.Description.ValueString())
		}
	}
	if !plan.TokenLifetimeSeconds.IsUnknown() && !plan.TokenLifetimeSeconds.Equal(state.TokenLifetimeSeconds) {
		in.TokenLifetimeSeconds = int64Ptr(plan.TokenLifetimeSeconds)
	}
	if !plan.AppliesToAllWorkspaces.IsUnknown() && !plan.AppliesToAllWorkspaces.Equal(state.AppliesToAllWorkspaces) {
		in.AppliesToAllWorkspaces = boolPtr(plan.AppliesToAllWorkspaces)
	}
	if !plan.WorkspaceID.IsNull() && !plan.WorkspaceID.Equal(state.WorkspaceID) {
		in.WorkspaceID = stringPtr(plan.WorkspaceID)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.UpdateFederationRule(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating federation rule", err)
		return
	}
	rule = r.rereadRule(ctx, rule)
	newState := plan
	resp.Diagnostics.Append(flattenFederationRule(ctx, rule, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *federationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state federationRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.ArchiveOnDestroy.ValueBool() {
		tflog.Info(ctx, "archive_on_destroy is false; leaving federation rule in place", map[string]any{"id": state.ID.ValueString()})
		return
	}
	if _, err := r.client.ArchiveFederationRule(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error archiving federation rule", err)
	}
}

func (r *federationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("archive_on_destroy"), types.BoolValue(true))...)
}

func expandRuleMatch(ctx context.Context, obj types.Object, diags *diag.Diagnostics) client.RuleMatch {
	var m ruleMatchModel
	diags.Append(obj.As(ctx, &m, basetypesObjectAsOptions)...)
	out := client.RuleMatch{
		SubjectPrefix: stringPtr(m.SubjectPrefix),
		Condition:     stringPtr(m.Condition),
		Audience:      stringPtr(m.Audience),
	}
	if !m.Claims.IsNull() && !m.Claims.IsUnknown() {
		claims := map[string]string{}
		diags.Append(m.Claims.ElementsAs(ctx, &claims, false)...)
		out.Claims = claims
	}
	return out
}

func flattenRuleMatch(ctx context.Context, m client.RuleMatch) (types.Object, diag.Diagnostics) {
	claims := types.MapNull(types.StringType)
	var diags diag.Diagnostics
	if len(m.Claims) > 0 {
		v, d := types.MapValueFrom(ctx, types.StringType, m.Claims)
		diags.Append(d...)
		claims = v
	}
	obj, d := types.ObjectValue(attrTypesRuleMatch, map[string]attr.Value{
		"subject_prefix": stringFromPtr(m.SubjectPrefix),
		"claims":         claims,
		"condition":      stringFromPtr(m.Condition),
		"audience":       stringFromPtr(m.Audience),
	})
	diags.Append(d...)
	return obj, diags
}

func flattenFederationRule(ctx context.Context, rule *client.FederationRule, m *federationRuleModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(rule.ID)
	m.IssuerID = types.StringValue(rule.IssuerID)
	m.Name = types.StringValue(rule.Name)
	m.OAuthScope = types.StringValue(rule.OAuthScope)
	m.ServiceAccountID = types.StringValue(rule.Target.ServiceAccountID)
	m.TokenLifetimeSeconds = types.Int64Value(rule.TokenLifetimeSeconds)
	m.AppliesToAllWorkspaces = types.BoolValue(rule.AppliesToAllWorkspaces)
	m.IssuerName = stringFromPtr(rule.IssuerName)
	m.ServiceAccountName = stringFromPtr(rule.Target.ServiceAccountName)
	m.CreatedAt = types.StringValue(rule.CreatedAt)
	m.UpdatedAt = types.StringValue(rule.UpdatedAt)
	m.ArchivedAt = stringFromPtr(rule.ArchivedAt)
	if m.ArchiveOnDestroy.IsNull() || m.ArchiveOnDestroy.IsUnknown() {
		m.ArchiveOnDestroy = types.BoolValue(true)
	}
	if rule.Description == nil || (*rule.Description == "" && (m.Description.IsNull() || m.Description.IsUnknown())) {
		m.Description = types.StringNull()
	} else {
		m.Description = types.StringValue(*rule.Description)
	}

	ids := rule.WorkspaceIDs
	if ids == nil {
		ids = []string{}
	}
	l, d := types.ListValueFrom(ctx, types.StringType, ids)
	diags.Append(d...)
	m.WorkspaceIDs = l

	// Multi-workspace rules report workspace_id null; keep the configured
	// binding as long as it is still among the enabled workspaces.
	switch {
	case rule.WorkspaceID != nil:
		m.WorkspaceID = types.StringValue(*rule.WorkspaceID)
	case !m.WorkspaceID.IsNull() && !m.WorkspaceID.IsUnknown() && slices.Contains(ids, m.WorkspaceID.ValueString()):
		// keep
	default:
		m.WorkspaceID = types.StringNull()
	}

	obj, d := flattenRuleMatch(ctx, rule.Match)
	diags.Append(d...)
	m.Match = obj
	return diags
}

// rereadRule fetches the rule again after a write: the create and update
// responses omit the read-time fields issuer_name and service_account_name,
// which GET populates. Falls back to the write response on error.
func (r *federationRuleResource) rereadRule(ctx context.Context, written *client.FederationRule) *client.FederationRule {
	got, err := r.client.GetFederationRule(ctx, written.ID)
	if err != nil {
		return written
	}
	return got
}
