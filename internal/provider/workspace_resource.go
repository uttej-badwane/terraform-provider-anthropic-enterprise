package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
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

func init() { registerResource(NewWorkspaceResource) }

var (
	_ resource.Resource                = &workspaceResource{}
	_ resource.ResourceWithConfigure   = &workspaceResource{}
	_ resource.ResourceWithImportState = &workspaceResource{}
)

// NewWorkspaceResource returns the anthropic_workspace resource.
func NewWorkspaceResource() resource.Resource { return &workspaceResource{} }

type workspaceResource struct {
	client *client.Client
}

type workspaceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	DisplayColor         types.String `tfsdk:"display_color"`
	ExternalKeyID        types.String `tfsdk:"external_key_id"`
	Tags                 types.Map    `tfsdk:"tags"`
	AllowedInferenceGeos types.List   `tfsdk:"allowed_inference_geos"`
	DefaultInferenceGeo  types.String `tfsdk:"default_inference_geo"`
	WorkspaceGeo         types.String `tfsdk:"workspace_geo"`
	ArchiveOnDestroy     types.Bool   `tfsdk:"archive_on_destroy"`
	CompartmentID        types.String `tfsdk:"compartment_id"`
	CreatedAt            types.String `tfsdk:"created_at"`
	ArchivedAt           types.String `tfsdk:"archived_at"`
}

func (r *workspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (r *workspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a workspace in a Console organization. Workspaces isolate API keys, members, " +
			"rate limits and (optionally) a customer-managed encryption key.\n\n" +
			"Requires `admin_api_key` or `oauth_token`.\n\n" +
			"~> The Admin API cannot delete workspaces. `terraform destroy` **archives** the workspace, which is " +
			"irreversible and also archives every API key scoped to it. Set `archive_on_destroy = false` to leave the " +
			"workspace untouched and only remove it from state. The default workspace cannot be managed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Workspace id (`wrkspc_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name, 1 to 40 characters.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 40)},
			},
			"display_color": schema.StringAttribute{
				MarkdownDescription: "Hex color shown in the Console (for example `#6C5BB9`). Assigned by the API when omitted.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"external_key_id": schema.StringAttribute{
				MarkdownDescription: "Customer-managed encryption key to attach (`ekey_...`, see `anthropic_external_key`). " +
					"Write-once: it can be set on an existing workspace that has none, but changing it forces a new workspace.",
				Optional: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIf(
					func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
						resp.RequiresReplace = !req.StateValue.IsNull() && req.StateValue.ValueString() != "" &&
							!req.PlanValue.IsUnknown() && req.PlanValue.ValueString() != req.StateValue.ValueString()
					},
					"Changing or removing an attached external key requires a new workspace.",
					"Changing or removing an attached external key requires a new workspace.",
				)},
			},
			"tags": schema.MapAttribute{
				MarkdownDescription: "Key/value tags. Keys may not start with `anthropic`.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators:          []validator.Map{mapvalidator.KeysAre(notPrefixedWith("anthropic"))},
			},
			"allowed_inference_geos": schema.ListAttribute{
				MarkdownDescription: "Inference geographies the workspace may use (`global`, `us`). Omit for unrestricted.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"default_inference_geo": schema.StringAttribute{
				MarkdownDescription: "Default inference geography (`global` or `us`). Defaults to `global`.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.OneOf("global", "us")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workspace_geo": schema.StringAttribute{
				MarkdownDescription: "Geography where workspace data is stored. Only `us` today. Immutable.",
				Optional:            true,
				Computed:            true,
				Validators:          []validator.String{stringvalidator.OneOf("us")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"archive_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Archive the workspace when the resource is destroyed. Defaults to `true`. " +
					"When `false`, destroy only removes the resource from state. Imported resources start with `false`, so removing one from configuration cannot destroy it until you opt in.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"compartment_id": schema.StringAttribute{
				MarkdownDescription: "Encryption compartment id.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "Archive timestamp; null while the workspace is live.",
				Computed:            true,
			},
		},
	}
}

func (r *workspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAdmin, &resp.Diagnostics)
}

