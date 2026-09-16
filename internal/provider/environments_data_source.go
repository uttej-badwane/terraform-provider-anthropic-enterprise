package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() {
	registerDataSource(NewEnvironmentDataSource)
	registerDataSource(NewEnvironmentsDataSource)
}

var attrTypesEnvironmentSummary = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "description": types.StringType, "type": types.StringType,
	"networking_type": types.StringType, "allowed_hosts": types.ListType{ElemType: types.StringType},
	"allow_mcp_servers": types.BoolType, "allow_package_managers": types.BoolType,
	"packages": types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
	"scope":    types.StringType, "metadata": types.MapType{ElemType: types.StringType},
	"created_at": types.StringType, "updated_at": types.StringType, "archived_at": types.StringType,
}

func environmentSummaryAttrs() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": dsString("Environment id."), "name": dsString("Name."), "description": dsString("Description."),
		"type": dsString("`cloud` or `self_hosted`."), "networking_type": dsString("`unrestricted` or `limited`; null for self-hosted."),
		"allowed_hosts":          dsStringList("Allowed hosts under limited networking."),
		"allow_mcp_servers":      dsBool("Whether MCP server traffic is allowed under limited networking."),
		"allow_package_managers": dsBool("Whether package-manager traffic is allowed under limited networking."),
		"packages":               schema.MapAttribute{MarkdownDescription: "Pre-installed packages keyed by ecosystem (`apt`, `cargo`, `gem`, `go`, `npm`, `pip`).", Computed: true, ElementType: types.ListType{ElemType: types.StringType}},
		"scope":                  dsString("`organization` or `account`."),
		"metadata":               schema.MapAttribute{MarkdownDescription: "Metadata.", Computed: true, ElementType: types.StringType},
		"created_at":             dsString("Creation timestamp."), "updated_at": dsString("Last update timestamp."), "archived_at": dsString("Archive timestamp; null while live."),
	}
}

func environmentSummaryRow(ctx context.Context, e *client.Environment, diags *diag.Diagnostics) map[string]attr.Value {
	netType, hosts, mcp, pm := types.StringNull(), types.ListNull(types.StringType), types.BoolNull(), types.BoolNull()
	if n := e.Config.Networking; n != nil {
		netType = types.StringValue(n.Type)
		l, d := types.ListValueFrom(ctx, types.StringType, nonNilSlice(n.AllowedHosts))
		diags.Append(d...)
		hosts, mcp, pm = l, boolFromPtrOrNull(n.AllowMCPServers), boolFromPtrOrNull(n.AllowPackageManagers)
	}
	pk := map[string][]string{}
	if p := e.Config.Packages; p != nil {
		pk = map[string][]string{"apt": nonNilSlice(p.Apt), "cargo": nonNilSlice(p.Cargo), "gem": nonNilSlice(p.Gem), "go": nonNilSlice(p.Go), "npm": nonNilSlice(p.Npm), "pip": nonNilSlice(p.Pip)}
	}
	pkv, d := types.MapValueFrom(ctx, types.ListType{ElemType: types.StringType}, pk)
	diags.Append(d...)
	meta, d := types.MapValueFrom(ctx, types.StringType, nonNilMap(e.Metadata))
	diags.Append(d...)
	return map[string]attr.Value{
		"id": types.StringValue(e.ID), "name": types.StringValue(e.Name), "description": stringFromPtr(e.Description), "type": types.StringValue(e.Config.Type),
		"networking_type": netType, "allowed_hosts": hosts, "allow_mcp_servers": mcp, "allow_package_managers": pm, "packages": pkv,
		"scope": types.StringValue(e.Scope), "metadata": meta,
		"created_at": types.StringValue(e.CreatedAt), "updated_at": types.StringValue(e.UpdatedAt), "archived_at": stringFromPtr(e.ArchivedAt),
	}
}

func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// --- anthropic_environment -------------------------------------------------------

var (
	_ datasource.DataSourceWithConfigure        = &environmentDataSource{}
	_ datasource.DataSourceWithConfigValidators = &environmentDataSource{}
)

// NewEnvironmentDataSource returns the anthropic_environment data source.
func NewEnvironmentDataSource() datasource.DataSource { return &environmentDataSource{} }

type environmentDataSource struct{ client *client.Client }

func (d *environmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *environmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := environmentSummaryAttrs()
	attrs["id"] = schema.StringAttribute{MarkdownDescription: "Environment id. Exactly one of `id` or `name` must be set.", Optional: true, Computed: true}
	attrs["name"] = schema.StringAttribute{MarkdownDescription: "Environment name (unique among live environments).", Optional: true, Computed: true}
	resp.Schema = schema.Schema{MarkdownDescription: "Looks up one Managed Agents environment by id or name." + managedAgentsNote, Attributes: attrs}
}

func (d *environmentDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name"))}
}

func (d *environmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *environmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var id, name types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var env *client.Environment
	if !id.IsNull() {
		e, err := d.client.GetEnvironment(ctx, id.ValueString())
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error reading environment", err)
			return
		}
		env = e
	} else {
		list, err := d.client.ListEnvironments(ctx, false)
		if err != nil {
			apiErrorDiag(&resp.Diagnostics, "Error listing environments", err)
			return
		}
		for i := range list {
			if list[i].Name == name.ValueString() {
				env = &list[i]
				break
			}
		}
		if env == nil {
			resp.Diagnostics.AddError("Environment not found", fmt.Sprintf("no live environment named %q", name.ValueString()))
			return
		}
	}
	obj, diags := types.ObjectValue(attrTypesEnvironmentSummary, environmentSummaryRow(ctx, env, &resp.Diagnostics))
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, obj)...)
}

// --- anthropic_environments ---------------------------------------------------------

var _ datasource.DataSourceWithConfigure = &environmentsDataSource{}

// NewEnvironmentsDataSource returns the anthropic_environments data source.
func NewEnvironmentsDataSource() datasource.DataSource { return &environmentsDataSource{} }

type environmentsDataSource struct{ client *client.Client }

type environmentsDSModel struct {
	IncludeArchived types.Bool `tfsdk:"include_archived"`
	Environments    types.List `tfsdk:"environments"`
}

func (d *environmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environments"
}

func (d *environmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists Managed Agents environments." + managedAgentsNote,
		Attributes: map[string]schema.Attribute{
			"include_archived": schema.BoolAttribute{MarkdownDescription: "Include archived environments. Defaults to `false`.", Optional: true},
			"environments":     schema.ListNestedAttribute{MarkdownDescription: "Environments.", Computed: true, NestedObject: schema.NestedAttributeObject{Attributes: environmentSummaryAttrs()}},
		},
	}
}

func (d *environmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAPIKey, &resp.Diagnostics)
}

func (d *environmentsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg environmentsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.ListEnvironments(ctx, cfg.IncludeArchived.ValueBool())
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error listing environments", err)
		return
	}
	rows := make([]map[string]attr.Value, 0, len(list))
	for i := range list {
		rows = append(rows, environmentSummaryRow(ctx, &list[i], &resp.Diagnostics))
	}
	cfg.Environments = objectList(&resp.Diagnostics, attrTypesEnvironmentSummary, rows)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
