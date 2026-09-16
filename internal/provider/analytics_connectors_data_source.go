package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

func init() { registerDataSource(NewAnalyticsConnectorsDataSource) }

var (
	_ datasource.DataSourceWithConfigure        = &analyticsConnectorsDataSource{}
	_ datasource.DataSourceWithConfigValidators = &analyticsConnectorsDataSource{}
)

// NewAnalyticsConnectorsDataSource returns the anthropic_analytics_connectors data source.
func NewAnalyticsConnectorsDataSource() datasource.DataSource {
	return &analyticsConnectorsDataSource{}
}

type analyticsConnectorsDataSource struct{ client *client.Client }

func connectorRowAttrs() map[string]schema.Attribute {
	return groupDimAttrs(map[string]schema.Attribute{
		"connector_name":                                          dsString("Normalized connector name, or an opaque connector id."),
		"connector_display_name":                                  dsString("Resolved display name when `connector_name` is an opaque id."),
		"distinct_user_count":                                     dsInt64("Distinct users who used the connector."),
		"individual_auth_distinct_user_count":                     dsInt64("Distinct users on their own credential; null when not reported."),
		"managed_auth_distinct_user_count":                        dsInt64("Distinct users on Enterprise Managed Auth; null when not reported."),
		"read_call_count":                                         dsInt64("Tool calls annotated read-only; null when the split is unavailable."),
		"write_call_count":                                        dsInt64("Tool calls annotated as writes; null when the split is unavailable."),
		"unclassified_call_count":                                 dsInt64("Tool calls without a trusted annotation; null when the split is unavailable."),
		"chat_distinct_conversation_connector_used_count":         dsInt64("Distinct chat conversations that used the connector."),
		"claude_code_distinct_session_connector_used_count":       dsInt64("Distinct Claude Code sessions that used the connector."),
		"cowork_distinct_session_connector_used_count":            dsInt64("Distinct Cowork sessions that used the connector."),
		"office_excel_distinct_session_connector_used_count":      dsInt64("Distinct Excel sessions that used the connector."),
		"office_outlook_distinct_session_connector_used_count":    dsInt64("Distinct Outlook sessions that used the connector."),
		"office_powerpoint_distinct_session_connector_used_count": dsInt64("Distinct PowerPoint sessions that used the connector."),
		"office_word_distinct_session_connector_used_count":       dsInt64("Distinct Word sessions that used the connector."),
	})
}

func (d *analyticsConnectorsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_analytics_connectors"
}

func (d *analyticsConnectorsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := analyticsListInputs([]string{"connector_name", "product", "rbac_group_id", "user_id"}, []string{"product", "rbac_group_id", "user_id"})
	attrs["rows"] = schema.ListNestedAttribute{MarkdownDescription: "One row per connector (times group dimensions).", Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: connectorRowAttrs()}}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Per-connector (MCP) usage for a Claude Enterprise organization, including read/write call classification " +
			"and managed-auth adoption." + analyticsNote,
		Attributes: attrs,
	}
}

func (d *analyticsConnectorsDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return analyticsListValidators()
}

func (d *analyticsConnectorsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromDataSource(req, client.CredAnalytics, &resp.Diagnostics)
}

func (d *analyticsConnectorsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg analyticsListModel
	p, ok := readAnalytics(ctx, req, resp, &cfg)
	if !ok {
		return
	}
	rows, err := d.client.ListAnalyticsConnectors(ctx, p)
	if err != nil {
		apiErrorDiag(&resp.Diagnostics, "Error reading analytics connectors", err)
		return
	}
	rows = capRows(rows, cfg.LimitRows)
	vals := make([]map[string]attr.Value, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		vals = append(vals, groupDimValues(map[string]attr.Value{
			"connector_name":                                          types.StringValue(r.ConnectorName),
			"connector_display_name":                                  stringFromPtr(r.ConnectorDisplayName),
			"distinct_user_count":                                     types.Int64Value(r.DistinctUserCount),
			"individual_auth_distinct_user_count":                     int64FromPtr(r.IndividualAuthDistinctUserCount),
			"managed_auth_distinct_user_count":                        int64FromPtr(r.ManagedAuthDistinctUserCount),
			"read_call_count":                                         int64FromPtr(r.ReadCallCount),
			"write_call_count":                                        int64FromPtr(r.WriteCallCount),
			"unclassified_call_count":                                 int64FromPtr(r.UnclassifiedCallCount),
			"chat_distinct_conversation_connector_used_count":         int64FromPtr(r.ChatMetrics.DistinctConversationConnectorUsedCount),
			"claude_code_distinct_session_connector_used_count":       int64FromPtr(r.ClaudeCodeMetrics.DistinctSessionConnectorUsedCount),
			"cowork_distinct_session_connector_used_count":            int64FromPtr(r.CoworkMetrics.DistinctSessionConnectorUsedCount),
			"office_excel_distinct_session_connector_used_count":      int64FromPtr(r.OfficeMetrics.Excel.DistinctSessionConnectorUsedCount),
			"office_outlook_distinct_session_connector_used_count":    int64FromPtr(r.OfficeMetrics.Outlook.DistinctSessionConnectorUsedCount),
			"office_powerpoint_distinct_session_connector_used_count": int64FromPtr(r.OfficeMetrics.PowerPoint.DistinctSessionConnectorUsedCount),
			"office_word_distinct_session_connector_used_count":       int64FromPtr(r.OfficeMetrics.Word.DistinctSessionConnectorUsedCount),
		}, r.Product, r.RBACGroupID, r.RBACGroupName, r.UserID))
	}
	cfg.Rows = objectList(&resp.Diagnostics, attrTypesFromSchema(connectorRowAttrs()), vals)
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
