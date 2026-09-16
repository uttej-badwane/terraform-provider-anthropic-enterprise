package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
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

func init() { registerResource(NewAgentResource) }

var (
	_ resource.Resource                   = &agentResource{}
	_ resource.ResourceWithConfigure      = &agentResource{}
	_ resource.ResourceWithImportState    = &agentResource{}
	_ resource.ResourceWithValidateConfig = &agentResource{}
)

// NewAgentResource returns the anthropic_agent resource.
func NewAgentResource() resource.Resource { return &agentResource{} }

type agentResource struct{ client *client.Client }

type agentModel struct {
	ID                types.String         `tfsdk:"id"`
	Name              types.String         `tfsdk:"name"`
	Model             types.String         `tfsdk:"model"`
	ModelEffort       types.String         `tfsdk:"model_effort"`
	ModelSpeed        types.String         `tfsdk:"model_speed"`
	ModelInferenceGeo types.String         `tfsdk:"model_inference_geo"`
	Description       types.String         `tfsdk:"description"`
	System            types.String         `tfsdk:"system"`
	Tools             jsontypes.Normalized `tfsdk:"tools"`
	MCPServers        types.List           `tfsdk:"mcp_servers"`
	Skills            types.List           `tfsdk:"skills"`
	Multiagent        jsontypes.Normalized `tfsdk:"multiagent"`
	Metadata          types.Map            `tfsdk:"metadata"`
	ArchiveOnDestroy  types.Bool           `tfsdk:"archive_on_destroy"`
	Version           types.Int64          `tfsdk:"version"`
	CreatedAt         types.String         `tfsdk:"created_at"`
	UpdatedAt         types.String         `tfsdk:"updated_at"`
	ArchivedAt        types.String         `tfsdk:"archived_at"`
}

type mcpServerModel struct {
	Name types.String `tfsdk:"name"`
	URL  types.String `tfsdk:"url"`
}

type skillRefModel struct {
	Type            types.String `tfsdk:"type"`
	SkillID         types.String `tfsdk:"skill_id"`
	Version         types.String `tfsdk:"version"`
	ResolvedVersion types.String `tfsdk:"resolved_version"`
}

var attrTypesMCPServer = map[string]attr.Type{"name": types.StringType, "url": types.StringType}
var attrTypesSkillRef = map[string]attr.Type{
	"type": types.StringType, "skill_id": types.StringType, "version": types.StringType, "resolved_version": types.StringType,
}

