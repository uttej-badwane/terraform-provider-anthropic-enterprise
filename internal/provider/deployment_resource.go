package provider

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewDeploymentResource) }

var (
	_ resource.Resource                = &deploymentResource{}
	_ resource.ResourceWithConfigure   = &deploymentResource{}
	_ resource.ResourceWithImportState = &deploymentResource{}
)

// NewDeploymentResource returns the anthropic_deployment resource.
func NewDeploymentResource() resource.Resource { return &deploymentResource{} }

type deploymentResource struct{ client *client.Client }

type deploymentModel struct {
	ID                 types.String         `tfsdk:"id"`
	Name               types.String         `tfsdk:"name"`
	AgentID            types.String         `tfsdk:"agent_id"`
	AgentVersion       types.Int64          `tfsdk:"agent_version"`
	EnvironmentID      types.String         `tfsdk:"environment_id"`
	InitialEvents      jsontypes.Normalized `tfsdk:"initial_events"`
	Schedule           types.Object         `tfsdk:"schedule"`
	GitHubRepositories types.List           `tfsdk:"github_repositories"`
	Files              types.List           `tfsdk:"files"`
	MemoryStores       types.List           `tfsdk:"memory_stores"`
	VaultIDs           types.List           `tfsdk:"vault_ids"`
	BudgetCents        types.String         `tfsdk:"budget_max_list_cost_cents"`
	Description        types.String         `tfsdk:"description"`
	Metadata           types.Map            `tfsdk:"metadata"`
	Paused             types.Bool           `tfsdk:"paused"`
	ArchiveOnDestroy   types.Bool           `tfsdk:"archive_on_destroy"`
	Status             types.String         `tfsdk:"status"`
	PausedReasonType   types.String         `tfsdk:"paused_reason_type"`
	PausedReasonError  types.String         `tfsdk:"paused_reason_error"`
	LastRunAt          types.String         `tfsdk:"last_run_at"`
	UpcomingRunsAt     types.List           `tfsdk:"upcoming_runs_at"`
	CreatedAt          types.String         `tfsdk:"created_at"`
	UpdatedAt          types.String         `tfsdk:"updated_at"`
	ArchivedAt         types.String         `tfsdk:"archived_at"`
}

type scheduleModel struct {
	CronExpression types.String `tfsdk:"cron_expression"`
	Timezone       types.String `tfsdk:"timezone"`
}

type githubRepoModel struct {
	URL                types.String `tfsdk:"url"`
	AuthorizationToken types.String `tfsdk:"authorization_token"`
	CheckoutBranch     types.String `tfsdk:"checkout_branch"`
	CheckoutCommit     types.String `tfsdk:"checkout_commit"`
	MountPath          types.String `tfsdk:"mount_path"`
}

type fileResourceModel struct {
	FileID    types.String `tfsdk:"file_id"`
	MountPath types.String `tfsdk:"mount_path"`
}

type memoryStoreRefModel struct {
	MemoryStoreID types.String `tfsdk:"memory_store_id"`
	Access        types.String `tfsdk:"access"`
	Instructions  types.String `tfsdk:"instructions"`
}

var (
	scheduleAttrTypes   = map[string]attr.Type{"cron_expression": types.StringType, "timezone": types.StringType}
	githubRepoAttrTypes = map[string]attr.Type{"url": types.StringType, "authorization_token": types.StringType, "checkout_branch": types.StringType, "checkout_commit": types.StringType, "mount_path": types.StringType}
	fileResAttrTypes    = map[string]attr.Type{"file_id": types.StringType, "mount_path": types.StringType}
	memStoreRefTypes    = map[string]attr.Type{"memory_store_id": types.StringType, "access": types.StringType, "instructions": types.StringType}
)

func (r *deploymentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment"
}

