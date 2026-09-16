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

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewVaultResource) }

var (
	_ resource.Resource                = &vaultResource{}
	_ resource.ResourceWithConfigure   = &vaultResource{}
	_ resource.ResourceWithImportState = &vaultResource{}
)

// NewVaultResource returns the anthropic_vault resource.
func NewVaultResource() resource.Resource { return &vaultResource{} }

type vaultResource struct{ client *client.Client }

type vaultModel struct {
	ID              types.String `tfsdk:"id"`
	DisplayName     types.String `tfsdk:"display_name"`
	Metadata        types.Map    `tfsdk:"metadata"`
	DeleteOnDestroy types.Bool   `tfsdk:"delete_on_destroy"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UpdatedAt       types.String `tfsdk:"updated_at"`
	ArchivedAt      types.String `tfsdk:"archived_at"`
}

func (r *vaultResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vault"
}

func (r *vaultResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Managed Agents vault: a container for credentials (`anthropic_vault_credential`) that " +
			"agents and deployments use at run time." + managedAgentsNote + "\n\n" +
			"~> With `delete_on_destroy = true` (the default) destroy hard-deletes the vault and every credential in it. " +
			"Set it to `false` to archive instead (secrets are purged, records kept).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Vault id (`vlt_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name, 1 to 255 characters. Not unique.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 255)},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Key/value metadata (up to 16 pairs).",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"delete_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Hard-delete on destroy (default `true`). `false` archives the vault instead. Imported resources start with `false`, so removing one from configuration cannot destroy it until you opt in.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"created_at":  schema.StringAttribute{MarkdownDescription: "Creation timestamp.", Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":  schema.StringAttribute{MarkdownDescription: "Last update timestamp.", Computed: true},
			"archived_at": schema.StringAttribute{MarkdownDescription: "Archive timestamp; null while live.", Computed: true},
		},
	}
}

func (r *vaultResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (r *vaultResource) flatten(ctx context.Context, v *client.Vault, m *vaultModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(v.ID)
	m.DisplayName = types.StringValue(v.DisplayName)
	m.Metadata = metadataToMap(ctx, v.Metadata, m.Metadata, diags)
	m.CreatedAt = types.StringValue(v.CreatedAt)
	m.UpdatedAt = types.StringValue(v.UpdatedAt)
	m.ArchivedAt = stringFromPtr(v.ArchivedAt)
	if m.DeleteOnDestroy.IsNull() || m.DeleteOnDestroy.IsUnknown() {
		m.DeleteOnDestroy = types.BoolValue(true)
	}
}

func (r *vaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vaultModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	v, err := r.client.CreateVault(ctx, client.VaultCreate{DisplayName: plan.DisplayName.ValueString(), Metadata: metadataFromMap(ctx, plan.Metadata, &resp.Diagnostics)})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating vault", err)
		return
	}
	r.flatten(ctx, v, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vaultModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	v, err := r.client.GetVault(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading vault", err)
		return
	}
	if v.ArchivedAt != nil {
		resp.State.RemoveResource(ctx)
		return
	}
	r.flatten(ctx, v, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vaultModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in := client.VaultUpdate{Metadata: metadataPatch(ctx, plan.Metadata, state.Metadata, &resp.Diagnostics)}
	if !plan.DisplayName.Equal(state.DisplayName) {
		in.DisplayName = stringPtr(plan.DisplayName)
	}
	v, err := r.client.UpdateVault(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating vault", err)
		return
	}
	r.flatten(ctx, v, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vaultModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var err error
	if state.DeleteOnDestroy.ValueBool() {
		err = r.client.DeleteVault(ctx, state.ID.ValueString())
	} else {
		_, err = r.client.ArchiveVault(ctx, state.ID.ValueString())
	}
	if err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error deleting vault", err)
	}
}

func (r *vaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("delete_on_destroy"), types.BoolValue(false))...)
}
