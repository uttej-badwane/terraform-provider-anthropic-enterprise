package client

import (
	"context"
	"net/url"
	"strconv"
)

const analyticsPath = orgPath + "/analytics"

// AnalyticsListParams are the shared parameters of the per-entity analytics
// lists (users, skills, connectors, plugins, artifacts).
type AnalyticsListParams struct {
	Date         string
	StartingDate string
	EndingDate   string
	Filters      []string // dimension:value
	GroupBy      []string
	Order        string
	OrderBy      string
}

func (p AnalyticsListParams) values() url.Values {
	q := url.Values{}
	if p.Date != "" {
		q.Set("date", p.Date)
	}
	if p.StartingDate != "" {
		q.Set("starting_date", p.StartingDate)
	}
	if p.EndingDate != "" {
		q.Set("ending_date", p.EndingDate)
	}
	addAll(q, "filter[]", p.Filters)
	addAll(q, "group_by[]", p.GroupBy)
	if p.Order != "" {
		q.Set("order", p.Order)
	}
	if p.OrderBy != "" {
		q.Set("order_by", p.OrderBy)
	}
	return q
}

// ListAnalyticsUsers returns per-user activity rows.
func (c *Client) ListAnalyticsUsers(ctx context.Context, p AnalyticsListParams) ([]AnalyticsUserActivity, error) {
	return listToken[AnalyticsUserActivity](ctx, c, CredAnalytics, analyticsPath+"/users", p.values())
}

// ListAnalyticsSkills returns per-skill usage rows.
func (c *Client) ListAnalyticsSkills(ctx context.Context, p AnalyticsListParams) ([]AnalyticsSkillUsage, error) {
	return listToken[AnalyticsSkillUsage](ctx, c, CredAnalytics, analyticsPath+"/skills", p.values())
}

// ListAnalyticsConnectors returns per-connector usage rows.
func (c *Client) ListAnalyticsConnectors(ctx context.Context, p AnalyticsListParams) ([]AnalyticsConnectorUsage, error) {
	return listToken[AnalyticsConnectorUsage](ctx, c, CredAnalytics, analyticsPath+"/connectors", p.values())
}

// ListAnalyticsPlugins returns per-plugin usage rows.
func (c *Client) ListAnalyticsPlugins(ctx context.Context, p AnalyticsListParams) ([]AnalyticsPluginUsage, error) {
	return listToken[AnalyticsPluginUsage](ctx, c, CredAnalytics, analyticsPath+"/plugins", p.values())
}

// ListAnalyticsArtifacts returns the artifact-type cube for one day.
func (c *Client) ListAnalyticsArtifacts(ctx context.Context, p AnalyticsListParams) ([]AnalyticsArtifactUsage, error) {
	return listToken[AnalyticsArtifactUsage](ctx, c, CredAnalytics, analyticsPath+"/artifacts", p.values())
}

// AnalyticsReportParams are the shared parameters of the Enterprise usage and
// cost reports.
type AnalyticsReportParams struct {
	StartingAt          string
	EndingAt            string
	BucketWidth         string
	GroupBy             []string
	Products            []string
	Models              []string
	ContextWindows      []string
	InferenceGeos       []string
	Speeds              []string
	RBACGroupIDs        []string
	UserIDs             []string
	SlackChannelIDs     []string
	ClaudeTagCategories []string
	ClaudeTagUserIDs    []string
	// Per-user reports only.
	Order               string
	OrderBy             string
	ExcludeDeletedUsers bool
}

func (p AnalyticsReportParams) values() url.Values {
	q := url.Values{}
	q.Set("starting_at", p.StartingAt)
	if p.EndingAt != "" {
		q.Set("ending_at", p.EndingAt)
	}
	if p.BucketWidth != "" {
		q.Set("bucket_width", p.BucketWidth)
	}
	addAll(q, "group_by[]", p.GroupBy)
	addAll(q, "products[]", p.Products)
	addAll(q, "models[]", p.Models)
	addAll(q, "context_windows[]", p.ContextWindows)
	addAll(q, "inference_geos[]", p.InferenceGeos)
	addAll(q, "speeds[]", p.Speeds)
	addAll(q, "rbac_group_ids[]", p.RBACGroupIDs)
	addAll(q, "user_ids[]", p.UserIDs)
	addAll(q, "slack_channel_ids[]", p.SlackChannelIDs)
	addAll(q, "claude_tag_categories[]", p.ClaudeTagCategories)
	addAll(q, "claude_tag_user_ids[]", p.ClaudeTagUserIDs)
	if p.Order != "" {
		q.Set("order", p.Order)
	}
	if p.OrderBy != "" {
		q.Set("order_by", p.OrderBy)
	}
	if p.ExcludeDeletedUsers {
		q.Set("exclude_deleted_users", "true")
	}
	return q
}

type analyticsEnvelope[T any] struct {
	Data            []T     `json:"data"`
	HasMore         bool    `json:"has_more"`
	NextPage        *string `json:"next_page"`
	DataRefreshedAt *string `json:"data_refreshed_at"`
	OrganizationID  string  `json:"organization_id"`
}

func analyticsReport[T any](ctx context.Context, c *Client, path string, q url.Values) (*AnalyticsReport[T], error) {
	q.Set("limit", strconv.Itoa(listPageSize))
	out := &AnalyticsReport[T]{}
	for {
		var page analyticsEnvelope[T]
		if err := c.get(ctx, CredAnalytics, path, q, &page); err != nil {
			return nil, err
		}
		out.Data = append(out.Data, page.Data...)
		if page.DataRefreshedAt != nil {
			out.DataRefreshedAt = page.DataRefreshedAt
		}
		if page.OrganizationID != "" {
			out.OrganizationID = page.OrganizationID
		}
		if page.NextPage == nil || *page.NextPage == "" {
			return out, nil
		}
		q.Set("page", *page.NextPage)
	}
}

// GetAnalyticsUsageReport returns Enterprise token usage over time.
func (c *Client) GetAnalyticsUsageReport(ctx context.Context, p AnalyticsReportParams) (*AnalyticsReport[AnalyticsUsageBucket], error) {
	return analyticsReport[AnalyticsUsageBucket](ctx, c, analyticsPath+"/usage_report", p.values())
}

// GetAnalyticsCostReport returns Enterprise cost over time.
func (c *Client) GetAnalyticsCostReport(ctx context.Context, p AnalyticsReportParams) (*AnalyticsReport[AnalyticsCostBucket], error) {
	return analyticsReport[AnalyticsCostBucket](ctx, c, analyticsPath+"/cost_report", p.values())
}

// ListAnalyticsUserUsage returns per-user token usage rows.
func (c *Client) ListAnalyticsUserUsage(ctx context.Context, p AnalyticsReportParams) (*AnalyticsReport[AnalyticsUserUsageRow], error) {
	return analyticsReport[AnalyticsUserUsageRow](ctx, c, analyticsPath+"/user_usage_report", p.values())
}

// ListAnalyticsUserCost returns per-user cost rows.
func (c *Client) ListAnalyticsUserCost(ctx context.Context, p AnalyticsReportParams) (*AnalyticsReport[AnalyticsUserCostRow], error) {
	return analyticsReport[AnalyticsUserCostRow](ctx, c, analyticsPath+"/user_cost_report", p.values())
}