func (r *deploymentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Managed Agents deployment: an agent plus environment, initial events, mounted resources and " +
			"an optional cron schedule that fires sessions autonomously." + managedAgentsNote + "\n\n" +
			"~> Deployments cannot be deleted. `terraform destroy` **archives** the deployment (terminal). Set `archive_on_destroy = false` " +
			"to only remove it from state. The provider always pins an explicit `agent_version`, so the API's re-pin-to-latest behaviour " +
			"never produces drift. A deployment the platform pauses because a referenced object was archived shows `paused = true` with " +
			"`paused_reason_type = \"error\"`, which surfaces as drift until you fix the reference.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{MarkdownDescription: "Deployment id (`depl_...`).", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":     schema.StringAttribute{MarkdownDescription: "Name, 1 to 256 characters.", Required: true, Validators: []validator.String{stringvalidator.LengthBetween(1, 256)}},
			"agent_id": schema.StringAttribute{MarkdownDescription: "Agent to run (`agent_...`).", Required: true},
			"agent_version": schema.Int64Attribute{
				MarkdownDescription: "Agent version to pin. Defaults to the agent's latest version at creation and then stays fixed; " +
					"set it explicitly (for example to `anthropic_agent.x.version`) to roll forward.",
				Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.StringAttribute{MarkdownDescription: "Environment to run in (`env_...`).", Required: true},
			"initial_events": schema.StringAttribute{
				MarkdownDescription: "JSON array of 1 to 50 initial events (`user.message`, `user.define_outcome`, `system.message`). " +
					"Compared semantically; server-added fields are ignored.",
				Required:   true,
				CustomType: jsontypes.NormalizedType{},
			},
			"schedule": schema.SingleNestedAttribute{
				MarkdownDescription: "Cron schedule. Omit for a manual-only deployment.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"cron_expression": schema.StringAttribute{MarkdownDescription: "Five-field POSIX cron expression.", Required: true},
					"timezone":        schema.StringAttribute{MarkdownDescription: "IANA timezone, for example `UTC`.", Required: true},
				},
			},
			"github_repositories": schema.ListNestedAttribute{
				MarkdownDescription: "Git repositories mounted into each session.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"url":                 schema.StringAttribute{MarkdownDescription: "Repository URL.", Required: true},
					"authorization_token": writeOnlySecret("Token used to clone private repositories.", false),
					"checkout_branch":     schema.StringAttribute{MarkdownDescription: "Branch to check out.", Optional: true},
					"checkout_commit":     schema.StringAttribute{MarkdownDescription: "Commit SHA to check out (instead of a branch).", Optional: true},
					"mount_path":          schema.StringAttribute{MarkdownDescription: "Mount path inside the session.", Optional: true},
				}},
			},
			"files": schema.ListNestedAttribute{
				MarkdownDescription: "Files (Files API ids) mounted into each session.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"file_id":    schema.StringAttribute{MarkdownDescription: "File id.", Required: true},
					"mount_path": schema.StringAttribute{MarkdownDescription: "Mount path inside the session.", Optional: true},
				}},
			},
			"memory_stores": schema.ListNestedAttribute{
				MarkdownDescription: "Memory stores mounted into each session.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"memory_store_id": schema.StringAttribute{MarkdownDescription: "Memory store id (`memstore_...`).", Required: true},
					"access":          schema.StringAttribute{MarkdownDescription: "`read_write` (default) or `read_only`.", Optional: true, Computed: true, Validators: []validator.String{stringvalidator.OneOf("read_write", "read_only")}},
					"instructions":    schema.StringAttribute{MarkdownDescription: "Instructions for using the store (max 4096).", Optional: true},
				}},
			},
			"vault_ids":                  schema.ListAttribute{MarkdownDescription: "Vaults whose credentials sessions may use (max 50).", ElementType: types.StringType, Optional: true},
			"budget_max_list_cost_cents": schema.StringAttribute{MarkdownDescription: "Per-run budget cap in USD cents as an integer string. Omit for no cap.", Optional: true, Validators: []validator.String{stringvalidator.RegexMatches(digitsOnlyRegexp, "must be an integer number of cents")}},
			"description":                schema.StringAttribute{MarkdownDescription: "Description (max 2048).", Optional: true},
			"metadata":                   schema.MapAttribute{MarkdownDescription: "Key/value metadata (up to 16 pairs).", ElementType: types.StringType, Optional: true},
			"paused": schema.BoolAttribute{
				MarkdownDescription: "Pause scheduled runs. Defaults to `false`. Also becomes `true` when the platform pauses the deployment after an error.",
				Optional:            true, Computed: true, Default: booldefault.StaticBool(false),
			},
			"archive_on_destroy":  schema.BoolAttribute{MarkdownDescription: "Archive on destroy (default `true`); `false` only removes the resource from state.", Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"status":              schema.StringAttribute{MarkdownDescription: "`active` or `paused`.", Computed: true},
			"paused_reason_type":  schema.StringAttribute{MarkdownDescription: "`manual` or `error` when paused; null otherwise.", Computed: true},
			"paused_reason_error": schema.StringAttribute{MarkdownDescription: "Error type behind an error pause.", Computed: true},
			"last_run_at":         schema.StringAttribute{MarkdownDescription: "Timestamp of the last run.", Computed: true},
			"upcoming_runs_at":    schema.ListAttribute{MarkdownDescription: "Next scheduled run timestamps (up to 5).", ElementType: types.StringType, Computed: true},
			"created_at":          schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":          schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
			"archived_at":         schema.StringAttribute{MarkdownDescription: "Archive timestamp; null while live.", Computed: true},
		},
	}
}

