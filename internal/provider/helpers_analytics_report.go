package provider

import (
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

var (
	usageGroupBy = []string{"claude_tag_category", "claude_tag_user_id", "context_window", "inference_geo", "model", "product", "rbac_group_id", "slack_channel_id", "speed"}
	costGroupBy  = append(append([]string{}, usageGroupBy...), "cost_type", "token_type")
	products     = []string{"chat", "claude-tag", "claude_code", "claude_design", "claude_in_chrome", "cowork", "office_agent"}
)

// analyticsReportInputsModel holds the shared inputs of the four Enterprise reports.
type analyticsReportInputsModel struct {
	StartingAt          types.String `tfsdk:"starting_at"`
	EndingAt            types.String `tfsdk:"ending_at"`
	BucketWidth         types.String `tfsdk:"bucket_width"`
	GroupBy             types.List   `tfsdk:"group_by"`
	Products            types.List   `tfsdk:"products"`
	Models              types.List   `tfsdk:"models"`
	ContextWindows      types.List   `tfsdk:"context_windows"`
	InferenceGeos       types.List   `tfsdk:"inference_geos"`
	Speeds              types.List   `tfsdk:"speeds"`
	RBACGroupIDs        types.List   `tfsdk:"rbac_group_ids"`
	UserIDs             types.List   `tfsdk:"user_ids"`
	SlackChannelIDs     types.List   `tfsdk:"slack_channel_ids"`
	ClaudeTagCategories types.List   `tfsdk:"claude_tag_categories"`
	ClaudeTagUserIDs    types.List   `tfsdk:"claude_tag_user_ids"`
}

func (m *analyticsReportInputsModel) params(diags *diag.Diagnostics) client.AnalyticsReportParams {
	return client.AnalyticsReportParams{
		StartingAt:          m.StartingAt.ValueString(),
		EndingAt:            m.EndingAt.ValueString(),
		BucketWidth:         m.BucketWidth.ValueString(),
		GroupBy:             listStrings(diags, m.GroupBy),
		Products:            listStrings(diags, m.Products),
		Models:              listStrings(diags, m.Models),
		ContextWindows:      listStrings(diags, m.ContextWindows),
		InferenceGeos:       listStrings(diags, m.InferenceGeos),
		Speeds:              listStrings(diags, m.Speeds),
		RBACGroupIDs:        listStrings(diags, m.RBACGroupIDs),
		UserIDs:             listStrings(diags, m.UserIDs),
		SlackChannelIDs:     listStrings(diags, m.SlackChannelIDs),
		ClaudeTagCategories: listStrings(diags, m.ClaudeTagCategories),
		ClaudeTagUserIDs:    listStrings(diags, m.ClaudeTagUserIDs),
	}
}

func oneOfList(desc string, values ...string) schema.ListAttribute {
	return optionalStringList(desc, listvalidator.ValueStringsAre(stringvalidator.OneOf(values...)))
}

// analyticsReportInputs are the shared input attributes of the Enterprise reports.
func analyticsReportInputs(groupBy []string, bucketRequiredNote string) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"starting_at": schema.StringAttribute{MarkdownDescription: "Inclusive start, RFC 3339. No earlier than 2026-01-01 and within the last 365 days.", Required: true},
		"ending_at":   schema.StringAttribute{MarkdownDescription: "Exclusive end, RFC 3339. Defaults to now; the range may span at most 31 days.", Optional: true},
		"bucket_width": schema.StringAttribute{MarkdownDescription: "Bucket granularity: `1d` (default), `1h` or `1m`." + bucketRequiredNote, Optional: true,
			Validators: []validator.String{stringvalidator.OneOf("1d", "1h", "1m")}},
		"group_by":              oneOfList("Dimensions to break each bucket out by.", groupBy...),
		"products":              oneOfList("Restrict to these products.", products...),
		"models":                optionalStringList("Restrict to these model names."),
		"context_windows":       oneOfList("Restrict to these context windows.", "0-200k", "200k-1M"),
		"inference_geos":        oneOfList("Restrict to these inference geographies.", "global", "us", "not_available"),
		"speeds":                oneOfList("Restrict to these speeds.", "standard", "fast"),
		"rbac_group_ids":        optionalStringList("Restrict to members of these RBAC groups."),
		"user_ids":              optionalStringList("Restrict to these users (`user_...`)."),
		"slack_channel_ids":     optionalStringList("Restrict to these Slack channel ids (Claude Tag)."),
		"claude_tag_categories": oneOfList("Restrict to these Claude Tag categories.", "dm", "engaged", "monitoring", "proactive", "scheduled"),
		"claude_tag_user_ids":   optionalStringList("Restrict to these Slack user ids (Claude Tag)."),
	}
}

