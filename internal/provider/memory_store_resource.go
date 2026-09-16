package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

func init() { registerResource(NewMemoryStoreResource) }

var (
	_ resource.Resource                = &memoryStoreResource{}
	_ resource.ResourceWithConfigure   = &memoryStoreResource{}
	_ resource.ResourceWithImportState = &memoryStoreResource{}
)

// NewMemoryStoreResource returns the anthropic_memory_store resource.
func NewMemoryStoreResource() resource.Resource { return &memoryStoreResource{} }

type memoryStoreResource struct{ client *client.Client }

type memoryStoreModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Metadata        types.Map    `tfsdk:"metadata"`
	DeleteOnDestroy types.Bool   `tfsdk:"delete_on_destroy"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	ArchivedAt      types.String `tfsdk:"archived_at"`
}

func (r *memoryStoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memory_store"
}

func (r *memoryStoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Managed Agents memory store: a persistent namespace agents read and write across sessions " +
			"(mounted at `/mnt/memory/<slug>/`). Names are not unique. The memories themselves are agent-written runtime data " +
			"and are not managed here.\n\n`terraform destroy` deletes the store and its memories. Set `delete_on_destroy = false` " +
			"to archive it instead. The API uses a separate beta header for memory stores; the provider sends it automatically." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Memory store id (`memstore_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name, 1 to 255 characters; drives the mount slug.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 255)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description, up to 1024 characters.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.LengthAtMost(1024)},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key/value metadata, max 16 pairs.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"delete_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Delete the store on destroy (default `true`). When `false`, it is archived instead.",
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

func (r *memoryStoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (r *memoryStoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan memoryStoreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.MemoryStoreCreate{Name: plan.Name.ValueString(), Description: stringPtr(plan.Description), Metadata: expandMetadata(ctx, plan.Metadata, &resp.Diagnostics)}
	if resp.Diagnostics.HasError() {
		return
	}
	ms, err := r.client.CreateMemoryStore(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating memory store", err)
		return
	}
	state := plan
	resp.Diagnostics.Append(flattenMemoryStore(ctx, ms, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *memoryStoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state memoryStoreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ms, err := r.client.GetMemoryStore(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading memory store", err)
		return
	}
	if ms.ArchivedAt != nil {
		tflog.Info(ctx, "memory store archived out of band; removing from state", map[string]any{"id": ms.ID})
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(flattenMemoryStore(ctx, ms, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *memoryStoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state memoryStoreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.MemoryStoreUpdate{}
	if !plan.Name.Equal(state.Name) {
		in.Name = stringPtr(plan.Name)
	}
	if !plan.Description.Equal(state.Description) {
		desc := plan.Description.ValueString() // null clears via ""
		in.Description = &desc
	}
	if !plan.Metadata.Equal(state.Metadata) {
		in.Metadata = metadataPatch(ctx, plan.Metadata, state.Metadata, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}
	ms, err := r.client.UpdateMemoryStore(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating memory store", err)
		return
	}
	newState := plan
	resp.Diagnostics.Append(flattenMemoryStore(ctx, ms, &newState)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *memoryStoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state memoryStoreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if state.DeleteOnDestroy.ValueBool() {
		if err := r.client.DeleteMemoryStore(ctx, id); err != nil && !client.IsNotFound(err) {
			apiErrorDiag(&resp.Diagnostics, "Error deleting memory store", err)
		}
		return
	}
	if _, err := r.client.ArchiveMemoryStore(ctx, id); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error archiving memory store", err)
	}
}

func (r *memoryStoreResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("delete_on_destroy"), types.BoolValue(true))...)
}

func flattenMemoryStore(ctx context.Context, ms *client.MemoryStore, m *memoryStoreModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(ms.ID)
	m.Name = types.StringValue(ms.Name)
	if ms.Description == "" {
		m.Description = types.StringNull()
	} else {
		m.Description = types.StringValue(ms.Description)
	}
	m.CreatedAt = types.StringValue(ms.CreatedAt)
	m.UpdatedAt = types.StringValue(ms.UpdatedAt)
	m.ArchivedAt = stringFromPtr(ms.ArchivedAt)
	if m.DeleteOnDestroy.IsNull() || m.DeleteOnDestroy.IsUnknown() {
		m.DeleteOnDestroy = types.BoolValue(true)
	}
	if len(ms.Metadata) == 0 && (m.Metadata.IsNull() || m.Metadata.IsUnknown()) {
		m.Metadata = types.MapNull(types.StringType)
	} else {
		mv, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(ms.Metadata))
		diags.Append(d...)
		m.Metadata = mv
	}
	return diags
}