func (r *agentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (r *agentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Managed Agents agent definition: model, system prompt, tools, MCP servers, skills and " +
			"coordinator roster. Every effective change creates a new immutable `version`; reference it from " +
			"`anthropic_deployment.agent_version` to pin deployments.\n\n" +
			"~> Agents cannot be deleted. `terraform destroy` **archives** the agent (terminal). Set `archive_on_destroy = false` " +
			"to only remove it from state." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Agent id (`agent_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name, 1 to 256 characters. Not unique.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 256)},
			},
			"model": schema.StringAttribute{
				MarkdownDescription: "Model id, for example `claude-opus-5`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"model_effort": schema.StringAttribute{
				MarkdownDescription: "Reasoning effort: `low`, `medium`, `high`, `xhigh` or `max`. The API assigns a per-model default when omitted.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.OneOf("low", "medium", "high", "xhigh", "max")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"model_speed": schema.StringAttribute{
				MarkdownDescription: "`standard` or `fast`. Defaults to `standard`.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.OneOf("standard", "fast")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"model_inference_geo": schema.StringAttribute{
				MarkdownDescription: "Inference geography pin (for example `us`). Omit to inherit the workspace default.",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "What the agent does, up to 2048 characters.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.LengthAtMost(2048)},
			},
			"system": schema.StringAttribute{
				MarkdownDescription: "System prompt, up to 100000 characters.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.LengthAtMost(100000)},
			},
			"tools": schema.StringAttribute{
				MarkdownDescription: "JSON array of toolsets (`agent_toolset_20260401`, `mcp_toolset`, `custom`), as documented for the " +
					"agents API. Use `jsonencode()`. The API fills defaults (`default_config`, per-tool `type`, `enabled`); the provider " +
					"treats the response as matching when every configured key is present with the same value, so server-added " +
					"defaults never produce a diff. Every `mcp_servers` entry must be referenced by an `mcp_toolset`.",
				Optional:   true,
				CustomType: jsontypes.NormalizedType{},
			},
			"mcp_servers": schema.ListNestedAttribute{
				MarkdownDescription: "MCP servers the agent connects to (max 20). Names must be unique and referenced from `tools`.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{MarkdownDescription: "Server name, 1 to 255 characters.", Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 255)}},
					"url":  schema.StringAttribute{MarkdownDescription: "Server endpoint URL.", Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 2048)}},
				}},
			},
			"skills": schema.ListNestedAttribute{
				MarkdownDescription: "Skills available to the agent (max 20).",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"type":     schema.StringAttribute{MarkdownDescription: "`anthropic` or `custom`.", Required: true, Validators: []validator.String{stringvalidator.OneOf("anthropic", "custom")}},
					"skill_id": schema.StringAttribute{MarkdownDescription: "Anthropic skill name (for example `xlsx`) or custom skill id (`skill_...`).", Required: true},
					"version": schema.StringAttribute{
						MarkdownDescription: "Version to pin. Omit or set `latest` to follow the newest version; the concrete value is in `resolved_version`.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"resolved_version": schema.StringAttribute{
						MarkdownDescription: "Concrete version the API resolved at the last apply.",
						Computed:            true,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
				}},
			},
			"multiagent": schema.StringAttribute{
				MarkdownDescription: "JSON coordinator topology `{\"type\":\"coordinator\",\"agents\":[...]}`. Roster entries may be agent ids, " +
					"`{type = \"agent\", id, version}`, `{type = \"self\"}` or `{type = \"advisor\", model}`. Compared like `tools`.",
				Optional:   true,
				CustomType: jsontypes.NormalizedType{},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key/value metadata, max 16 pairs.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"archive_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Archive the agent on destroy. Defaults to `true`. When `false`, destroy only removes the resource from state. Imported resources start with `false`, so removing one from configuration cannot destroy it until you opt in.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"version": schema.Int64Attribute{
				MarkdownDescription: "Current version; increments on every effective update, so it shows as known after apply whenever another attribute changes.",
				Computed:            true,
			},
			"created_at":  schema.StringAttribute{MarkdownDescription: "Creation timestamp (RFC 3339).", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":  schema.StringAttribute{MarkdownDescription: "Last update timestamp (RFC 3339).", Computed: true},
			"archived_at": schema.StringAttribute{MarkdownDescription: "Archive timestamp; null while live.", Computed: true},
		},
	}
}

// ValidateConfig checks that every MCP server is referenced by an mcp_toolset.
func (r *agentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg agentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.MCPServers.IsNull() || cfg.MCPServers.IsUnknown() {
		return
	}
	var servers []mcpServerModel
	resp.Diagnostics.Append(cfg.MCPServers.ElementsAs(ctx, &servers, false)...)
	if len(servers) == 0 {
		return
	}
	if cfg.Tools.IsUnknown() {
		return
	}
	referenced := map[string]bool{}
	if !cfg.Tools.IsNull() {
		var tools []map[string]any
		if err := json.Unmarshal([]byte(cfg.Tools.ValueString()), &tools); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("tools"), "Invalid tools JSON", err.Error())
			return
		}
		for _, t := range tools {
			if t["type"] == "mcp_toolset" {
				if n, ok := t["mcp_server_name"].(string); ok {
					referenced[n] = true
				}
			}
		}
	}
	for i, s := range servers {
		if s.Name.IsUnknown() {
			continue
		}
		if !referenced[s.Name.ValueString()] {
			resp.Diagnostics.AddAttributeError(path.Root("mcp_servers").AtListIndex(i), "Unreferenced MCP server",
				fmt.Sprintf("mcp_servers[%d] (%q) must be referenced by an mcp_toolset entry in tools; the API rejects unreferenced servers.", i, s.Name.ValueString()))
		}
	}
}

func (r *agentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAPIKey, &resp.Diagnostics)
}

func modelParamsFromPlan(m agentModel) client.ModelParams {
	return client.ModelParams{ID: m.Model.ValueString(), Effort: stringPtr(m.ModelEffort), Speed: stringPtr(m.ModelSpeed), InferenceGeo: stringPtr(m.ModelInferenceGeo)}
}

func expandMCPServers(ctx context.Context, l types.List, diags *diag.Diagnostics) []client.MCPServer {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var in []mcpServerModel
	diags.Append(l.ElementsAs(ctx, &in, false)...)
	out := make([]client.MCPServer, 0, len(in))
	for _, s := range in {
		out = append(out, client.MCPServer{Type: "url", Name: s.Name.ValueString(), URL: s.URL.ValueString()})
	}
	return out
}