// dimension attributes on usage/cost rows.
func reportDimAttrs(m map[string]schema.Attribute) map[string]schema.Attribute {
	for _, k := range []string{"model", "product", "context_window", "inference_geo", "speed", "rbac_group_id", "claude_tag_category", "claude_tag_user_id", "slack_channel_id"} {
		m[k] = dsString("Dimension `" + k + "`; null unless grouped by it.")
	}
	return m
}

func reportDimValues(m map[string]attr.Value, d client.AnalyticsDims) map[string]attr.Value {
	m["model"] = stringFromPtr(d.Model)
	m["product"] = stringFromPtr(d.Product)
	m["context_window"] = stringFromPtr(d.ContextWindow)
	m["inference_geo"] = stringFromPtr(d.InferenceGeo)
	m["speed"] = stringFromPtr(d.Speed)
	m["rbac_group_id"] = stringFromPtr(d.RBACGroupID)
	m["claude_tag_category"] = stringFromPtr(d.ClaudeTagCategory)
	m["claude_tag_user_id"] = stringFromPtr(d.ClaudeTagUserID)
	m["slack_channel_id"] = stringFromPtr(d.SlackChannelID)
	return m
}

func usageMetricAttrs(m map[string]schema.Attribute) map[string]schema.Attribute {
	m["uncached_input_tokens"] = dsInt64("Uncached input tokens.")
	m["cache_read_input_tokens"] = dsInt64("Input tokens read from cache.")
	m["cache_creation_1h_input_tokens"] = dsInt64("Input tokens written to the 1 hour cache.")
	m["cache_creation_5m_input_tokens"] = dsInt64("Input tokens written to the 5 minute cache.")
	m["output_tokens"] = dsInt64("Output tokens.")
	m["requests"] = dsInt64("Request count; null on some grouped rows.")
	m["web_search_requests"] = dsInt64("Server-side web search requests.")
	return reportDimAttrs(m)
}

func usageMetricValues(m map[string]attr.Value, r *client.AnalyticsUsageResult) map[string]attr.Value {
	m["uncached_input_tokens"] = types.Int64Value(r.UncachedInputTokens)
	m["cache_read_input_tokens"] = types.Int64Value(r.CacheReadInputTokens)
	m["cache_creation_1h_input_tokens"] = types.Int64Value(r.CacheCreation.Ephemeral1hInputTokens)
	m["cache_creation_5m_input_tokens"] = types.Int64Value(r.CacheCreation.Ephemeral5mInputTokens)
	m["output_tokens"] = types.Int64Value(r.OutputTokens)
	m["requests"] = int64FromPtr(r.Requests)
	m["web_search_requests"] = types.Int64Value(r.ServerToolUse.WebSearchRequests)
	return reportDimValues(m, r.AnalyticsDims)
}

func costMetricAttrs(m map[string]schema.Attribute) map[string]schema.Attribute {
	m["amount"] = dsString("Post-discount cost in fractional cents as a decimal string.")
	m["list_amount"] = dsString("List-price cost in fractional cents as a decimal string.")
	m["currency"] = dsString("Currency code, always `USD`.")
	m["requests"] = dsInt64("Request count; null when grouped by `cost_type` or `token_type`.")
	m["cost_type"] = dsString("`tokens`, `web_search` or `code_execution`; null unless grouped by `cost_type`.")
	m["token_type"] = dsString("Token type; null unless grouped by `token_type`.")
	return reportDimAttrs(m)
}

func costMetricValues(m map[string]attr.Value, r *client.AnalyticsCostResult) map[string]attr.Value {
	m["amount"] = types.StringValue(r.Amount)
	m["list_amount"] = types.StringValue(r.ListAmount)
	m["currency"] = types.StringValue(r.Currency)
	m["requests"] = int64FromPtr(r.Requests)
	m["cost_type"] = stringFromPtr(r.CostType)
	m["token_type"] = stringFromPtr(r.TokenType)
	return reportDimValues(m, r.AnalyticsDims)
}

// addRat parses a decimal string into total, warning on garbage.
func addRat(diags *diag.Diagnostics, total *big.Rat, s, field string) {
	if v, ok := new(big.Rat).SetString(s); ok {
		total.Add(total, v)
		return
	}
	diags.AddWarning("Unparseable amount", fmt.Sprintf("%s %q was skipped when computing the total", field, s))
}

// formatCents renders a rational with six decimals, matching the API format.
func formatCents(r *big.Rat) string { return r.FloatString(6) }
