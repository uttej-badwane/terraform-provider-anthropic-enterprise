package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewDeploymentDataSource)
	registerDataSource(NewDeploymentsDataSource)
}

// --- anthropic_deployment -------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &deploymentDataSource{}

// NewDeploymentDataSource returns the anthropic_deployment data source.
func NewDeploymentDataSource() datasource.DataSource { return &deploymentDataSource{} }

type deploymentDataSource struct{ client *client.Client }

type deploymentDSModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	AgentID          types.String `tfsdk:"agent_id"`
	AgentVersion     types.Int64  `tfsdk:"agent_version"`
	EnvironmentID    types.String `tfsdk:"environment_id"`
	ScheduleCron     types.String `tfsdk:"schedule_cron_expression"`
	ScheduleTimezone types.String `tfsdk:"schedule_timezone"`
	VaultIDs         types.List   `tfsdk:"vault_ids"`
	BudgetCents      types.String `tfsdk:"budget_max_list_cost_cents"`
	Description      types.String `tfsdk:"description"`
	Metadata         types.Map    `tfsdk:"metadata"`
	Status           types.String `tfsdk:"status"`
	PausedReasonType types.String `tfsdk:"paused_reason_type"`
	LastRunAt        types.String `tfsdk:"last_run_at"`
	UpcomingRunsAt   types.List   `tfsdk:"upcoming_runs_at"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
	ArchivedAt       types.String `tfsdk:"archived_at"`
}

func (d *deploymentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

func (d *deploymentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads one Managed Agents deployment. Mounted-resource secrets are never returned." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"id":                         schema.StringAttribute{MarkdownDescription: "Deployment id (`depl_...`).", Required: true},
			"name":                       dsString("Name."),
			"agent_id":                   dsString("Agent id."),
			"agent_version":              dsInt64("Pinned agent version."),
			"environment_id":             dsString("Environment id."),
			"schedule_cron_expression":   dsString("Cron expression; null for manual-only deployments."),
			"schedule_timezone":          dsString("Schedule timezone."),
			"vault_ids":                  dsStringList("Vault ids."),
			"budget_max_list_cost_cents": dsString("Per-run budget in USD cents; null when uncapped."),
			"description":                dsString("Description."),
			"metadata":                   schema.MapAttribute{MarkdownDescription: "Metadata.", Computed: true, ElementType: types.StringType},
			"status":                     dsString("`active` or `paused`."),
			"paused_reason_type":         dsString("`manual` or `error` when paused."),
			"last_run_at":                dsString("Timestamp of the last run."),
			"upcoming_runs_at":           dsStringList("Next scheduled runs."),
			"created_at":                 dsString("Creation timestamp."),
			"updated_at":                 dsString("Last update timestamp."),
			"archived_at":                dsString("Archive timestamp; null while live."),
		},
	}
}

func (d *deploymentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func flattenDeploymentDS(ctx context.Context, dep *client.Deployment, diags *diag.Diagnostics) deploymentDSModel {
	m := deploymentDSModel{
		ID: types.StringValue(dep.ID), Name: types.StringValue(dep.Name), AgentID: types.StringValue(dep.Agent.ID), AgentVersion: int64FromPtr(dep.Agent.Version),
		EnvironmentID: types.StringValue(dep.EnvironmentID), Description: stringFromPtr(dep.Description), Status: types.StringValue(dep.Status),
		LastRunAt: stringFromPtr(dep.LastRunAt), CreatedAt: types.StringValue(dep.CreatedAt), UpdatedAt: types.StringValue(dep.UpdatedAt), ArchivedAt: stringFromPtr(dep.ArchivedAt),
		ScheduleCron: types.StringNull(), ScheduleTimezone: types.StringNull(), BudgetCents: types.StringNull(), PausedReasonType: types.StringNull(),
	}
	if dep.Schedule != nil {
		m.ScheduleCron, m.ScheduleTimezone = types.StringValue(dep.Schedule.Expression), types.StringValue(dep.Schedule.Timezone)
	}
	if dep.Budget != nil {
		m.BudgetCents = types.StringValue(dep.Budget.MaxListCost.Amount)
	}
	if dep.PausedReason != nil {
		m.PausedReasonType = types.StringValue(dep.PausedReason.Type)
	}
	var dg diag.Diagnostics
	m.VaultIDs, dg = types.ListValueFrom(ctx, types.StringType, nonNilStrings(dep.VaultIDs))
	diags.Append(dg...)
	m.UpcomingRunsAt, dg = types.ListValueFrom(ctx, types.StringType, nonNilStrings(dep.UpcomingRuns))
	diags.Append(dg...)
	m.Metadata, dg = types.MapValueFrom(ctx, types.StringType, nonNilMap(dep.Metadata))
	diags.Append(dg...)
	return m
}

func (d *deploymentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dep, err := d.client.GetDeployment(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading deployment", err)
		return
	}
	m := flattenDeploymentDS(ctx, dep, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// --- anthropic_deployments ------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &deploymentsDataSource{}

// NewDeploymentsDataSource returns the anthropic_deployments data source.
func NewDeploymentsDataSource() datasource.DataSource { return &deploymentsDataSource{} }

type deploymentsDataSource struct{ client *client.Client }

type deploymentsModel struct {
	AgentID         types.String `tfsdk:"agent_id"`
	Status          types.String `tfsdk:"status"`
	IncludeArchived types.Bool   `tfsdk:"include_archived"`
	Deployments     types.List   `tfsdk:"deployments"`
}

var deploymentSummaryAttrTypes = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "agent_id": types.StringType, "agent_version": types.Int64Type,
	"environment_id": types.StringType, "status": types.StringType, "archived_at": types.StringType,
}

func (d *deploymentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployments"
}

func (d *deploymentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Managed Agents deployments." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"agent_id":         schema.StringAttribute{MarkdownDescription: "Only deployments of this agent.", Optional: true},
			"status":           schema.StringAttribute{MarkdownDescription: "Only `active` or `paused` deployments.", Optional: true, Validators: []validator.String{stringvalidator.OneOf("active", "paused")}},
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived deployments. Defaults to `false`.", Optional: true},
			"deployments": schema.ListNestedAttribute{MarkdownDescription: "Deployments.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"id": dsString("Deployment id."), "name": dsString("Name."), "agent_id": dsString("Agent id."), "agent_version": dsInt64("Pinned agent version."),
				"environment_id": dsString("Environment id."), "status": dsString("`active` or `paused`."), "archived_at": dsString("Archive timestamp."),
			}}},
		},
	}
}

func (d *deploymentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *deploymentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg deploymentsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListDeployments(ctx, client.DeploymentListOptions{AgentID: cfg.AgentID.ValueString(), Status: cfg.Status.ValueString(), IncludeArchived: cfg.IncludeArchived.ValueBool()})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing deployments", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for _, dep := range list {
		rows = append(rows, map[string]attr.Value{
			"id": types.StringValue(dep.ID), "name": types.StringValue(dep.Name), "agent_id": types.StringValue(dep.Agent.ID), "agent_version": int64FromPtr(dep.Agent.Version),
			"environment_id": types.StringValue(dep.EnvironmentID), "status": types.StringValue(dep.Status), "archived_at": stringFromPtr(dep.ArchivedAt),
		})
	}
	cfg.Deployments = objectList(&resp.Diagnostics, deploymentSummaryAttrTypes, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
