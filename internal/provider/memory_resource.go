package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewMemoryResource) }

var (
	_ resource.Resource                = &memoryResource{}
	_ resource.ResourceWithConfigure   = &memoryResource{}
	_ resource.ResourceWithImportState = &memoryResource{}
)

// maxMemoryContentBytes is the API's limit on one memory's content.
const maxMemoryContentBytes = 102400

// NewMemoryResource returns the anthropic_memory resource.
func NewMemoryResource() resource.Resource { return &memoryResource{} }

type memoryResource struct{ client *client.Client }

type memoryModel struct {
	ID               types.String `tfsdk:"id"`
	MemoryStoreID    types.String `tfsdk:"memory_store_id"`
	Path             types.String `tfsdk:"path"`
	Content          types.String `tfsdk:"content"`
	ContentSHA256    types.String `tfsdk:"content_sha256"`
	ContentSizeBytes types.Int64  `tfsdk:"content_size_bytes"`
	MemoryVersionID  types.String `tfsdk:"memory_version_id"`
	CreatedAt        types.String `tfsdk:"created_at"`
	UpdatedAt        types.String `tfsdk:"updated_at"`
}

func (r *memoryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memory"
}

func (r *memoryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages one memory: a text document at a path inside a Managed Agents memory store. Use it to seed " +
			"a store with reference material agents read, such as conventions or runbooks.\n\n" +
			"Agents can write to memories too. If an agent rewrites a memory this resource manages, the next plan shows the " +
			"agent's content being replaced with the configured one. For a memory agents are meant to keep editing, either " +
			"leave it out of Terraform or add `lifecycle { ignore_changes = [content] }`. Updates carry the content hash " +
			"Terraform last read, so an agent write landing between plan and apply fails the apply instead of being " +
			"overwritten unseen.\n\n`terraform destroy` deletes the memory. Its version history stays listable in the API." +
			managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Memory id (`mem_...`). Stable across renames.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"memory_store_id": schema.StringAttribute{
				MarkdownDescription: "Memory store to hold the memory (`memstore_...`). Changing it creates a new memory.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"path": schema.StringAttribute{
				MarkdownDescription: "Path within the store, for example `/conventions/style.md`. Must start with `/`, at most " +
					"1,024 bytes, unique within the store, case-sensitive, with no empty, `.` or `..` segments. Changing it " +
					"renames the memory in place.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(2, 1024),
					stringvalidator.RegexMatches(regexp.MustCompile(`^/`), "must start with /"),
				},
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "UTF-8 text content, at most 102,400 bytes. An empty string creates an empty memory. " +
					"Use `file()` to load it from disk.",
				Required:   true,
				Validators: []validator.String{stringvalidator.LengthAtMost(maxMemoryContentBytes)},
			},
			"content_sha256": schema.StringAttribute{
				MarkdownDescription: "Lowercase hex SHA-256 of `content`, as the API computes it.",
				Computed:            true,
			},
			"content_size_bytes": schema.Int64Attribute{
				MarkdownDescription: "Size of `content` in bytes.",
				Computed:            true,
			},
			"memory_version_id": schema.StringAttribute{
				MarkdownDescription: "Current version (`memver_...`). Changes with every content update.",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{MarkdownDescription: "Creation timestamp (RFC 3339).", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{MarkdownDescription: "Last update timestamp (RFC 3339).", Computed: true},
		},
	}
}

func (r *memoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (r *memoryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan memoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.CreateMemory(ctx, plan.MemoryStoreID.ValueString(), client.MemoryCreate{
		Path: plan.Path.ValueString(), Content: plan.Content.ValueString(),
	})
	if err != nil {
		if client.IsConflict(err) {
			resp.Diagnostics.AddError("Error creating memory",
				"A memory already exists at "+plan.Path.ValueString()+" in this store. Import it with "+
					"`terraform import` using memory_store_id/memory_id, or choose another path.\n\n"+err.Error())
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error creating memory", err)
		return
	}
	state := plan
	flattenMemory(m, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *memoryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state memoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := r.client.GetMemory(ctx, state.MemoryStoreID.ValueString(), state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading memory", err)
		return
	}
	flattenMemory(m, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *memoryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state memoryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.MemoryUpdate{
		Precondition: &client.MemoryPrecondition{Type: "content_sha256", ContentSHA256: state.ContentSHA256.ValueString()},
	}
	if !plan.Path.Equal(state.Path) {
		in.Path = stringPtr(plan.Path)
	}
	if !plan.Content.Equal(state.Content) {
		content := plan.Content.ValueString()
		in.Content = &content
	}
	m, err := r.client.UpdateMemory(ctx, state.MemoryStoreID.ValueString(), state.ID.ValueString(), in)
	if err != nil {
		if client.IsConflict(err) {
			resp.Diagnostics.AddError("Error updating memory",
				"The memory was not changed. Either its content changed after Terraform last read it, most likely "+
					"written by an agent, or another memory already uses the new path. Run `terraform plan` to see the "+
					"current content before applying again. To let agents keep editing it, add "+
					"`lifecycle { ignore_changes = [content] }`.\n\n"+err.Error())
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error updating memory", err)
		return
	}
	newState := plan
	flattenMemory(m, &newState)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *memoryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state memoryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteMemory(ctx, state.MemoryStoreID.ValueString(), state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error deleting memory", err)
	}
}

func (r *memoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := splitCompositeID(req.ID, "memory_store_id/memory_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("memory_store_id"), types.StringValue(parts[0]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(parts[1]))...)
}

// flattenMemory copies an API memory into the model. Content is kept from the
// model when the response omits it, which it does only for the basic view.
func flattenMemory(m *client.Memory, s *memoryModel) {
	s.ID = types.StringValue(m.ID)
	s.MemoryStoreID = types.StringValue(m.MemoryStoreID)
	s.Path = types.StringValue(m.Path)
	if m.Content != nil {
		s.Content = types.StringValue(*m.Content)
	}
	s.ContentSHA256 = types.StringValue(m.ContentSHA256)
	s.ContentSizeBytes = types.Int64Value(m.ContentSizeBytes)
	s.MemoryVersionID = types.StringValue(m.MemoryVersionID)
	s.CreatedAt = types.StringValue(m.CreatedAt)
	s.UpdatedAt = types.StringValue(m.UpdatedAt)
}