func (r *deploymentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (r *deploymentResource) schedule(ctx context.Context, m deploymentModel, diags *diag.Diagnostics) *client.Schedule {
	if m.Schedule.IsNull() || m.Schedule.IsUnknown() {
		return nil
	}
	var s scheduleModel
	diags.Append(m.Schedule.As(ctx, &s, basetypesObjectAsOptions)...)
	return &client.Schedule{Type: "cron", Expression: s.CronExpression.ValueString(), Timezone: s.Timezone.ValueString()}
}

// resources assembles the API resources array; secrets come from cfg (write-only).
func (r *deploymentResource) resources(ctx context.Context, plan, cfg deploymentModel, diags *diag.Diagnostics) json.RawMessage {
	out := []map[string]any{}
	var repos, cfgRepos []githubRepoModel
	if !plan.GitHubRepositories.IsNull() {
		diags.Append(plan.GitHubRepositories.ElementsAs(ctx, &repos, false)...)
	}
	if !cfg.GitHubRepositories.IsNull() {
		diags.Append(cfg.GitHubRepositories.ElementsAs(ctx, &cfgRepos, false)...)
	}
	for i, g := range repos {
		item := map[string]any{"type": "github_repository", "url": g.URL.ValueString()}
		if i < len(cfgRepos) && !cfgRepos[i].AuthorizationToken.IsNull() {
			item["authorization_token"] = cfgRepos[i].AuthorizationToken.ValueString()
		}
		if !g.CheckoutBranch.IsNull() {
			item["checkout"] = map[string]any{"type": "branch", "name": g.CheckoutBranch.ValueString()}
		} else if !g.CheckoutCommit.IsNull() {
			item["checkout"] = map[string]any{"type": "commit", "sha": g.CheckoutCommit.ValueString()}
		}
		if !g.MountPath.IsNull() {
			item["mount_path"] = g.MountPath.ValueString()
		}
		out = append(out, item)
	}
	var files []fileResourceModel
	if !plan.Files.IsNull() {
		diags.Append(plan.Files.ElementsAs(ctx, &files, false)...)
	}
	for _, f := range files {
		item := map[string]any{"type": "file", "file_id": f.FileID.ValueString()}
		if !f.MountPath.IsNull() {
			item["mount_path"] = f.MountPath.ValueString()
		}
		out = append(out, item)
	}
	var stores []memoryStoreRefModel
	if !plan.MemoryStores.IsNull() {
		diags.Append(plan.MemoryStores.ElementsAs(ctx, &stores, false)...)
	}
	for _, s := range stores {
		item := map[string]any{"type": "memory_store", "memory_store_id": s.MemoryStoreID.ValueString()}
		if !s.Access.IsNull() && !s.Access.IsUnknown() {
			item["access"] = s.Access.ValueString()
		}
		if !s.Instructions.IsNull() {
			item["instructions"] = s.Instructions.ValueString()
		}
		out = append(out, item)
	}
	b, _ := json.Marshal(out)
	return b
}

func (r *deploymentResource) budget(m deploymentModel) *client.Budget {
	if m.BudgetCents.IsNull() || m.BudgetCents.IsUnknown() {
		return nil
	}
	return &client.Budget{Type: "limit", MaxListCost: client.Money{Amount: m.BudgetCents.ValueString(), Currency: "USD"}}
}

// flatten copies the API object into the model. Lists are rebuilt from the
// API resources; the write-only authorization_token stays null.
func (r *deploymentResource) flatten(ctx context.Context, d *client.Deployment, m *deploymentModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Name)
	m.AgentID = types.StringValue(d.Agent.ID)
	m.AgentVersion = int64FromPtr(d.Agent.Version)
	m.EnvironmentID = types.StringValue(d.EnvironmentID)
	m.Description = stringFromPtr(d.Description)
	m.Metadata = metadataToMap(ctx, d.Metadata, m.Metadata, diags)
	m.Status = types.StringValue(d.Status)
	m.PausedReasonType, m.PausedReasonError = types.StringNull(), types.StringNull()
	if d.PausedReason != nil {
		m.PausedReasonType = types.StringValue(d.PausedReason.Type)
		if d.PausedReason.Error != nil {
			m.PausedReasonError = types.StringValue(d.PausedReason.Error.Type)
		}
	}
	m.Paused = types.BoolValue(d.Status == "paused")
	m.LastRunAt = stringFromPtr(d.LastRunAt)
	runs, dg := types.ListValueFrom(ctx, types.StringType, nonNilStrings(d.UpcomingRuns))
	diags.Append(dg...)
	m.UpcomingRunsAt = runs
	m.CreatedAt = types.StringValue(d.CreatedAt)
	m.UpdatedAt = types.StringValue(d.UpdatedAt)
	m.ArchivedAt = stringFromPtr(d.ArchivedAt)
	if m.ArchiveOnDestroy.IsNull() || m.ArchiveOnDestroy.IsUnknown() {
		m.ArchiveOnDestroy = types.BoolValue(true)
	}
	if d.Budget != nil {
		m.BudgetCents = types.StringValue(d.Budget.MaxListCost.Amount)
	} else {
		m.BudgetCents = types.StringNull()
	}
	if d.Schedule != nil {
		obj, dg := types.ObjectValue(scheduleAttrTypes, map[string]attr.Value{"cron_expression": types.StringValue(d.Schedule.Expression), "timezone": types.StringValue(d.Schedule.Timezone)})
		diags.Append(dg...)
		m.Schedule = obj
	} else {
		m.Schedule = types.ObjectNull(scheduleAttrTypes)
	}
	if len(d.VaultIDs) == 0 && (m.VaultIDs.IsNull() || m.VaultIDs.IsUnknown()) {
		m.VaultIDs = types.ListNull(types.StringType)
	} else {
		l, dg := types.ListValueFrom(ctx, types.StringType, nonNilStrings(d.VaultIDs))
		diags.Append(dg...)
		m.VaultIDs = l
	}
	// initial_events: keep the configured JSON when the API echoes a superset.
	if !m.InitialEvents.IsNull() && !m.InitialEvents.IsUnknown() && len(d.InitialEvents) > 0 {
		if ok, err := jsonSubsetEqual([]byte(m.InitialEvents.ValueString()), d.InitialEvents); err != nil || !ok {
			m.InitialEvents = jsontypes.NewNormalizedValue(string(d.InitialEvents))
		}
	} else if len(d.InitialEvents) > 0 {
		m.InitialEvents = jsontypes.NewNormalizedValue(string(d.InitialEvents))
	}
	r.flattenResources(d.Resources, m, diags)
}

