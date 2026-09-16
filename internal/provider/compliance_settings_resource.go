package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerResource(NewComplianceSettingsResource)
	registerDataSource(NewComplianceSettingsDataSource)
}

// --- resource ----------------------------------------------------------------

var (
	_ resource.Resource                = &complianceSettingsResource{}
	_ resource.ResourceWithConfigure   = &complianceSettingsResource{}
	_ resource.ResourceWithImportState = &complianceSettingsResource{}
)

// NewComplianceSettingsResource returns the anthropic_compliance_settings resource.
func NewComplianceSettingsResource() resource.Resource { return &complianceSettingsResource{} }

type complianceSettingsResource struct{ client *client.Client }

type complianceSettingsModel struct {
	ID    types.String `tfsdk:"id"`
	State types.String `tfsdk:"state"`
}

const complianceSettingsID = "compliance_settings"

func (r *complianceSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_settings"
}

func (r *complianceSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the organization-wide Compliance API toggle. This is a singleton: the organization has " +
			"exactly one settings object, so declare at most one instance. Requires `admin_api_key` or `oauth_token`.\n\n" +
			"~> Destroying this resource only removes it from state; the setting keeps its last value.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Always `compliance_settings`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "`enabled` or `disabled`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("enabled", "disabled")},
			},
		},
	}
}

func (r *complianceSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromResource(req, client.CredAdmin, &resp.Diagnostics)
}

func (r *complianceSettingsResource) apply(ctx context.Context, state string) (*client.ComplianceSettings, error) {
	return r.client.UpdateComplianceSettings(ctx, client.ComplianceSettingsUpdate{State: client.ComplianceState{Type: state}})
}

func (r *complianceSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan complianceSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cs, err := r.apply(ctx, plan.State.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating compliance settings", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, complianceSettingsModel{ID: types.StringValue(complianceSettingsID), State: types.StringValue(cs.State.Type)})...)
}

func (r *complianceSettingsResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	cs, err := r.client.GetComplianceSettings(ctx)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading compliance settings", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, complianceSettingsModel{ID: types.StringValue(complianceSettingsID), State: types.StringValue(cs.State.Type)})...)
}

func (r *complianceSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan complianceSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cs, err := r.apply(ctx, plan.State.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error updating compliance settings", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, complianceSettingsModel{ID: types.StringValue(complianceSettingsID), State: types.StringValue(cs.State.Type)})...)
}

func (r *complianceSettingsResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	// Singleton: nothing to delete server-side.
}

func (r *complianceSettingsResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(complianceSettingsID))...)
}

// --- data source -------------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &complianceSettingsDataSource{}

// NewComplianceSettingsDataSource returns the anthropic_compliance_settings data source.
func NewComplianceSettingsDataSource() datasource.DataSource { return &complianceSettingsDataSource{} }

type complianceSettingsDataSource struct{ client *client.Client }

func (d *complianceSettingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_settings"
}

func (d *complianceSettingsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Reads the organization-wide Compliance API toggle. Requires `admin_api_key` or `oauth_token`.",
		Attributes: map[string]dsschema.Attribute{
			"id":    dsString("Always `compliance_settings`."),
			"state": dsString("`enabled` or `disabled`."),
		},
	}
}

func (d *complianceSettingsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *complianceSettingsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	cs, err := d.client.GetComplianceSettings(ctx)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading compliance settings", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, complianceSettingsModel{ID: types.StringValue(complianceSettingsID), State: types.StringValue(cs.State.Type)})...)
}