func expandSkills(ctx context.Context, l types.List, diags *diag.Diagnostics) []client.SkillRef {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var in []skillRefModel
	diags.Append(l.ElementsAs(ctx, &in, false)...)
	out := make([]client.SkillRef, 0, len(in))
	for _, s := range in {
		ref := client.SkillRef{Type: s.Type.ValueString(), SkillID: s.SkillID.ValueString()}
		if !s.Version.IsNull() && !s.Version.IsUnknown() && s.Version.ValueString() != "latest" {
			ref.Version = stringPtr(s.Version)
		}
		out = append(out, ref)
	}
	return out
}

func expandMetadata(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := map[string]string{}
	diags.Append(m.ElementsAs(ctx, &out, false)...)
	return out
}

func rawJSON(v jsontypes.Normalized) json.RawMessage {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return json.RawMessage(v.ValueString())
}

func (r *agentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan agentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.AgentCreate{
		Name:        plan.Name.ValueString(),
		Model:       modelParamsFromPlan(plan),
		Description: stringPtr(plan.Description),
		System:      stringPtr(plan.System),
		Tools:       rawJSON(plan.Tools),
		MCPServers:  expandMCPServers(ctx, plan.MCPServers, &resp.Diagnostics),
		Skills:      expandSkills(ctx, plan.Skills, &resp.Diagnostics),
		Multiagent:  rawJSON(plan.Multiagent),
		Metadata:    expandMetadata(ctx, plan.Metadata, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.client.CreateAgent(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating agent", err)
		return
	}
	tflog.Trace(ctx, "created agent", map[string]any{"id": a.ID})
	state := plan
	resp.Diagnostics.Append(flattenAgent(ctx, a, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *agentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state agentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.client.GetAgent(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading agent", err)
		return
	}
	if a.ArchivedAt != nil {
		tflog.Info(ctx, "agent archived out of band; removing from state", map[string]any{"id": a.ID})
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(flattenAgent(ctx, a, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *agentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state agentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.AgentUpdate{}
	if !plan.Name.Equal(state.Name) {
		in.Name = stringPtr(plan.Name)
	}
	if !plan.Model.Equal(state.Model) || knownDiff(plan.ModelEffort, state.ModelEffort) || knownDiff(plan.ModelSpeed, state.ModelSpeed) || !plan.ModelInferenceGeo.Equal(state.ModelInferenceGeo) {
		mp := modelParamsFromPlan(plan)
		if mp.Effort == nil {
			mp.Effort = stringPtr(state.ModelEffort)
		}
		if mp.Speed == nil {
			mp.Speed = stringPtr(state.ModelSpeed)
		}
		in.Model = &mp
	}
	if !plan.Description.Equal(state.Description) {
		in.Description = optFromString(plan.Description)
	}
	if !plan.System.Equal(state.System) {
		in.System = optFromString(plan.System)
	}
	toolsChanged := !plan.Tools.Equal(state.Tools)
	serversChanged := !plan.MCPServers.Equal(state.MCPServers)
	if toolsChanged || serversChanged {
		in.Tools = rawJSON(plan.Tools)
		if in.Tools == nil {
			in.Tools = json.RawMessage("[]")
		}
		servers := expandMCPServers(ctx, plan.MCPServers, &resp.Diagnostics)
		if servers == nil {
			servers = []client.MCPServer{}
		}
		in.MCPServers = &servers
	}
	if !plan.Skills.Equal(state.Skills) {
		skills := expandSkills(ctx, plan.Skills, &resp.Diagnostics)
		if skills == nil {
			skills = []client.SkillRef{}
		}
		in.Skills = &skills
	}
	if !plan.Multiagent.Equal(state.Multiagent) {
		in.Multiagent = rawJSON(plan.Multiagent)
		if in.Multiagent == nil {
			in.Multiagent = json.RawMessage("null")
		}
	}
	if !plan.Metadata.Equal(state.Metadata) {
		in.Metadata = metadataPatch(ctx, plan.Metadata, state.Metadata, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.client.UpdateAgent(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating agent", err)
		return
	}
	newState := plan
	resp.Diagnostics.Append(flattenAgent(ctx, a, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *agentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state agentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.ArchiveOnDestroy.ValueBool() {
		tflog.Info(ctx, "archive_on_destroy is false; leaving agent in place", map[string]any{"id": state.ID.ValueString()})
		return
	}
	if _, err := r.client.ArchiveAgent(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error archiving agent", err)
	}
}

func (r *agentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("archive_on_destroy"), types.BoolValue(false))...)
}

// knownDiff reports a change only when the plan value is known.
func knownDiff(plan, state types.String) bool {
	return !plan.IsUnknown() && !plan.Equal(state)
}

func optFromString(v types.String) client.Opt[string] {
	if v.IsNull() || v.IsUnknown() {
		return client.Null[string]()
	}
	return client.Some(v.ValueString())
}

// reconcileJSON keeps the configured JSON when the API response is a superset
// of it; otherwise it stores the canonical API value (real drift).
func reconcileJSON(prior jsontypes.Normalized, api json.RawMessage, emptyIsNull string, diags *diag.Diagnostics) jsontypes.Normalized {
	apiStr := string(api)
	if len(api) == 0 || apiStr == "null" || apiStr == emptyIsNull {
		return jsontypes.NewNormalizedNull()
	}
	if !prior.IsNull() && !prior.IsUnknown() {
		ok, err := jsonSubsetEqual([]byte(prior.ValueString()), api)
		if err != nil {
			diags.AddError("Error comparing JSON", err.Error())
			return prior
		}
		if ok {
			return prior
		}
	}
	canon, err := canonicalJSON(api)
	if err != nil {
		diags.AddError("Error canonicalizing JSON", err.Error())
		return prior
	}
	return jsontypes.NewNormalizedValue(canon)
}

func flattenAgent(ctx context.Context, a *client.Agent, m *agentModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(a.ID)
	m.Name = types.StringValue(a.Name)
	m.Model = types.StringValue(a.Model.ID)
	if a.Model.Effort != nil {
		m.ModelEffort = types.StringValue(a.Model.Effort.Type)
	} else {
		m.ModelEffort = types.StringNull()
	}
	m.ModelSpeed = stringFromPtr(a.Model.Speed)
	m.ModelInferenceGeo = stringFromPtr(a.Model.InferenceGeo)
	m.Description = stringFromPtr(a.Description)
	m.System = stringFromPtr(a.System)
	m.Version = types.Int64Value(a.Version)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	m.ArchivedAt = stringFromPtr(a.ArchivedAt)
	if m.ArchiveOnDestroy.IsNull() || m.ArchiveOnDestroy.IsUnknown() {
		m.ArchiveOnDestroy = types.BoolValue(true)
	}

	m.Tools = reconcileJSON(m.Tools, a.Tools, "[]", &diags)
	m.Multiagent = reconcileJSON(m.Multiagent, a.Multiagent, "", &diags)

	if len(a.MCPServers) == 0 && (m.MCPServers.IsNull() || m.MCPServers.IsUnknown()) {
		m.MCPServers = types.ListNull(types.ObjectType{AttrTypes: attrTypesMCPServer})
	} else {
		rows := make([]attr.Value, 0, len(a.MCPServers))
		for _, s := range a.MCPServers {
			obj, d := types.ObjectValue(attrTypesMCPServer, map[string]attr.Value{"name": types.StringValue(s.Name), "url": types.StringValue(s.URL)})
			diags.Append(d...)
			rows = append(rows, obj)
		}
		l, d := types.ListValue(types.ObjectType{AttrTypes: attrTypesMCPServer}, rows)
		diags.Append(d...)
		m.MCPServers = l
	}

	var prior []skillRefModel
	if !m.Skills.IsNull() && !m.Skills.IsUnknown() {
		diags.Append(m.Skills.ElementsAs(ctx, &prior, false)...)
	}
	if len(a.Skills) == 0 && (m.Skills.IsNull() || m.Skills.IsUnknown()) {
		m.Skills = types.ListNull(types.ObjectType{AttrTypes: attrTypesSkillRef})
	} else {
		rows := make([]attr.Value, 0, len(a.Skills))
		for i, s := range a.Skills {
			version := stringFromPtr(s.Version)
			if i < len(prior) && prior[i].Type.ValueString() == s.Type && prior[i].SkillID.ValueString() == s.SkillID {
				pv := prior[i].Version
				switch {
				case pv.IsNull() || pv.IsUnknown():
					version = types.StringNull() // follow latest; concrete value in resolved_version
				case pv.ValueString() == "latest":
					version = pv
				}
			}
			obj, d := types.ObjectValue(attrTypesSkillRef, map[string]attr.Value{
				"type": types.StringValue(s.Type), "skill_id": types.StringValue(s.SkillID), "version": version, "resolved_version": stringFromPtr(s.Version),
			})
			diags.Append(d...)
			rows = append(rows, obj)
		}
		l, d := types.ListValue(types.ObjectType{AttrTypes: attrTypesSkillRef}, rows)
		diags.Append(d...)
		m.Skills = l
	}

	if len(a.Metadata) == 0 && (m.Metadata.IsNull() || m.Metadata.IsUnknown()) {
		m.Metadata = types.MapNull(types.StringType)
	} else {
		mv, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(a.Metadata))
		diags.Append(d...)
		m.Metadata = mv
	}
	return diags
}
