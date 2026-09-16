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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewEnvironmentResource) }

var (
	_ resource.Resource                = &environmentResource{}
	_ resource.ResourceWithConfigure   = &environmentResource{}
	_ resource.ResourceWithImportState = &environmentResource{}
)

// NewEnvironmentResource returns the anthropic_environment resource.
func NewEnvironmentResource() resource.Resource { return &environmentResource{} }

type environmentResource struct{ client *client.Client }

type environmentModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Type            types.String `tfsdk:"type"`
	Networking      types.Object `tfsdk:"networking"`
	Packages        types.Object `tfsdk:"packages"`
	Scope           types.String `tfsdk:"scope"`
	Metadata        types.Map    `tfsdk:"metadata"`
	DeleteOnDestroy types.Bool   `tfsdk:"delete_on_destroy"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	ArchivedAt      types.String `tfsdk:"archived_at"`
}

type networkingModel struct {
	Type                 types.String `tfsdk:"type"`
	AllowedHosts         types.List   `tfsdk:"allowed_hosts"`
	AllowMCPServers      types.Bool   `tfsdk:"allow_mcp_servers"`
	AllowPackageManagers types.Bool   `tfsdk:"allow_package_managers"`
}

type packagesModel struct {
	Apt   types.List `tfsdk:"apt"`
	Cargo types.List `tfsdk:"cargo"`
	Gem   types.List `tfsdk:"gem"`
	Go    types.List `tfsdk:"go"`
	Npm   types.List `tfsdk:"npm"`
	Pip   types.List `tfsdk:"pip"`
}

var attrTypesNetworking = map[string]attr.Type{
	"type": types.StringType, "allowed_hosts": types.ListType{ElemType: types.StringType},
	"allow_mcp_servers": types.BoolType, "allow_package_managers": types.BoolType,
}

var packageEcosystems = []string{"apt", "cargo", "gem", "go", "npm", "pip"}

var attrTypesPackages = func() map[string]attr.Type {
	m := map[string]attr.Type{}
	for _, k := range packageEcosystems {
		m[k] = types.ListType{ElemType: types.StringType}
	}
	return m
}()

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *environmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	listAttr := func(desc string) schema.ListAttribute {
		return schema.ListAttribute{MarkdownDescription: desc, Optional: true, Computed: true, ElementType: types.StringType}
	}
	packageAttrs := map[string]schema.Attribute{}
	for _, k := range packageEcosystems {
		packageAttrs[k] = listAttr("Packages installed with `" + k + "`.")
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Managed Agents environment: the container configuration (network policy and " +
			"pre-installed packages) that sessions and deployments run in. Environment names are unique within a workspace. " +
			"Configuration changes affect new sessions only.\n\n" +
			"`terraform destroy` deletes the environment (the API refuses while sessions reference it). Set " +
			"`delete_on_destroy = false` to archive it instead." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Environment id (`env_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique name, 1 to 256 characters.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 256)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description, up to 1024 characters.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.LengthAtMost(1024)},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "`cloud` (Anthropic-hosted container) or `self_hosted`. Changing it forces a new environment.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("cloud", "self_hosted")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"networking": schema.SingleNestedAttribute{
				MarkdownDescription: "Network policy for `cloud` environments. Defaults to `{ type = \"unrestricted\" }`. Null for `self_hosted`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						MarkdownDescription: "`unrestricted` or `limited`.",
						Optional:            true,
						Computed:            true,
						Validators:          []validator.String{stringvalidator.OneOf("unrestricted", "limited")},
					},
					"allowed_hosts":          listAttr("Hosts reachable under `limited` networking (bare host or `*.example.com`)."),
					"allow_mcp_servers":      schema.BoolAttribute{MarkdownDescription: "Allow traffic to the agent's MCP servers under `limited` networking.", Optional: true, Computed: true},
					"allow_package_managers": schema.BoolAttribute{MarkdownDescription: "Allow package-manager traffic under `limited` networking. Required when `packages` is set.", Optional: true, Computed: true},
				},
			},
			"packages": schema.SingleNestedAttribute{
				MarkdownDescription: "Packages pre-installed in `cloud` environments, per ecosystem. Under `limited` networking, `allow_package_managers` must be `true`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes:          packageAttrs,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "`organization` (default) or `account`. Changing it forces a new environment.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.OneOf("organization", "account")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key/value metadata, max 16 pairs.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"delete_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Delete the environment on destroy (default `true`). When `false`, it is archived instead.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"created_at":  schema.StringAttribute{MarkdownDescription: "Creation timestamp (RFC 3339).", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":  schema.StringAttribute{MarkdownDescription: "Last update timestamp (RFC 3339).", Computed: true},
			"archived_at": schema.StringAttribute{MarkdownDescription: "Archive timestamp; null while live.", Computed: true},
		},
	}
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAPIKey, &resp.Diagnostics)
}