func (r *deploymentResource) flattenResources(raw json.RawMessage, m *deploymentModel, diags *diag.Diagnostics) {
	var res []struct {
		Type          string                            `json:"type"`
		URL           *string                           `json:"url"`
		Checkout      *struct{ Type, Name, SHA string } `json:"checkout"`
		MountPath     *string                           `json:"mount_path"`
		FileID        *string                           `json:"file_id"`
		MemoryStoreID *string                           `json:"memory_store_id"`
		Access        *string                           `json:"access"`
		Instructions  *string                           `json:"instructions"`
	}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &res); err != nil {
			diags.AddError("Unexpected deployment resources payload", err.Error())
			return
		}
	}
	var repos, files, stores []attr.Value
	for _, x := range res {
		switch x.Type {
		case "github_repository":
			branch, commit := types.StringNull(), types.StringNull()
			if x.Checkout != nil {
				switch x.Checkout.Type {
				case "branch":
					branch = types.StringValue(x.Checkout.Name)
				case "commit":
					commit = types.StringValue(x.Checkout.SHA)
				}
			}
			obj, dg := types.ObjectValue(githubRepoAttrTypes, map[string]attr.Value{"url": stringFromPtr(x.URL), "authorization_token": types.StringNull(),
				"checkout_branch": branch, "checkout_commit": commit, "mount_path": stringFromPtr(x.MountPath)})
			diags.Append(dg...)
			repos = append(repos, obj)
		case "file":
			obj, dg := types.ObjectValue(fileResAttrTypes, map[string]attr.Value{"file_id": stringFromPtr(x.FileID), "mount_path": stringFromPtr(x.MountPath)})
			diags.Append(dg...)
			files = append(files, obj)
		case "memory_store":
			access := "read_write"
			if x.Access != nil {
				access = *x.Access
			}
			obj, dg := types.ObjectValue(memStoreRefTypes, map[string]attr.Value{"memory_store_id": stringFromPtr(x.MemoryStoreID), "access": types.StringValue(access), "instructions": stringFromPtr(x.Instructions)})
			diags.Append(dg...)
			stores = append(stores, obj)
		}
	}
	set := func(dst *types.List, elemTypes map[string]attr.Type, vals []attr.Value) {
		if len(vals) == 0 && (dst.IsNull() || dst.IsUnknown()) {
			*dst = types.ListNull(types.ObjectType{AttrTypes: elemTypes})
			return
		}
		if vals == nil {
			vals = []attr.Value{}
		}
		l, dg := types.ListValue(types.ObjectType{AttrTypes: elemTypes}, vals)
		diags.Append(dg...)
		*dst = l
	}
	set(&m.GitHubRepositories, githubRepoAttrTypes, repos)
	set(&m.Files, fileResAttrTypes, files)
	set(&m.MemoryStores, memStoreRefTypes, stores)
}

