package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewDeploymentRunDataSource)
	registerDataSource(NewDeploymentRunsDataSource)
}

var attrTypesDeploymentRun = map[string]attr.Type{
	"id":                   types.StringType,
	"deployment_id":        types.StringType,
	"session_id":           types.StringType,
	"created_at":           types.StringType,
	"agent_id":             types.StringType,
	"agent_version":        types.Int64Type,
	"trigger_type":         types.StringType,
	"trigger_scheduled_at": types.StringType,
	"error_json":           types.StringType,
}

func deploymentRunAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id":            dsString("Run id (`drun_...`)."),
		"deployment_id": dsString("Deployment that fired (`depl_...`)."),
		"session_id": dsString("Session the run created (`sesn_...`); null when the run failed before a session existed. " +
			"A run has either this or `error_json`."),
		"created_at":    dsString("When the run was created (RFC 3339)."),
		"agent_id":      dsString("Agent the run executed."),
		"agent_version": dsInt64("Agent version the run was pinned to."),
		"trigger_type":  dsString("What caused the run, for example `schedule`."),
		"trigger_scheduled_at": dsString("Scheduled firing time for a `schedule` trigger (RFC 3339); " +
			"null for other trigger types. This is the slot the run belongs to, which can differ from `created_at`."),
		"error_json": dsString("The run's error, verbatim as JSON; null when the run did not fail. " +
			"It is exposed raw because its populated shape is not documented; decode it with `jsondecode`."),
	}
}

func deploymentRunRow(r *client.DeploymentRun) map[string]attr.Value {
	errJSON := types.StringNull()
	// A JSON null decodes to the four bytes "null" rather than an empty slice,
	// so an absent error has to be recognised explicitly.
	if len(r.Error) > 0 && string(r.Error) != "null" {
		errJSON = types.StringValue(string(r.Error))
	}
	return map[string]attr.Value{
		"id":                   types.StringValue(r.ID),
		"deployment_id":        types.StringValue(r.DeploymentID),
		"session_id":           stringFromPtr(r.SessionID),
		"created_at":           types.StringValue(r.CreatedAt),
		"agent_id":             types.StringValue(r.Agent.ID),
		"agent_version":        types.Int64Value(r.Agent.Version),
		"trigger_type":         types.StringValue(r.TriggerContext.Type),
		"trigger_scheduled_at": stringFromPtr(r.TriggerContext.ScheduledAt),
		"error_json":           errJSON,
	}
}

// --- anthropic_deployment_run ------------------------------------------------

var _ datasource.DataSourceWithConfigure = &deploymentRunDataSource{}

// NewDeploymentRunDataSource returns the anthropic_deployment_run data source.
func NewDeploymentRunDataSource() datasource.DataSource { return &deploymentRunDataSource{} }

type deploymentRunDataSource struct{ client *client.Client }

func (d *deploymentRunDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_run"
}

func (d *deploymentRunDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := deploymentRunAttrs()
	attrs["id"] = schema.StringAttribute{MarkdownDescription: "Run id (`drun_...`).", Required: true}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads one deployment run." + managedAgentsNote, Attributes: attrs}
}

func (d *deploymentRunDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *deploymentRunDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRootID, &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	run, err := d.client.GetDeploymentRun(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading deployment run", err)
		return
	}
	obj, diags := types.ObjectValue(attrTypesDeploymentRun, deploymentRunRow(run))
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}

// --- anthropic_deployment_runs -----------------------------------------------

var _ datasource.DataSourceWithConfigure = &deploymentRunsDataSource{}

// NewDeploymentRunsDataSource returns the anthropic_deployment_runs data source.
func NewDeploymentRunsDataSource() datasource.DataSource { return &deploymentRunsDataSource{} }

type deploymentRunsDataSource struct{ client *client.Client }

type deploymentRunsDSModel struct {
	DeploymentID types.String `tfsdk:"deployment_id"`
	TriggerType  types.String `tfsdk:"trigger_type"`
	HasError     types.Bool   `tfsdk:"has_error"`
	CreatedAtGte types.String `tfsdk:"created_at_gte"`
	CreatedAtLte types.String `tfsdk:"created_at_lte"`
	Runs         types.List   `tfsdk:"runs"`
}

func (d *deploymentRunsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_runs"
}

func (d *deploymentRunsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists deployment runs, newest first. A run is one firing of a deployment's schedule." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"deployment_id": schema.StringAttribute{
				MarkdownDescription: "Restrict to one deployment. Omit to list every deployment in the workspace. " +
					"A well-formed id that matches no deployment returns an empty list; a malformed id is rejected with " +
					"`Invalid deployment ID`.",
				Optional: true,
			},
			"trigger_type": schema.StringAttribute{
				MarkdownDescription: "Restrict to runs with this trigger, for example `schedule`.",
				Optional:            true,
			},
			"has_error": schema.BoolAttribute{
				MarkdownDescription: "`true` returns only failed runs, `false` only runs that created a session. Omit for both.",
				Optional:            true,
			},
			"created_at_gte": schema.StringAttribute{MarkdownDescription: "Only runs created at or after this RFC 3339 time.", Optional: true},
			"created_at_lte": schema.StringAttribute{MarkdownDescription: "Only runs created at or before this RFC 3339 time.", Optional: true},
			"runs": schema.ListNestedAttribute{
				MarkdownDescription: "Runs, newest first.",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: deploymentRunAttrs()},
			},
		},
	}
}

func (d *deploymentRunsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *deploymentRunsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg deploymentRunsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opts := client.DeploymentRunListOptions{
		DeploymentID: cfg.DeploymentID.ValueString(),
		TriggerType:  cfg.TriggerType.ValueString(),
		CreatedAtGte: cfg.CreatedAtGte.ValueString(),
		CreatedAtLte: cfg.CreatedAtLte.ValueString(),
	}
	// has_error is tri-state: unset means both, so only forward a set value.
	if !cfg.HasError.IsNull() && !cfg.HasError.IsUnknown() {
		v := cfg.HasError.ValueBool()
		opts.HasError = &v
	}
	list, err := d.client.ListDeploymentRuns(ctx, opts)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing deployment runs", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for i := range list {
		rows = append(rows, deploymentRunRow(&list[i]))
	}
	cfg.Runs = objectList(&resp.Diagnostics, attrTypesDeploymentRun, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
