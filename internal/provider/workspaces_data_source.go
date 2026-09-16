package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewWorkspacesDataSource)
	registerDataSource(NewWorkspaceDataSource)
}

var workspaceDSAttributes = map[string]schema.Attribute{
	"id":                     dsString("Workspace id."),
	"name":                   dsString("Display name."),
	"display_color":          dsString("Hex color."),
	"external_key_id":        dsString("Attached customer-managed encryption key id, if any."),
	"tags":                   schema.MapAttribute{MarkdownDescription: "Tags.", Computed: true, ElementType: types.StringType},
	"allowed_inference_geos": dsStringList("Allowed inference geographies; null when unrestricted."),
	"default_inference_geo":  dsString("Default inference geography."),
	"workspace_geo":          dsString("Workspace data geography."),
	"compartment_id":         dsString("Encryption compartment id."),
	"created_at":             dsString("Creation timestamp."),
	"archived_at":            dsString("Archive timestamp; null while live."),
}

// --- anthropic_workspaces ---------------------------------------------------

var _ datasource.DataSourceWithConfigure = &workspacesDataSource{}

// NewWorkspacesDataSource returns the anthropic_workspaces data source.
func NewWorkspacesDataSource() datasource.DataSource { return &workspacesDataSource{} }

type workspacesDataSource struct{ client *client.Client }

type workspacesModel struct {
	IncludeArchived types.Bool `tfsdk:"include_archived"`
	Workspaces      types.List `tfsdk:"workspaces"`
}

func (d *workspacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspaces"
}

func (d *workspacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists workspaces in the organization. Requires `admin_api_key` or `oauth_token`. " +
			"The default workspace is not returned by the API.",
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived workspaces. Defaults to `false`.", Optional: true},
			"workspaces": schema.ListNestedAttribute{
				MarkdownDescription: "Workspaces.",
				Computed:            true,
				NestedObject:        schema.NestedAttributeObject{Attributes: workspaceDSAttributes},
			},
		},
	}
}

func (d *workspacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *workspacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg workspacesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListWorkspaces(ctx, cfg.IncludeArchived.ValueBool())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing workspaces", err)
		return
	}
	objs := make([]attr.Value, 0, len(list))
	for i := range list {
		obj, diags := workspaceObject(ctx, &list[i])
		resp.Diagnostics.Append(diags...)
		objs = append(objs, obj)
	}
	l, diags := types.ListValue(types.ObjectType{AttrTypes: attrTypesWorkspace}, objs)
	resp.Diagnostics.Append(diags...)
	cfg.Workspaces = l
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}

// --- anthropic_workspace ----------------------------------------------------

var (
	_ datasource.DataSourceWithConfigure        = &workspaceDataSource{}
	_ datasource.DataSourceWithConfigValidators = &workspaceDataSource{}
)

// NewWorkspaceDataSource returns the anthropic_workspace data source.
func NewWorkspaceDataSource() datasource.DataSource { return &workspaceDataSource{} }

type workspaceDataSource struct{ client *client.Client }

func (d *workspaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (d *workspaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{}
	for k, v := range workspaceDSAttributes {
		attrs[k] = v
	}
	attrs["id"] = schema.StringAttribute{MarkdownDescription: "Workspace id. Exactly one of `id` or `name` must be set.", Optional: true, Computed: true}
	attrs["name"] = schema.StringAttribute{MarkdownDescription: "Workspace name (must be unique among live workspaces).", Optional: true, Computed: true}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up one workspace by id or name. Requires `admin_api_key` or `oauth_token`.",
		Attributes:          attrs,
	}
}

func (d *workspaceDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name"))}
}

func (d *workspaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAdmin, &resp.Diagnostics)
}

func (d *workspaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ws *client.Workspace
	if !id.IsNull() {
		w, err := d.client.GetWorkspace(ctx, id.ValueString())
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error reading workspace", err)
			return
		}
		ws = w
	} else {
		list, err := d.client.ListWorkspaces(ctx, false)
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error listing workspaces", err)
			return
		}
		for i := range list {
			if list[i].Name == name.ValueString() {
				if ws != nil {
					resp.Diagnostics.AddError("Ambiguous workspace name", fmt.Sprintf("more than one live workspace is named %q; use id", name.ValueString()))
					return
				}
				ws = &list[i]
			}
		}
		if ws == nil {
			resp.Diagnostics.AddError("Workspace not found", fmt.Sprintf("no live workspace named %q", name.ValueString()))
			return
		}
	}
	obj, diags := workspaceObject(ctx, ws)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}