func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := client.WorkspaceCreate{
		Name:          plan.Name.ValueString(),
		DisplayColor:  stringPtr(plan.DisplayColor),
		ExternalKeyID: stringPtr(plan.ExternalKeyID),
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		tags := map[string]string{}
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		in.Tags = tags
	}
	in.DataResidency = residencyParams(ctx, plan, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	ws, err := r.client.CreateWorkspace(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating workspace", err)
		return
	}
	tflog.Trace(ctx, "created workspace", map[string]any{"id": ws.ID})

	state := plan
	resp.Diagnostics.Append(flattenWorkspace(ctx, ws, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ws, err := r.client.GetWorkspace(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading workspace", err)
		return
	}
	if ws.ArchivedAt != nil {
		tflog.Info(ctx, "workspace archived out of band; removing from state", map[string]any{"id": ws.ID})
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(flattenWorkspace(ctx, ws, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state workspaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := client.WorkspaceUpdate{}
	if !plan.Name.Equal(state.Name) {
		in.Name = stringPtr(plan.Name)
	}
	if !plan.DisplayColor.IsUnknown() && !plan.DisplayColor.IsNull() && !plan.DisplayColor.Equal(state.DisplayColor) {
		in.DisplayColor = stringPtr(plan.DisplayColor)
	}
	if !plan.ExternalKeyID.IsNull() && !plan.ExternalKeyID.Equal(state.ExternalKeyID) {
		in.ExternalKeyID = stringPtr(plan.ExternalKeyID)
	}
	if !plan.Tags.Equal(state.Tags) {
		want := map[string]string{}
		have := map[string]string{}
		if !plan.Tags.IsNull() {
			resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &want, false)...)
		}
		if !state.Tags.IsNull() {
			resp.Diagnostics.Append(state.Tags.ElementsAs(ctx, &have, false)...)
		}
		in.Tags = map[string]*string{}
		for k, v := range want {
			in.Tags[k] = &v
		}
		for k := range have {
			if _, ok := want[k]; !ok {
				in.Tags[k] = nil
			}
		}
	}
	if !plan.AllowedInferenceGeos.Equal(state.AllowedInferenceGeos) || (!plan.DefaultInferenceGeo.IsUnknown() && !plan.DefaultInferenceGeo.Equal(state.DefaultInferenceGeo)) {
		in.DataResidency = residencyParams(ctx, plan, false, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	ws, err := r.client.UpdateWorkspace(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating workspace", err)
		return
	}

	newState := plan
	resp.Diagnostics.Append(flattenWorkspace(ctx, ws, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *workspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.ArchiveOnDestroy.ValueBool() {
		tflog.Info(ctx, "archive_on_destroy is false; leaving workspace in place", map[string]any{"id": state.ID.ValueString()})
		return
	}
	if _, err := r.client.ArchiveWorkspace(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error archiving workspace", err)
	}
}

func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("archive_on_destroy"), types.BoolValue(false))...)
}

// residencyParams builds the data_residency request block from the plan.
func residencyParams(ctx context.Context, plan workspaceModel, create bool, diags *diag.Diagnostics) *client.DataResidencyParams {
	out := &client.DataResidencyParams{}
	set := false
	if !plan.AllowedInferenceGeos.IsNull() && !plan.AllowedInferenceGeos.IsUnknown() {
		var geos []string
		diags.Append(plan.AllowedInferenceGeos.ElementsAs(ctx, &geos, false)...)
		out.AllowedInferenceGeos = &client.AllowedGeos{Geos: geos}
		set = true
	} else if !create {
		out.AllowedInferenceGeos = &client.AllowedGeos{Unrestricted: true}
		set = true
	}
	if !plan.DefaultInferenceGeo.IsNull() && !plan.DefaultInferenceGeo.IsUnknown() {
		out.DefaultInferenceGeo = stringPtr(plan.DefaultInferenceGeo)
		set = true
	}
	if create && !plan.WorkspaceGeo.IsNull() && !plan.WorkspaceGeo.IsUnknown() {
		out.WorkspaceGeo = stringPtr(plan.WorkspaceGeo)
		set = true
	}
	if !set {
		return nil
	}
	return out
}

// flattenWorkspace copies an API workspace into the model. Config-only
// attributes (tags, allowed_inference_geos) keep null when the API reports
// the empty/unrestricted default and the config did not set them.
func flattenWorkspace(ctx context.Context, ws *client.Workspace, m *workspaceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(ws.ID)
	m.Name = types.StringValue(ws.Name)
	m.DisplayColor = types.StringValue(ws.DisplayColor)
	m.ExternalKeyID = stringFromPtr(ws.ExternalKeyID)
	m.CompartmentID = types.StringValue(ws.CompartmentID)
	m.CreatedAt = types.StringValue(ws.CreatedAt)
	m.ArchivedAt = stringFromPtr(ws.ArchivedAt)
	if m.ArchiveOnDestroy.IsNull() || m.ArchiveOnDestroy.IsUnknown() {
		m.ArchiveOnDestroy = types.BoolValue(true)
	}

	if len(ws.Tags) == 0 && (m.Tags.IsNull() || m.Tags.IsUnknown()) {
		m.Tags = types.MapNull(types.StringType)
	} else {
		tags := ws.Tags
		if tags == nil {
			tags = map[string]string{}
		}
		v, d := types.MapValueFrom(ctx, types.StringType, tags)
		diags.Append(d...)
		m.Tags = v
	}

	if ws.DataResidency == nil {
		m.AllowedInferenceGeos = types.ListNull(types.StringType)
		if m.DefaultInferenceGeo.IsUnknown() {
			m.DefaultInferenceGeo = types.StringNull()
		}
		if m.WorkspaceGeo.IsUnknown() {
			m.WorkspaceGeo = types.StringNull()
		}
		return diags
	}
	if ws.DataResidency.AllowedInferenceGeos.Unrestricted {
		m.AllowedInferenceGeos = types.ListNull(types.StringType)
	} else {
		v, d := types.ListValueFrom(ctx, types.StringType, ws.DataResidency.AllowedInferenceGeos.Geos)
		diags.Append(d...)
		m.AllowedInferenceGeos = v
	}
	m.DefaultInferenceGeo = types.StringValue(ws.DataResidency.DefaultInferenceGeo)
	m.WorkspaceGeo = types.StringValue(ws.DataResidency.WorkspaceGeo)
	return diags
}

// attrTypesWorkspace is reused by data sources that list workspaces.
var attrTypesWorkspace = map[string]attr.Type{
	"id":                     types.StringType,
	"name":                   types.StringType,
	"display_color":          types.StringType,
	"external_key_id":        types.StringType,
	"tags":                   types.MapType{ElemType: types.StringType},
	"allowed_inference_geos": types.ListType{ElemType: types.StringType},
	"default_inference_geo":  types.StringType,
	"workspace_geo":          types.StringType,
	"compartment_id":         types.StringType,
	"created_at":             types.StringType,
	"archived_at":            types.StringType,
}

func workspaceObject(ctx context.Context, ws *client.Workspace) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	tags, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(ws.Tags))
	diags.Append(d...)
	geos := types.ListNull(types.StringType)
	defGeo, wsGeo := types.StringNull(), types.StringNull()
	if ws.DataResidency != nil {
		if !ws.DataResidency.AllowedInferenceGeos.Unrestricted {
			geos, d = types.ListValueFrom(ctx, types.StringType, ws.DataResidency.AllowedInferenceGeos.Geos)
			diags.Append(d...)
		}
		defGeo = types.StringValue(ws.DataResidency.DefaultInferenceGeo)
		wsGeo = types.StringValue(ws.DataResidency.WorkspaceGeo)
	}
	obj, d := types.ObjectValue(attrTypesWorkspace, map[string]attr.Value{
		"id":                     types.StringValue(ws.ID),
		"name":                   types.StringValue(ws.Name),
		"display_color":          types.StringValue(ws.DisplayColor),
		"external_key_id":        stringFromPtr(ws.ExternalKeyID),
		"tags":                   tags,
		"allowed_inference_geos": geos,
		"default_inference_geo":  defGeo,
		"workspace_geo":          wsGeo,
		"compartment_id":         types.StringValue(ws.CompartmentID),
		"created_at":             types.StringValue(ws.CreatedAt),
		"archived_at":            stringFromPtr(ws.ArchivedAt),
	})
	diags.Append(d...)
	return obj, diags
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