func expandEnvConfig(ctx context.Context, m environmentModel, diags *diag.Diagnostics) client.EnvironmentConfig {
	cfg := client.EnvironmentConfig{Type: m.Type.ValueString()}
	if cfg.Type != "cloud" {
		return cfg
	}
	if !m.Networking.IsNull() && !m.Networking.IsUnknown() {
		var n networkingModel
		diags.Append(m.Networking.As(ctx, &n, basetypesObjectAsOptions)...)
		net := &client.Networking{Type: n.Type.ValueString(), AllowMCPServers: boolPtr(n.AllowMCPServers), AllowPackageManagers: boolPtr(n.AllowPackageManagers)}
		if !n.AllowedHosts.IsNull() && !n.AllowedHosts.IsUnknown() {
			var hosts []string
			diags.Append(n.AllowedHosts.ElementsAs(ctx, &hosts, false)...)
			net.AllowedHosts = hosts
			if hosts == nil {
				net.AllowedHosts = []string{}
			}
		}
		cfg.Networking = net
	}
	if !m.Packages.IsNull() && !m.Packages.IsUnknown() {
		var p packagesModel
		diags.Append(m.Packages.As(ctx, &p, basetypesObjectAsOptions)...)
		pk := &client.Packages{}
		for k, dst := range map[string]*[]string{"apt": &pk.Apt, "cargo": &pk.Cargo, "gem": &pk.Gem, "go": &pk.Go, "npm": &pk.Npm, "pip": &pk.Pip} {
			l := packageList(p, k)
			if l.IsNull() || l.IsUnknown() {
				continue
			}
			var vals []string
			diags.Append(l.ElementsAs(ctx, &vals, false)...)
			if vals == nil {
				vals = []string{}
			}
			*dst = vals
		}
		cfg.Packages = pk
	}
	return cfg
}

