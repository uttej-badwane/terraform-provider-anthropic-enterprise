package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

var pathToolsJSON = path.Root("tools_json")

func init() {
	registerDataSource(NewCountTokensDataSource)
}

var _ datasource.DataSourceWithConfigure = &countTokensDataSource{}

// NewCountTokensDataSource returns the anthropic_count_tokens data source.
func NewCountTokensDataSource() datasource.DataSource { return &countTokensDataSource{} }

type countTokensDataSource struct{ client *client.Client }

type countTokensMessageModel struct {
	Role    types.String `tfsdk:"role"`
	Content types.String `tfsdk:"content"`
}

type countTokensDSModel struct {
	Model       types.String              `tfsdk:"model"`
	System      types.String              `tfsdk:"system"`
	Messages    []countTokensMessageModel `tfsdk:"messages"`
	ToolsJSON   types.String              `tfsdk:"tools_json"`
	InputTokens types.Int64               `tfsdk:"input_tokens"`
}

func (d *countTokensDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_count_tokens"
}

func (d *countTokensDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Counts the input tokens a request would use, without sending it to a model.\n\n" +
			"Useful for asserting at plan time that a prompt fits a budget, for example an agent's system prompt. " +
			"The count depends on the model, so pass the one that will actually serve the request." + modelsNote,
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				MarkdownDescription: "Model to count against, for example `claude-opus-5`. A retired or unknown model fails the plan.",
				Required:            true,
			},
			"system": schema.StringAttribute{
				MarkdownDescription: "System prompt to include in the count.",
				Optional:            true,
			},
			"messages": schema.ListNestedAttribute{
				MarkdownDescription: "Conversation turns. At least one is required, even when only the system prompt is " +
					"of interest; a one-word user message adds only a few tokens.",
				Required: true,
				Validators: []validator.List{
					// The endpoint rejects an empty list with "at least one message is required".
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"role": schema.StringAttribute{
						MarkdownDescription: "`user` or `assistant`. A system prompt goes in the top-level `system` attribute; " +
							"the endpoint rejects a `system` role here.",
						Required:   true,
						Validators: []validator.String{stringvalidator.OneOf("user", "assistant")},
					},
					"content": schema.StringAttribute{MarkdownDescription: "Message text.", Required: true},
				}},
			},
			"tools_json": schema.StringAttribute{
				MarkdownDescription: "Tool definitions to include in the count, as a JSON array; build it with `jsonencode`. " +
					"Tools can dominate a count: one small tool definition added about 540 tokens in testing.",
				Optional: true,
			},
			"input_tokens": dsInt64("Input tokens the request would use."),
		},
	}
}

func (d *countTokensDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *countTokensDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg countTokensDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	in := client.CountTokensRequest{
		Model:  cfg.Model.ValueString(),
		System: cfg.System.ValueString(),
	}
	for _, m := range cfg.Messages {
		in.Messages = append(in.Messages, client.CountTokensMessage{Role: m.Role.ValueString(), Content: m.Content.ValueString()})
	}

	if !cfg.ToolsJSON.IsNull() && !cfg.ToolsJSON.IsUnknown() && cfg.ToolsJSON.ValueString() != "" {
		raw := json.RawMessage(cfg.ToolsJSON.ValueString())
		// Check the shape here so a malformed value is reported against the
		// attribute rather than surfacing as an opaque 400 from the API.
		var tools []json.RawMessage
		if err := json.Unmarshal(raw, &tools); err != nil {
			resp.Diagnostics.AddAttributeError(pathToolsJSON, "Invalid tools_json",
				"tools_json must be a JSON array of tool definitions: "+err.Error())
			return
		}
		in.Tools = raw
	}

	n, err := d.client.CountTokens(ctx, in)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error counting tokens", err)
		return
	}
	cfg.InputTokens = types.Int64Value(n)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
