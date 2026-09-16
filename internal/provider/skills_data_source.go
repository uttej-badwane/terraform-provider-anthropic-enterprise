package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewSkillDataSource)
	registerDataSource(NewSkillsDataSource)
	registerDataSource(NewSkillVersionsDataSource)
}

var attrTypesSkillSummary = map[string]attr.Type{
	"id": types.StringType, "display_name": types.StringType, "latest_version_id": types.StringType, "source_type": types.StringType,
	"created_at": types.StringType, "updated_at": types.StringType,
}

func skillSummaryAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": dsString("Skill id."), "display_name": dsString("Display label."), "latest_version_id": dsString("Newest version id (what `latest` resolves to)."),
		"source_type": dsString("`custom`, `anthropic`, `anthropic_example` or `plugin`."), "created_at": dsString("Creation timestamp."), "updated_at": dsString("Last update timestamp."),
	}
}

func skillSummaryRow(sk *client.Skill) map[string]attr.Value {
	return map[string]attr.Value{
		"id": types.StringValue(sk.ID), "display_name": types.StringValue(sk.DisplayName), "latest_version_id": types.StringValue(sk.LatestVersionID),
		"source_type": types.StringValue(sk.Source.Type), "created_at": types.StringValue(sk.CreatedAt), "updated_at": types.StringValue(sk.UpdatedAt),
	}
}

// --- anthropic_skill ------------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &skillDataSource{}

// NewSkillDataSource returns the anthropic_skill data source.
func NewSkillDataSource() datasource.DataSource { return &skillDataSource{} }

type skillDataSource struct{ client *client.Client }

type skillDSModel struct {
	ID                       types.String `tfsdk:"id"`
	DisplayName              types.String `tfsdk:"display_name"`
	LatestVersionID          types.String `tfsdk:"latest_version_id"`
	SourceType               types.String `tfsdk:"source_type"`
	LatestVersionName        types.String `tfsdk:"latest_version_name"`
	LatestVersionDescription types.String `tfsdk:"latest_version_description"`
	CreatedAt                types.String `tfsdk:"created_at"`
	UpdatedAt                types.String `tfsdk:"updated_at"`
}

func (d *skillDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (d *skillDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := skillSummaryAttrs()
	attrs["id"] = schema.StringAttribute{MarkdownDescription: "Skill id (`skill_...`) or an Anthropic skill name such as `xlsx`.", Required: true}
	attrs["latest_version_name"] = dsString("Slug of the latest version.")
	attrs["latest_version_description"] = dsString("Description of the latest version.")
	resp.Schema = schema.Schema{MarkdownDescription: "Reads one skill and its latest version." + managedAgentsNote, Attributes: attrs}
}

func (d *skillDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *skillDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg skillDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sk, err := d.client.GetSkill(ctx, cfg.ID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading skill", err)
		return
	}
	cfg.DisplayName = types.StringValue(sk.DisplayName)
	cfg.LatestVersionID = types.StringValue(sk.LatestVersionID)
	cfg.SourceType = types.StringValue(sk.Source.Type)
	cfg.CreatedAt = types.StringValue(sk.CreatedAt)
	cfg.UpdatedAt = types.StringValue(sk.UpdatedAt)
	v, err := d.client.GetSkillVersion(ctx, sk.ID, "latest")
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading latest skill version", err)
		return
	}
	cfg.LatestVersionName = types.StringValue(v.Name)
	cfg.LatestVersionDescription = types.StringValue(v.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_skills -------------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &skillsDataSource{}

// NewSkillsDataSource returns the anthropic_skills data source.
func NewSkillsDataSource() datasource.DataSource { return &skillsDataSource{} }

type skillsDataSource struct{ client *client.Client }

type skillsDSModel struct {
	Source types.String `tfsdk:"source"`
	Skills types.List   `tfsdk:"skills"`
}

func (d *skillsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skills"
}

func (d *skillsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists skills visible to the workspace: custom uploads and Anthropic-published skills." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"source": schema.StringAttribute{MarkdownDescription: "Filter by `custom` or `anthropic`.", Optional: true, Validators: []validator.String{stringvalidator.OneOf("custom", "anthropic")}},
			"skills": schema.ListNestedAttribute{MarkdownDescription: "Skills.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: skillSummaryAttrs()}},
		},
	}
}

func (d *skillsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *skillsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg skillsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListSkills(ctx, cfg.Source.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing skills", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for i := range list {
		rows = append(rows, skillSummaryRow(&list[i]))
	}
	cfg.Skills = objectList(&resp.Diagnostics, attrTypesSkillSummary, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_skill_versions -----------------------------------------------------

var _ datasource.DataSourceWithConfigure = &skillVersionsDataSource{}

// NewSkillVersionsDataSource returns the anthropic_skill_versions data source.
func NewSkillVersionsDataSource() datasource.DataSource { return &skillVersionsDataSource{} }

type skillVersionsDataSource struct{ client *client.Client }

type skillVersionsDSModel struct {
	SkillID  types.String `tfsdk:"skill_id"`
	Versions types.List   `tfsdk:"versions"`
}

var attrTypesSkillVersion = map[string]attr.Type{"id": types.StringType, "name": types.StringType, "description": types.StringType, "created_at": types.StringType}

func (d *skillVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill_versions"
}

func (d *skillVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists a skill's versions, newest first." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"skill_id": schema.StringAttribute{MarkdownDescription: "Skill id.", Required: true},
			"versions": schema.ListNestedAttribute{MarkdownDescription: "Versions.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"id": dsString("Version id."), "name": dsString("Slug."), "description": dsString("Description."), "created_at": dsString("Upload timestamp."),
			}}},
		},
	}
}

func (d *skillVersionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *skillVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg skillVersionsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListSkillVersions(ctx, cfg.SkillID.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing skill versions", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for _, v := range list {
		rows = append(rows, map[string]attr.Value{"id": types.StringValue(v.ID), "name": types.StringValue(v.Name), "description": types.StringValue(v.Description), "created_at": types.StringValue(v.CreatedAt)})
	}
	cfg.Versions = objectList(&resp.Diagnostics, attrTypesSkillVersion, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
