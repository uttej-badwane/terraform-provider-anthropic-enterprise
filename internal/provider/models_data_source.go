package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewModelDataSource)
	registerDataSource(NewModelsDataSource)
}

// The models endpoints are inference-scoped rather than administrative, so
// they take a regular workspace key. An org:admin OAuth token is rejected.
const modelsNote = "\n\n~> This data source needs `api_key` (a regular workspace or organization API key). " +
	"An `admin_api_key` or `oauth_token` is rejected: the endpoint requires an inference scope."

var attrTypesModel = map[string]attr.Type{
	"id":                types.StringType,
	"display_name":      types.StringType,
	"created_at":        types.StringType,
	"max_input_tokens":  types.Int64Type,
	"max_tokens":        types.Int64Type,
	"capabilities":      types.MapType{ElemType: types.BoolType},
	"capabilities_json": types.StringType,
}

func modelAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id":               dsString("Model id, as used by `anthropic_agent.model`."),
		"display_name":     dsString("Human-readable name."),
		"created_at":       dsString("Release timestamp (RFC 3339)."),
		"max_input_tokens": dsInt64("Largest context window the model accepts."),
		"max_tokens":       dsInt64("Largest output the model will produce."),
		"capabilities": schema.MapAttribute{
			MarkdownDescription: "Top-level support flag per capability, for example `code_execution` or `thinking`. " +
				"Use this for `lookup(...)` checks in configuration.",
			Computed:    true,
			ElementType: types.BoolType,
		},
		"capabilities_json": dsString("The capabilities payload verbatim, as JSON. " +
			"Some capabilities nest sub-capabilities that the flat `capabilities` map cannot express, " +
			"such as `context_management.compact_20260112`; reach them with `jsondecode`."),
	}
}

func modelRow(ctx context.Context, m *client.Model, diags *diag.Diagnostics) map[string]attr.Value {
	supported, err := m.SupportedCapabilities()
	if err != nil {
		diags.AddError("Unexpected capabilities payload",
			"Could not read the capabilities of model "+m.ID+": "+err.Error())
		supported = map[string]bool{}
	}
	caps, d := types.MapValueFrom(ctx, types.BoolType, supported)
	diags.Append(d...)

	rawJSON := types.StringNull()
	if len(m.Capabilities) > 0 {
		rawJSON = types.StringValue(string(m.Capabilities))
	}

	return map[string]attr.Value{
		"id":                types.StringValue(m.ID),
		"display_name":      types.StringValue(m.DisplayName),
		"created_at":        types.StringValue(m.CreatedAt),
		"max_input_tokens":  types.Int64Value(m.MaxInputTokens),
		"max_tokens":        types.Int64Value(m.MaxTokens),
		"capabilities":      caps,
		"capabilities_json": rawJSON,
	}
}

// --- anthropic_model ---------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &modelDataSource{}

// NewModelDataSource returns the anthropic_model data source.
func NewModelDataSource() datasource.DataSource { return &modelDataSource{} }

type modelDataSource struct{ client *client.Client }

func (d *modelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (d *modelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := modelAttrs()
	attrs["id"] = schema.StringAttribute{
		MarkdownDescription: "Model id, for example `claude-opus-5`. Reading a model that does not exist fails the plan, " +
			"which is the point: it catches a retired or mistyped id before an apply reaches it.",
		Required: true,
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads one model." + modelsNote, Attributes: attrs}
}

func (d *modelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *modelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRootID, &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m, err := d.client.GetModel(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading model", err)
		return
	}
	obj, diags := types.ObjectValue(attrTypesModel, modelRow(ctx, m, &resp.Diagnostics))
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}

// --- anthropic_models --------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &modelsDataSource{}

// NewModelsDataSource returns the anthropic_models data source.
func NewModelsDataSource() datasource.DataSource { return &modelsDataSource{} }

type modelsDataSource struct{ client *client.Client }

type modelsDSModel struct {
	Models types.List `tfsdk:"models"`
	IDs    types.List `tfsdk:"ids"`
}

func (d *modelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_models"
}

func (d *modelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the models the configured credential can call, newest first.\n\n" +
			"This reports what the credential can see, which is not the same as an organization-wide " +
			"allowlist: model entitlements can restrict access per role." + modelsNote,
		Attributes: map[string]schema.Attribute{
			"models": schema.ListNestedAttribute{
				MarkdownDescription: "Models, newest first.",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: modelAttrs()},
			},
			"ids": dsStringList("Just the model ids, in the same order, for `contains(...)` checks."),
		},
	}
}

func (d *modelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *modelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg modelsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListModels(ctx)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing models", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	ids := make([]string, 0, len(list))
	for i := range list {
		rows = append(rows, modelRow(ctx, &list[i], &resp.Diagnostics))
		ids = append(ids, list[i].ID)
	}
	idList, d2 := types.ListValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(d2...)
	cfg.Models = objectList(&resp.Diagnostics, attrTypesModel, rows)
	cfg.IDs = idList
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
