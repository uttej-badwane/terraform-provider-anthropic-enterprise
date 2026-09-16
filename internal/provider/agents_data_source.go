package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewAgentDataSource)
	registerDataSource(NewAgentsDataSource)
	registerDataSource(NewAgentVersionsDataSource)
}

var attrTypesAgentSummary = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "model": types.StringType, "version": types.Int64Type,
	"archived_at": types.StringType, "created_at": types.StringType, "updated_at": types.StringType,
}

func agentSummaryRow(a *client.Agent) map[string]attr.Value {
	return map[string]attr.Value{
		"id": types.StringValue(a.ID), "name": types.StringValue(a.Name), "model": types.StringValue(a.Model.ID), "version": types.Int64Value(a.Version),
		"archived_at": stringFromPtr(a.ArchivedAt), "created_at": types.StringValue(a.CreatedAt), "updated_at": types.StringValue(a.UpdatedAt),
	}
}

func agentSummaryAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": dsString("Agent id."), "name": dsString("Name."), "model": dsString("Model id."), "version": dsInt64("Current version."),
		"archived_at": dsString("Archive timestamp; null while live."), "created_at": dsString("Creation timestamp."), "updated_at": dsString("Last update timestamp."),
	}
}

// --- anthropic_agent ----------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &agentDataSource{}

// NewAgentDataSource returns the anthropic_agent data source.
func NewAgentDataSource() datasource.DataSource { return &agentDataSource{} }

type agentDataSource struct{ client *client.Client }

type agentDSModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	System            types.String `tfsdk:"system"`
	Model             types.String `tfsdk:"model"`
	ModelEffort       types.String `tfsdk:"model_effort"`
	ModelSpeed        types.String `tfsdk:"model_speed"`
	ModelInferenceGeo types.String `tfsdk:"model_inference_geo"`
	ToolsJSON         types.String `tfsdk:"tools_json"`
	MCPServers        types.List   `tfsdk:"mcp_servers"`
	Skills            types.List   `tfsdk:"skills"`
	MultiagentJSON    types.String `tfsdk:"multiagent_json"`
	Metadata          types.Map    `tfsdk:"metadata"`
	Version           types.Int64  `tfsdk:"version"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
	ArchivedAt        types.String `tfsdk:"archived_at"`
}

var attrTypesDSSkill = map[string]attr.Type{"type": types.StringType, "skill_id": types.StringType, "version": types.StringType}

func (d *agentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (d *agentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the current version of a Managed Agents agent." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{MarkdownDescription: "Agent id (`agent_...`).", Required: true},
			"name":                dsString("Name."),
			"description":         dsString("Description."),
			"system":              dsString("System prompt."),
			"model":               dsString("Model id."),
			"model_effort":        dsString("Reasoning effort."),
			"model_speed":         dsString("Inference speed."),
			"model_inference_geo": dsString("Inference geography pin; null when inherited."),
			"tools_json":          dsString("Resolved tools configuration as canonical JSON."),
			"mcp_servers": schema.ListNestedAttribute{MarkdownDescription: "MCP servers.", Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"name": dsString("Server name."), "url": dsString("Server URL.")}}},
			"skills": schema.ListNestedAttribute{MarkdownDescription: "Skills with their resolved versions.", Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{"type": dsString("`anthropic` or `custom`."), "skill_id": dsString("Skill id."), "version": dsString("Resolved version.")}}},
			"multiagent_json": dsString("Resolved coordinator topology as canonical JSON; null when not a coordinator."),
			"metadata":        schema.MapAttribute{MarkdownDescription: "Metadata.", Computed: true, ElementType: types.StringType},
			"version":         dsInt64("Current version."),
			"created_at":      dsString("Creation timestamp."),
			"updated_at":      dsString("Last update timestamp."),
			"archived_at":     dsString("Archive timestamp; null while live."),
		},
	}
}

func (d *agentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *agentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg agentDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := d.client.GetAgent(ctx, cfg.ID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading agent", err)
		return
	}
	resp.Diagnostics.Append(flattenAgentDS(ctx, a, &cfg)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

func canonicalOrNull(raw []byte, diags *diag.Diagnostics) types.String {
	s := string(raw)
	if len(raw) == 0 || s == "null" {
		return types.StringNull()
	}
	c, err := canonicalJSON(raw)
	if err != nil {
		diags.AddError("Error canonicalizing JSON", err.Error())
		return types.StringNull()
	}
	return types.StringValue(c)
}

func flattenAgentDS(ctx context.Context, a *client.Agent, m *agentDSModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(a.ID)
	m.Name = types.StringValue(a.Name)
	m.Description = stringFromPtr(a.Description)
	m.System = stringFromPtr(a.System)
	m.Model = types.StringValue(a.Model.ID)
	m.ModelEffort = types.StringNull()
	if a.Model.Effort != nil {
		m.ModelEffort = types.StringValue(a.Model.Effort.Type)
	}
	m.ModelSpeed = stringFromPtr(a.Model.Speed)
	m.ModelInferenceGeo = stringFromPtr(a.Model.InferenceGeo)
	m.ToolsJSON = canonicalOrNull(a.Tools, &diags)
	m.MultiagentJSON = canonicalOrNull(a.Multiagent, &diags)
	servers := make([]map[string]attr.Value, 0, len(a.MCPServers))
	for _, s := range a.MCPServers {
		servers = append(servers, map[string]attr.Value{"name": types.StringValue(s.Name), "url": types.StringValue(s.URL)})
	}
	m.MCPServers = objectList(&diags, attrTypesMCPServer, servers)
	skills := make([]map[string]attr.Value, 0, len(a.Skills))
	for _, s := range a.Skills {
		skills = append(skills, map[string]attr.Value{"type": types.StringValue(s.Type), "skill_id": types.StringValue(s.SkillID), "version": stringFromPtr(s.Version)})
	}
	m.Skills = objectList(&diags, attrTypesDSSkill, skills)
	mv, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(a.Metadata))
	diags.Append(d...)
	m.Metadata = mv
	m.Version = types.Int64Value(a.Version)
	m.CreatedAt = types.StringValue(a.CreatedAt)
	m.UpdatedAt = types.StringValue(a.UpdatedAt)
	m.ArchivedAt = stringFromPtr(a.ArchivedAt)
	return diags
}

// --- anthropic_agents ---------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &agentsDataSource{}

// NewAgentsDataSource returns the anthropic_agents data source.
func NewAgentsDataSource() datasource.DataSource { return &agentsDataSource{} }

type agentsDataSource struct{ client *client.Client }

type agentsDSModel struct {
	IncludeArchived types.Bool `tfsdk:"include_archived"`
	Agents          types.List `tfsdk:"agents"`
}

func (d *agentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agents"
}

func (d *agentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Managed Agents agents in the workspace." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived agents. Defaults to `false`.", Optional: true},
			"agents":           schema.ListNestedAttribute{MarkdownDescription: "Agents.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: agentSummaryAttrs()}},
		},
	}
}

func (d *agentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *agentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg agentsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListAgents(ctx, cfg.IncludeArchived.ValueBool())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing agents", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for i := range list {
		rows = append(rows, agentSummaryRow(&list[i]))
	}
	cfg.Agents = objectList(&resp.Diagnostics, attrTypesAgentSummary, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_agent_versions ----------------------------------------------------

var _ datasource.DataSourceWithConfigure = &agentVersionsDataSource{}

// NewAgentVersionsDataSource returns the anthropic_agent_versions data source.
func NewAgentVersionsDataSource() datasource.DataSource { return &agentVersionsDataSource{} }

type agentVersionsDataSource struct{ client *client.Client }

type agentVersionsDSModel struct {
	AgentID  types.String `tfsdk:"agent_id"`
	Versions types.List   `tfsdk:"versions"`
}

var attrTypesAgentVersion = map[string]attr.Type{"version": types.Int64Type, "name": types.StringType, "model": types.StringType, "updated_at": types.StringType}

func (d *agentVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_versions"
}

func (d *agentVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the immutable version history of an agent, newest first." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"agent_id": schema.StringAttribute{MarkdownDescription: "Agent id.", Required: true},
			"versions": schema.ListNestedAttribute{MarkdownDescription: "Versions.", Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"version": dsInt64("Version number."), "name": dsString("Name at that version."), "model": dsString("Model id at that version."), "updated_at": dsString("When the version was created."),
				}}},
		},
	}
}

func (d *agentVersionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *agentVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg agentVersionsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListAgentVersions(ctx, cfg.AgentID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing agent versions", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for _, v := range list {
		rows = append(rows, map[string]attr.Value{"version": types.Int64Value(v.Version), "name": types.StringValue(v.Name), "model": types.StringValue(v.Model.ID), "updated_at": types.StringValue(v.UpdatedAt)})
	}
	cfg.Versions = objectList(&resp.Diagnostics, attrTypesAgentVersion, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
