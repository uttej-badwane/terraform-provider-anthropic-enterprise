package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAnalyticsPluginsDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &analyticsPluginsDataSource{}
	_ datasource.DataSourceWithConfigValidators = &analyticsPluginsDataSource{}
)

// NewAnalyticsPluginsDataSource returns the anthropic_analytics_plugins data source.
func NewAnalyticsPluginsDataSource() datasource.DataSource { return &analyticsPluginsDataSource{} }

type analyticsPluginsDataSource struct{ client *client.Client }

func pluginRowAttrs() map[string]schema.Attribute {
	return groupDimAttrs(map[string]schema.Attribute{
		"plugin_name":         dsString("Plugin name; `third-party` is an aggregate bucket for unnamed plugin activity."),
		"plugin_id":           dsString("Stable plugin identifier when available."),
		"distinct_user_count": dsInt64("Distinct users with install or invocation activity."),
		"install_count":       dsInt64("Distinct users who installed the plugin."),
		"invocation_count":    dsInt64("Plugin invocations."),
		"claude_code_distinct_session_plugin_used_count": dsInt64("Distinct Claude Code sessions that invoked the plugin."),
		"cowork_distinct_session_plugin_used_count":      dsInt64("Distinct Cowork sessions that invoked the plugin."),
	})
}

func (d *analyticsPluginsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_plugins"
}

func (d *analyticsPluginsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := analyticsListInputs([]string{"plugin_name", "product", "rbac_group_id", "user_id"}, []string{"product", "rbac_group_id", "user_id"})
	attrs["rows"] = schema.ListNestedAttribute{MarkdownDescription: "One row per plugin (times group dimensions).", Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: pluginRowAttrs()}}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Per-plugin install and invocation usage across Cowork and Claude Code for a Claude Enterprise organization." + analyticsNote,
		Attributes:          attrs,
	}
}

func (d *analyticsPluginsDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return analyticsListValidators()
}

func (d *analyticsPluginsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsPluginsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsListModel
	p, ok := readAnalytics(ctx, req, resp, &cfg)
	if !ok {
		return
	}
	rows, err := d.client.ListAnalyticsPlugins(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics plugins", err)
		return
	}
	rows = capRows(rows, cfg.LimitRows)
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		vals = append(vals, groupDimValues(map[string]attr.Value{
			"plugin_name":         types.StringValue(r.PluginName),
			"plugin_id":           stringFromPtr(r.PluginID),
			"distinct_user_count": types.Int64Value(r.DistinctUserCount),
			"install_count":       int64FromPtr(r.InstallCount),
			"invocation_count":    types.Int64Value(r.InvocationCount),
			"claude_code_distinct_session_plugin_used_count": int64FromPtr(r.ClaudeCodeMetrics.DistinctSessionPluginUsedCount),
			"cowork_distinct_session_plugin_used_count":      int64FromPtr(r.CoworkMetrics.DistinctSessionPluginUsedCount),
		}, r.Product, r.RBACGroupID, r.RBACGroupName, r.UserID))
	}
	cfg.Rows = objectList(&resp.Diagnostics, attrTypesFromSchema(pluginRowAttrs()), vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
