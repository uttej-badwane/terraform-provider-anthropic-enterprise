package client

import (
	"context"
	"net/url"
	"slices"
)

// --- spend limit increase requests (Enterprise) -----------------------------

// IncreaseRequestListOptions filters ListSpendLimitIncreaseRequests.
type IncreaseRequestListOptions struct {
	Statuses []string
	ActorIDs []string
}

// ListSpendLimitIncreaseRequests lists members' requests for higher limits.
func (c *Client) ListSpendLimitIncreaseRequests(ctx context.Context, opts IncreaseRequestListOptions) ([]SpendLimitIncreaseRequest, error) {
	q := url.Values{}
	for _, s := range opts.Statuses {
		q.Add("status[]", s)
	}
	for _, a := range opts.ActorIDs {
		q.Add("actor_ids[]", a)
	}
	return listToken[SpendLimitIncreaseRequest](ctx, c, CredEnterprise, orgPath+"/spend_limit_increase_requests", q)
}

// GetSpendLimitIncreaseRequest returns one request.
func (c *Client) GetSpendLimitIncreaseRequest(ctx context.Context, id string) (*SpendLimitIncreaseRequest, error) {
	var out SpendLimitIncreaseRequest
	if err := c.get(ctx, CredEnterprise, orgPath+"/spend_limit_increase_requests/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- usage / cost / claude code reports (Console) ----------------------------

func addAll(q url.Values, key string, vals []string) {
	for _, v := range vals {
		q.Add(key, v)
	}
}

// GetUsageReport returns every bucket of the messages usage report.
func (c *Client) GetUsageReport(ctx context.Context, p UsageReportParams) ([]UsageBucket, error) {
	q := url.Values{}
	q.Set("starting_at", p.StartingAt)
	if p.EndingAt != "" {
		q.Set("ending_at", p.EndingAt)
	}
	if p.BucketWidth != "" {
		q.Set("bucket_width", p.BucketWidth)
	}
	addAll(q, "group_by[]", p.GroupBy)
	addAll(q, "api_key_ids[]", p.APIKeyIDs)
	addAll(q, "workspace_ids[]", p.WorkspaceIDs)
	addAll(q, "models[]", p.Models)
	addAll(q, "service_tiers[]", p.ServiceTiers)
	addAll(q, "context_window[]", p.ContextWindows)
	addAll(q, "inference_geos[]", p.InferenceGeos)
	addAll(q, "service_account_ids[]", p.ServiceAccountIDs)
	addAll(q, "account_ids[]", p.AccountIDs)
	addAll(q, "speeds[]", p.Speeds)

	// The speed dimension is gated on a beta header, and the header goes out
	// only when the caller asked for that dimension. Sending it on every
	// request would opt every reader of this report into a beta they did not
	// ask for, and betas change without a deprecation period.
	var opts []reqOption
	if len(p.Speeds) > 0 || slices.Contains(p.GroupBy, "speed") {
		opts = append(opts, withBeta(BetaFastMode))
	}
	return listToken[UsageBucket](ctx, c, CredAdmin, orgPath+"/usage_report/messages", q, opts...)
}

// GetCostReport returns every daily bucket of the cost report.
func (c *Client) GetCostReport(ctx context.Context, p CostReportParams) ([]CostBucket, error) {
	q := url.Values{}
	q.Set("starting_at", p.StartingAt)
	if p.EndingAt != "" {
		q.Set("ending_at", p.EndingAt)
	}
	q.Set("bucket_width", "1d")
	addAll(q, "group_by[]", p.GroupBy)
	return listToken[CostBucket](ctx, c, CredAdmin, orgPath+"/cost_report", q)
}

// GetClaudeCodeUsageReport returns every actor row for one UTC day (YYYY-MM-DD).
func (c *Client) GetClaudeCodeUsageReport(ctx context.Context, date string) ([]ClaudeCodeUsage, error) {
	q := url.Values{}
	q.Set("starting_at", date)
	return listToken[ClaudeCodeUsage](ctx, c, CredAdmin, orgPath+"/usage_report/claude_code", q)
}

// --- enterprise analytics -----------------------------------------------------

// AnalyticsSummariesParams filters GetAnalyticsSummaries.
type AnalyticsSummariesParams struct {
	StartingDate string
	EndingDate   string
	RBACGroupIDs []string
}

// GetAnalyticsSummaries returns daily organization engagement summaries.
func (c *Client) GetAnalyticsSummaries(ctx context.Context, p AnalyticsSummariesParams) ([]AnalyticsSummary, error) {
	q := url.Values{}
	q.Set("starting_date", p.StartingDate)
	if p.EndingDate != "" {
		q.Set("ending_date", p.EndingDate)
	}
	for _, g := range p.RBACGroupIDs {
		q.Add("filter[]", "rbac_group_id:"+g)
	}
	var out struct {
		Summaries []AnalyticsSummary `json:"summaries"`
	}
	if err := c.get(ctx, CredAnalytics, orgPath+"/analytics/summaries", q, &out); err != nil {
		return nil, err
	}
	return out.Summaries, nil
}

// --- compliance directory -----------------------------------------------------

const compliancePath = "/v1/compliance"

// ListComplianceOrganizations lists linked organizations under the parent.
func (c *Client) ListComplianceOrganizations(ctx context.Context) ([]ComplianceOrganization, error) {
	return listToken[ComplianceOrganization](ctx, c, CredCompliance, compliancePath+"/organizations", nil)
}

// ListComplianceUsers lists members of one linked organization.
func (c *Client) ListComplianceUsers(ctx context.Context, orgUUID string) ([]ComplianceUser, error) {
	return listToken[ComplianceUser](ctx, c, CredCompliance, compliancePath+"/organizations/"+url.PathEscape(orgUUID)+"/users", nil)
}

// ListComplianceRoles lists custom roles of one linked organization.
func (c *Client) ListComplianceRoles(ctx context.Context, orgUUID string) ([]ComplianceRole, error) {
	return listToken[ComplianceRole](ctx, c, CredCompliance, compliancePath+"/organizations/"+url.PathEscape(orgUUID)+"/roles", nil)
}

// GetComplianceRole returns one role.
func (c *Client) GetComplianceRole(ctx context.Context, orgUUID, roleID string) (*ComplianceRole, error) {
	var out ComplianceRole
	if err := c.get(ctx, CredCompliance, compliancePath+"/organizations/"+url.PathEscape(orgUUID)+"/roles/"+url.PathEscape(roleID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListComplianceRolePermissions lists a role's permissions.
func (c *Client) ListComplianceRolePermissions(ctx context.Context, orgUUID, roleID string) ([]RBACRolePermission, error) {
	return listToken[RBACRolePermission](ctx, c, CredCompliance, compliancePath+"/organizations/"+url.PathEscape(orgUUID)+"/roles/"+url.PathEscape(roleID)+"/permissions", nil)
}

// ListComplianceGroups lists RBAC and SCIM groups across the parent tree.
func (c *Client) ListComplianceGroups(ctx context.Context) ([]ComplianceGroup, error) {
	return listToken[ComplianceGroup](ctx, c, CredCompliance, compliancePath+"/groups", nil)
}

// GetComplianceGroup returns one group.
func (c *Client) GetComplianceGroup(ctx context.Context, id string) (*ComplianceGroup, error) {
	var out ComplianceGroup
	if err := c.get(ctx, CredCompliance, compliancePath+"/groups/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListComplianceGroupMembers lists a group's members.
func (c *Client) ListComplianceGroupMembers(ctx context.Context, id string) ([]ComplianceGroupMember, error) {
	return listToken[ComplianceGroupMember](ctx, c, CredCompliance, compliancePath+"/groups/"+url.PathEscape(id)+"/members", nil)
}

// GetEffectiveOrganizationSettings returns the resolved settings of a linked organization.
func (c *Client) GetEffectiveOrganizationSettings(ctx context.Context, orgUUID string) (*EffectiveOrganizationSettings, error) {
	var out EffectiveOrganizationSettings
	if err := c.get(ctx, CredCompliance, compliancePath+"/organizations/"+url.PathEscape(orgUUID)+"/settings", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