// reconcilePause pauses or unpauses to match the desired value.
func (r *deploymentResource) reconcilePause(ctx context.Context, d *client.Deployment, wantPaused bool) (*client.Deployment, error) {
	isPaused := d.Status == "paused"
	switch {
	case wantPaused && !isPaused:
		return r.client.PauseDeployment(ctx, d.ID)
	case !wantPaused && isPaused:
		return r.client.UnpauseDeployment(ctx, d.ID)
	}
	return d, nil
}

func (r *deploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg deploymentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.DeploymentCreate{
		Name:          plan.Name.ValueString(),
		Agent:         client.AgentRef{Type: "agent", ID: plan.AgentID.ValueString(), Version: int64Ptr(plan.AgentVersion)},
		EnvironmentID: plan.EnvironmentID.ValueString(),
		InitialEvents: json.RawMessage(plan.InitialEvents.ValueString()),
		Schedule:      r.schedule(ctx, plan, &resp.Diagnostics),
		Resources:     r.resources(ctx, plan, cfg, &resp.Diagnostics),
		Budget:        r.budget(plan),
		Description:   stringPtr(plan.Description),
		Metadata:      metadataFromMap(ctx, plan.Metadata, &resp.Diagnostics),
	}
	if !plan.VaultIDs.IsNull() && !plan.VaultIDs.IsUnknown() {
		in.VaultIDs = listStrings(&resp.Diagnostics, plan.VaultIDs)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.client.CreateDeployment(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating deployment", err)
		return
	}
	if d, err = r.reconcilePause(ctx, d, plan.Paused.ValueBool()); err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error pausing deployment", err)
		return
	}
	tflog.Trace(ctx, "created deployment", map[string]any{"id": d.ID})
	r.flatten(ctx, d, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.client.GetDeployment(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading deployment", err)
		return
	}
	if d.ArchivedAt != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	r.flatten(ctx, d, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *deploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, cfg deploymentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.DeploymentUpdate{Metadata: metadataPatch(ctx, plan.Metadata, state.Metadata, &resp.Diagnostics)}
	if !plan.Name.Equal(state.Name) {
		in.Name = stringPtr(plan.Name)
	}
	if !plan.AgentID.Equal(state.AgentID) || (!plan.AgentVersion.IsUnknown() && !plan.AgentVersion.Equal(state.AgentVersion)) {
		version := int64Ptr(plan.AgentVersion)
		if version == nil && plan.AgentID.Equal(state.AgentID) {
			version = int64Ptr(state.AgentVersion)
		}
		in.Agent = &client.AgentRef{Type: "agent", ID: plan.AgentID.ValueString(), Version: version}
	}
	if !plan.EnvironmentID.Equal(state.EnvironmentID) {
		in.EnvironmentID = stringPtr(plan.EnvironmentID)
	}
	if !plan.InitialEvents.Equal(state.InitialEvents) {
		in.InitialEvents = json.RawMessage(plan.InitialEvents.ValueString())
	}
	if !plan.Schedule.Equal(state.Schedule) {
		if plan.Schedule.IsNull() {
			in.Schedule = client.Null[*client.Schedule]()
		} else {
			in.Schedule = client.Some(r.schedule(ctx, plan, &resp.Diagnostics))
		}
	}
	if !plan.GitHubRepositories.Equal(state.GitHubRepositories) || !plan.Files.Equal(state.Files) || !plan.MemoryStores.Equal(state.MemoryStores) {
		in.Resources = r.resources(ctx, plan, cfg, &resp.Diagnostics)
	}
	if !plan.VaultIDs.Equal(state.VaultIDs) {
		ids := []string{}
		if !plan.VaultIDs.IsNull() {
			ids = listStrings(&resp.Diagnostics, plan.VaultIDs)
		}
		in.VaultIDs = &ids
	}
	if !plan.BudgetCents.Equal(state.BudgetCents) {
		if plan.BudgetCents.IsNull() {
			in.Budget = client.Null[*client.Budget]()
		} else {
			in.Budget = client.Some(r.budget(plan))
		}
	}
	if !plan.Description.Equal(state.Description) {
		if plan.Description.IsNull() {
			in.Description = client.Null[string]()
		} else {
			in.Description = client.Some(plan.Description.ValueString())
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.client.UpdateDeployment(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating deployment", err)
		return
	}
	if d, err = r.reconcilePause(ctx, d, plan.Paused.ValueBool()); err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error changing deployment pause state", err)
		return
	}
	r.flatten(ctx, d, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deploymentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.ArchiveOnDestroy.ValueBool() {
		return
	}
	if _, err := r.client.ArchiveDeployment(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error archiving deployment", err)
	}
}

func (r *deploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("archive_on_destroy"), types.BoolValue(true))...)
}

var digitsOnlyRegexp = regexp.MustCompile(`^[0-9]+$`)
