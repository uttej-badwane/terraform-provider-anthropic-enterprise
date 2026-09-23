package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerResource(NewServiceAccountResource) }

var (
	_ resource.Resource                = &serviceAccountResource{}
	_ resource.ResourceWithConfigure   = &serviceAccountResource{}
	_ resource.ResourceWithImportState = &serviceAccountResource{}
)

// slugRegexp matches the names accepted for service accounts, federation issuers and rules.
var slugRegexp = regexp.MustCompile(`^[a-z0-9-]+$`)

const slugMessage = "must contain only lowercase letters, digits and hyphens"

// NewServiceAccountResource returns the anthropic_service_account resource.
func NewServiceAccountResource() resource.Resource { return &serviceAccountResource{} }

type serviceAccountResource struct {
	client *client.Client
}

type serviceAccountModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	OrganizationRole  types.String `tfsdk:"organization_role"`
	ArchiveOnDestroy  types.Bool   `tfsdk:"archive_on_destroy"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
	ArchivedAt        types.String `tfsdk:"archived_at"`
	CreatedByActorID  types.String `tfsdk:"created_by_actor_id"`
	UpdatedByActorID  types.String `tfsdk:"updated_by_actor_id"`
	ArchivedByActorID types.String `tfsdk:"archived_by_actor_id"`
}

func (r *serviceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *serviceAccountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a service account: a non-human principal that receives short-lived tokens through " +
			"workload identity federation (see `anthropic_federation_rule`).\n\n" +
			"Requires `oauth_token` (an `org:admin` OAuth token); Admin API keys are rejected by this endpoint.\n\n" +
			"~> The Admin API cannot delete service accounts. `terraform destroy` **archives** the service account, which is " +
			"irreversible. Archiving fails while a live federation rule targets it. Set `archive_on_destroy = false` to only " +
			"remove the resource from state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Service account id (`svac_...`).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique slug: lowercase letters, digits and hyphens. Immutable; changing it forces a new service account.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(slugRegexp, slugMessage)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-text description. Removing it clears the description.",
				Optional:            true,
			},
			"organization_role": schema.StringAttribute{
				MarkdownDescription: "Organization role: `developer` (default) or `admin`. Assigning `admin` requires an interactive token.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("developer"),
				Validators:          []validator.String{stringvalidator.OneOf("developer", "admin")},
			},
			"archive_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Archive the service account when the resource is destroyed. Defaults to `true`. " +
					"When `false`, destroy only removes the resource from state. Imported resources start with `false`, so removing one from configuration cannot destroy it until you opt in.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "Creation timestamp (RFC 3339).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "Last update timestamp (RFC 3339).",
				Computed:            true,
			},
			"archived_at": schema.StringAttribute{
				MarkdownDescription: "Archive timestamp; null while the service account is live.",
				Computed:            true,
			},
			"created_by_actor_id": schema.StringAttribute{
				MarkdownDescription: "Id of the user or service account that created it.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_by_actor_id": schema.StringAttribute{
				MarkdownDescription: "Id of the user or service account that last updated it.",
				Computed:            true,
			},
			"archived_by_actor_id": schema.StringAttribute{
				MarkdownDescription: "Id of the user or service account that archived it; null while live.",
				Computed:            true,
			},
		},
	}
}

func (r *serviceAccountResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredOAuth, &resp.Diagnostics)
}

func (r *serviceAccountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan serviceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := r.client.CreateServiceAccount(ctx, client.ServiceAccountCreate{
		Name:             plan.Name.ValueString(),
		Description:      stringPtr(plan.Description),
		OrganizationRole: stringPtr(plan.OrganizationRole),
	})
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error creating service account", err)
		return
	}
	tflog.Trace(ctx, "created service account", map[string]any{"id": sa.ID})

	state := plan
	flattenServiceAccount(sa, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceAccountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sa, err := r.client.GetServiceAccount(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		apiErrorDiag(&resp.Diagnostics, "Error reading service account", err)
		return
	}
	if sa.ArchivedAt != nil {
		tflog.Info(ctx, "service account archived out of band; removing from state", map[string]any{"id": sa.ID})
		resp.State.RemoveResource(ctx)
		return
	}

	flattenServiceAccount(sa, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *serviceAccountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state serviceAccountModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := client.ServiceAccountUpdate{}
	if !plan.Description.Equal(state.Description) {
		if plan.Description.IsNull() {
			in.Description = client.Null[string]()
		} else {
			in.Description = client.Some(plan.Description.ValueString())
		}
	}
	if !plan.OrganizationRole.IsUnknown() && !plan.OrganizationRole.Equal(state.OrganizationRole) {
		in.OrganizationRole = stringPtr(plan.OrganizationRole)
	}

	sa, err := r.client.UpdateServiceAccount(ctx, state.ID.ValueString(), in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating service account", err)
		return
	}

	newState := plan
	flattenServiceAccount(sa, &newState)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *serviceAccountResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state serviceAccountModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.ArchiveOnDestroy.ValueBool() {
		tflog.Info(ctx, "archive_on_destroy is false; leaving service account in place", map[string]any{"id": state.ID.ValueString()})
		return
	}
	if _, err := r.client.ArchiveServiceAccount(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		apiErrorDiag(&resp.Diagnostics, "Error archiving service account", err)
	}
}

func (r *serviceAccountResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("archive_on_destroy"), types.BoolValue(false))...)
}

// flattenServiceAccount copies an API service account into the model. An empty
// description from the API is treated as null when the config has none.
func flattenServiceAccount(sa *client.ServiceAccount, m *serviceAccountModel) {
	m.ID = types.StringValue(sa.ID)
	m.Name = types.StringValue(sa.Name)
	m.OrganizationRole = types.StringValue(sa.OrganizationRole)
	m.CreatedAt = types.StringValue(sa.CreatedAt)
	m.UpdatedAt = types.StringValue(sa.UpdatedAt)
	m.ArchivedAt = stringFromPtr(sa.ArchivedAt)
	m.CreatedByActorID = stringFromPtr(sa.CreatedByActorID)
	m.UpdatedByActorID = stringFromPtr(sa.UpdatedByActorID)
	m.ArchivedByActorID = stringFromPtr(sa.ArchivedByActorID)
	if m.ArchiveOnDestroy.IsNull() || m.ArchiveOnDestroy.IsUnknown() {
		m.ArchiveOnDestroy = types.BoolValue(true)
	}
	if sa.Description == nil || (*sa.Description == "" && (m.Description.IsNull() || m.Description.IsUnknown())) {
		m.Description = types.StringNull()
	} else {
		m.Description = types.StringValue(*sa.Description)
	}
}

var attrTypesServiceAccount = map[string]attr.Type{
	"id":                   types.StringType,
	"name":                 types.StringType,
	"description":          types.StringType,
	"organization_role":    types.StringType,
	"created_at":           types.StringType,
	"updated_at":           types.StringType,
	"archived_at":          types.StringType,
	"created_by_actor_id":  types.StringType,
	"updated_by_actor_id":  types.StringType,
	"archived_by_actor_id": types.StringType,
}

func serviceAccountObject(sa *client.ServiceAccount) (types.Object, diag.Diagnostics) {
	desc := types.StringNull()
	if sa.Description != nil && *sa.Description != "" {
		desc = types.StringValue(*sa.Description)
	}
	return types.ObjectValue(attrTypesServiceAccount, map[string]attr.Value{
		"id":                   types.StringValue(sa.ID),
		"name":                 types.StringValue(sa.Name),
		"description":          desc,
		"organization_role":    types.StringValue(sa.OrganizationRole),
		"created_at":           types.StringValue(sa.CreatedAt),
		"updated_at":           types.StringValue(sa.UpdatedAt),
		"archived_at":          stringFromPtr(sa.ArchivedAt),
		"created_by_actor_id":  stringFromPtr(sa.CreatedByActorID),
		"updated_by_actor_id":  stringFromPtr(sa.UpdatedByActorID),
		"archived_by_actor_id": stringFromPtr(sa.ArchivedByActorID),
	})
}
