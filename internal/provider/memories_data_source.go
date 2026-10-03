package provider

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewMemoriesDataSource) }

var attrTypesMemory = map[string]attr.Type{
	"id": types.StringType, "path": types.StringType, "content_sha256": types.StringType, "content_size_bytes": types.Int64Type,
	"memory_version_id": types.StringType, "created_at": types.StringType, "updated_at": types.StringType,
}

var _ datasource.DataSourceWithConfigure = &memoriesDataSource{}

// NewMemoriesDataSource returns the anthropic_memories data source.
func NewMemoriesDataSource() datasource.DataSource { return &memoriesDataSource{} }

type memoriesDataSource struct{ client *client.Client }

type memoriesDSModel struct {
	MemoryStoreID types.String `tfsdk:"memory_store_id"`
	PathPrefix    types.String `tfsdk:"path_prefix"`
	Memories      types.List   `tfsdk:"memories"`
}

func (d *memoriesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memories"
}

func (d *memoriesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the memories in a Managed Agents memory store, without their content. Compare " +
			"`content_sha256` against a local hash to find changed memories, and read one with the `anthropic_memory` " +
			"resource or the API." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"memory_store_id": schema.StringAttribute{MarkdownDescription: "Memory store to list (`memstore_...`).", Required: true},
			"path_prefix": schema.StringAttribute{
				MarkdownDescription: "Only list memories under this prefix, which must end with `/`, for example `/conventions/`. " +
					"The prefix is sent in the request URL, so keep secrets and personal data out of it.",
				Optional:   true,
				Validators: []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^/(.*/)?$`), "must start and end with /")},
			},
			"memories": schema.ListNestedAttribute{
				MarkdownDescription: "Memories, in the API's order.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id": dsString("Memory id (`mem_...`)."), "path": dsString("Path within the store."),
					"content_sha256":     dsString("Lowercase hex SHA-256 of the content."),
					"content_size_bytes": schema.Int64Attribute{MarkdownDescription: "Content size in bytes.", Computed: true},
					"memory_version_id":  dsString("Current version (`memver_...`)."),
					"created_at":         dsString("Creation timestamp."), "updated_at": dsString("Last update timestamp."),
				}},
			},
		},
	}
}

func (d *memoriesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *memoriesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg memoriesDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListMemories(ctx, cfg.MemoryStoreID.ValueString(), cfg.PathPrefix.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing memories", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for _, m := range list {
		if m.Type != "" && m.Type != "memory" {
			continue // a memory_prefix rollup; only returned when depth is set, which this never sends
		}
		rows = append(rows, map[string]attr.Value{
			"id": types.StringValue(m.ID), "path": types.StringValue(m.Path), "content_sha256": types.StringValue(m.ContentSHA256),
			"content_size_bytes": types.Int64Value(m.ContentSizeBytes), "memory_version_id": types.StringValue(m.MemoryVersionID),
			"created_at": types.StringValue(m.CreatedAt), "updated_at": types.StringValue(m.UpdatedAt),
		})
	}
	cfg.Memories = objectList(&resp.Diagnostics, attrTypesMemory, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