func packageList(p packagesModel, k string) types.List {
	switch k {
	case "apt":
		return p.Apt
	case "cargo":
		return p.Cargo
	case "gem":
		return p.Gem
	case "go":
		return p.Go
	case "npm":
		return p.Npm
	default:
		return p.Pip
	}
}

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.EnvironmentCreate{
		Name:        plan.Name.ValueString(),
		Description: stringPtr(plan.Description),
		Config:      expandEnvConfig(ctx, plan, &resp.Diagnostics),
		Metadata:    expandMetadata(ctx, plan.Metadata, &resp.Diagnostics),
		Scope:       stringPtr(plan.Scope),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.client.CreateEnvironment(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating environment", err)
		return
	}
	state := plan
	resp.Diagnostics.Append(flattenEnvironment(ctx, e, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.client.GetEnvironment(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading environment", err)
		return
	}
	if e.ArchivedAt != nil {
		tflog.Info(ctx, "environment archived out of band; removing from state", map[string]any{"id": e.ID})
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(flattenEnvironment(ctx, e, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state environmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.EnvironmentUpdate{}
	if !plan.Name.Equal(state.Name) {
		in.Name = stringPtr(plan.Name)
	}
	if !plan.Description.Equal(state.Description) {
		in.Description = optFromString(plan.Description)
	}
	if (!plan.Networking.IsUnknown() && !plan.Networking.Equal(state.Networking)) || (!plan.Packages.IsUnknown() && !plan.Packages.Equal(state.Packages)) {
		cfg := expandEnvConfig(ctx, plan, &resp.Diagnostics)
		in.Config = &cfg
	}
	if !plan.Metadata.Equal(state.Metadata) {
		in.Metadata = metadataPatch(ctx, plan.Metadata, state.Metadata, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.client.UpdateEnvironment(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating environment", err)
		return
	}
	newState := plan
	resp.Diagnostics.Append(flattenEnvironment(ctx, e, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if state.DeleteOnDestroy.ValueBool() {
		if err := r.client.DeleteEnvironment(ctx, id); err != nil && !client.IsNotFound(err) {
			apiErrorDiag(&resp.Diagnostics, "Error deleting environment", err)
		}
		return
	}
	if _, err := r.client.ArchiveEnvironment(ctx, id); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error archiving environment", err)
	}
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("delete_on_destroy"), types.BoolValue(true))...)
}

// listOrNull keeps null when the prior value was null and the API returned an empty list.
func listOrNull(ctx context.Context, prior types.List, vals []string, diags *diag.Diagnostics) types.List {
	if len(vals) == 0 && (prior.IsNull() || prior.IsUnknown()) {
		return types.ListNull(types.StringType)
	}
	if vals == nil {
		vals = []string{}
	}
	l, d := types.ListValueFrom(ctx, types.StringType, vals)
	diags.Append(d...)
	return l
}

func flattenEnvironment(ctx context.Context, e *client.Environment, m *environmentModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(e.ID)
	m.Name = types.StringValue(e.Name)
	m.Description = stringFromPtr(e.Description)
	m.Type = types.StringValue(e.Config.Type)
	m.Scope = types.StringValue(e.Scope)
	m.CreatedAt = types.StringValue(e.CreatedAt)
	m.UpdatedAt = types.StringValue(e.UpdatedAt)
	m.ArchivedAt = stringFromPtr(e.ArchivedAt)
	if m.DeleteOnDestroy.IsNull() || m.DeleteOnDestroy.IsUnknown() {
		m.DeleteOnDestroy = types.BoolValue(true)
	}
	if len(e.Metadata) == 0 && (m.Metadata.IsNull() || m.Metadata.IsUnknown()) {
		m.Metadata = types.MapNull(types.StringType)
	} else {
		mv, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(e.Metadata))
		diags.Append(d...)
		m.Metadata = mv
	}

	if e.Config.Networking == nil {
		m.Networking = types.ObjectNull(attrTypesNetworking)
	} else {
		var prior networkingModel
		if !m.Networking.IsNull() && !m.Networking.IsUnknown() {
			diags.Append(m.Networking.As(ctx, &prior, basetypesObjectAsOptions)...)
		}
		n := e.Config.Networking
		obj, d := types.ObjectValue(attrTypesNetworking, map[string]attr.Value{
			"type":                   types.StringValue(n.Type),
			"allowed_hosts":          listOrNull(ctx, prior.AllowedHosts, n.AllowedHosts, &diags),
			"allow_mcp_servers":      boolFromPtrOrNull(n.AllowMCPServers),
			"allow_package_managers": boolFromPtrOrNull(n.AllowPackageManagers),
		})
		diags.Append(d...)
		m.Networking = obj
	}

	if e.Config.Packages == nil {
		m.Packages = types.ObjectNull(attrTypesPackages)
	} else {
		var prior packagesModel
		priorSet := !m.Packages.IsNull() && !m.Packages.IsUnknown()
		if priorSet {
			diags.Append(m.Packages.As(ctx, &prior, basetypesObjectAsOptions)...)
		}
		p := e.Config.Packages
		api := map[string][]string{"apt": p.Apt, "cargo": p.Cargo, "gem": p.Gem, "go": p.Go, "npm": p.Npm, "pip": p.Pip}
		total := 0
		for _, v := range api {
			total += len(v)
		}
		if total == 0 && !priorSet {
			m.Packages = types.ObjectNull(attrTypesPackages)
		} else {
			vals := map[string]attr.Value{}
			for _, k := range packageEcosystems {
				var pl types.List
				if priorSet {
					pl = packageList(prior, k)
				} else {
					pl = types.ListNull(types.StringType)
				}
				vals[k] = listOrNull(ctx, pl, api[k], &diags)
			}
			obj, d := types.ObjectValue(attrTypesPackages, vals)
			diags.Append(d...)
			m.Packages = obj
		}
	}
	return diags
}

func boolFromPtrOrNull(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}
