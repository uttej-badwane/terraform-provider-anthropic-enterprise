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
	registerDataSource(NewMemoryStoreDataSource)
	registerDataSource(NewMemoryStoresDataSource)
}

var attrTypesMemoryStore = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "description": types.StringType, "metadata": types.MapType{ElemType: types.StringType},
	"created_at": types.StringType, "updated_at": types.StringType, "archived_at": types.StringType,
}

func memoryStoreAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": dsString("Memory store id."), "name": dsString("Name."), "description": dsString("Description; null when unset."),
		"metadata":   schema.MapAttribute{MarkdownDescription: "Metadata.", Computed: true, ElementType: types.StringType},
		"created_at": dsString("Creation timestamp."), "updated_at": dsString("Last update timestamp."), "archived_at": dsString("Archive timestamp; null while live."),
	}
}

func memoryStoreRow(ctx context.Context, ms *client.MemoryStore, diags *diag.Diagnostics) map[string]attr.Value {
	meta, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(ms.Metadata))
	diags.Append(d...)
	desc := types.StringNull()
	if ms.Description != "" {
		desc = types.StringValue(ms.Description)
	}
	return map[string]attr.Value{
		"id": types.StringValue(ms.ID), "name": types.StringValue(ms.Name), "description": desc, "metadata": meta,
		"created_at": types.StringValue(ms.CreatedAt), "updated_at": types.StringValue(ms.UpdatedAt), "archived_at": stringFromPtr(ms.ArchivedAt),
	}
}

// --- anthropic_memory_store --------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &memoryStoreDataSource{}

// NewMemoryStoreDataSource returns the anthropic_memory_store data source.
func NewMemoryStoreDataSource() datasource.DataSource { return &memoryStoreDataSource{} }

type memoryStoreDataSource struct{ client *client.Client }

func (d *memoryStoreDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memory_store"
}

func (d *memoryStoreDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := memoryStoreAttrs()
	attrs["id"] = schema.StringAttribute{MarkdownDescription: "Memory store id (`memstore_...`).", Required: true}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads one Managed Agents memory store." + managedAgentsNote, Attributes: attrs}
}

func (d *memoryStoreDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *memoryStoreDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, pathRootID, &id)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ms, err := d.client.GetMemoryStore(ctx, id.ValueString())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading memory store", err)
		return
	}
	obj, diags := types.ObjectValue(attrTypesMemoryStore, memoryStoreRow(ctx, ms, &resp.Diagnostics))
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}

// --- anthropic_memory_stores -----------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &memoryStoresDataSource{}

// NewMemoryStoresDataSource returns the anthropic_memory_stores data source.
func NewMemoryStoresDataSource() datasource.DataSource { return &memoryStoresDataSource{} }

type memoryStoresDataSource struct{ client *client.Client }

type memoryStoresDSModel struct {
	IncludeArchived types.Bool `tfsdk:"include_archived"`
	MemoryStores    types.List `tfsdk:"memory_stores"`
}

func (d *memoryStoresDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_memory_stores"
}

func (d *memoryStoresDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Managed Agents memory stores." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived stores. Defaults to `false`.", Optional: true},
			"memory_stores":    schema.ListNestedAttribute{MarkdownDescription: "Memory stores.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: memoryStoreAttrs()}},
		},
	}
}

func (d *memoryStoresDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *memoryStoresDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg memoryStoresDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListMemoryStores(ctx, cfg.IncludeArchived.ValueBool())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing memory stores", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for i := range list {
		rows = append(rows, memoryStoreRow(ctx, &list[i], &resp.Diagnostics))
	}
	cfg.MemoryStores = objectList(&resp.Diagnostics, attrTypesMemoryStore, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
