package client

import "encoding/json"

// --- spend limit increase requests ------------------------------------------

// RequestActor is the actor on an increase request (user or scoped API key).
type RequestActor struct {
	Type           string  `json:"type"`
	UserID         *string `json:"user_id,omitempty"`
	Name           *string `json:"name,omitempty"`
	EmailAddress   *string `json:"email_address,omitempty"`
	Deleted        *bool   `json:"deleted,omitempty"`
	ScopedAPIKeyID *string `json:"scoped_api_key_id,omitempty"`
}

// IncreaseRequestSpendSummary is the snapshot attached to a pending request.
type IncreaseRequestSpendSummary struct {
	Actor             RequestActor `json:"actor"`
	Amount            *string      `json:"amount"`
	Currency          string       `json:"currency"`
	Period            string       `json:"period"`
	PeriodToDateSpend string       `json:"period_to_date_spend"`
}

// SpendLimitIncreaseRequest is a member's request for a higher limit.
type SpendLimitIncreaseRequest struct {
	ID           string                       `json:"id"`
	Actor        RequestActor                 `json:"actor"`
	CreatedAt    string                       `json:"created_at"`
	Period       string                       `json:"period"`
	ResolvedAt   *string                      `json:"resolved_at"`
	ResolvedBy   *RequestActor                `json:"resolved_by"`
	SpendSummary *IncreaseRequestSpendSummary `json:"spend_summary"`
	Status       string                       `json:"status"`
	Type         string                       `json:"type"`
	UpdatedAt    *string                      `json:"updated_at,omitempty"`
}

// --- usage report -----------------------------------------------------------

// CacheCreation splits cache-creation tokens by TTL.
type CacheCreation struct {
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
}

// ServerToolUse counts server-side tool calls.
type ServerToolUse struct {
	WebSearchRequests int64 `json:"web_search_requests"`
}

// UsageResult is one row inside a usage bucket.
type UsageResult struct {
	AccountID            *string       `json:"account_id"`
	APIKeyID             *string       `json:"api_key_id"`
	CacheCreation        CacheCreation `json:"cache_creation"`
	CacheReadInputTokens int64         `json:"cache_read_input_tokens"`
	ContextWindow        *string       `json:"context_window"`
	InferenceGeo         *string       `json:"inference_geo"`
	Model                *string       `json:"model"`
	OutputTokens         int64         `json:"output_tokens"`
	ServerToolUse        ServerToolUse `json:"server_tool_use"`
	ServiceAccountID     *string       `json:"service_account_id"`
	ServiceTier          *string       `json:"service_tier"`
	Speed                *string       `json:"speed,omitempty"`
	UncachedInputTokens  int64         `json:"uncached_input_tokens"`
	WorkspaceID          *string       `json:"workspace_id"`
}

// UsageBucket is one time bucket of the usage report.
type UsageBucket struct {
	StartingAt string        `json:"starting_at"`
	EndingAt   string        `json:"ending_at"`
	Results    []UsageResult `json:"results"`
}

// BetaFastMode gates the `speed` dimension on the messages usage report. It
// lives here rather than with the Managed Agents betas because fast mode is a
// property of inference, not of agents.
const BetaFastMode = "fast-mode-2026-02-01"

// UsageReportParams filters GetUsageReport.
type UsageReportParams struct {
	StartingAt        string
	EndingAt          string
	BucketWidth       string
	GroupBy           []string
	APIKeyIDs         []string
	WorkspaceIDs      []string
	Models            []string
	ServiceTiers      []string
	ContextWindows    []string
	InferenceGeos     []string
	ServiceAccountIDs []string
	AccountIDs        []string
	// Speeds filters to `standard` or `fast`. Setting it, or grouping by
	// `speed`, sends the fast-mode beta header; see GetUsageReport.
	Speeds []string
}

// --- cost report ------------------------------------------------------------

// CostResult is one row inside a cost bucket.
type CostResult struct {
	Amount        string  `json:"amount"`
	ContextWindow *string `json:"context_window"`
	CostType      *string `json:"cost_type"`
	Currency      string  `json:"currency"`
	Description   *string `json:"description"`
	InferenceGeo  *string `json:"inference_geo"`
	Model         *string `json:"model"`
	ServiceTier   *string `json:"service_tier"`
	TokenType     *string `json:"token_type"`
	WorkspaceID   *string `json:"workspace_id"`
}

// CostBucket is one daily bucket of the cost report.
type CostBucket struct {
	StartingAt string       `json:"starting_at"`
	EndingAt   string       `json:"ending_at"`
	Results    []CostResult `json:"results"`
}

// CostReportParams filters GetCostReport.
type CostReportParams struct {
	StartingAt string
	EndingAt   string
	GroupBy    []string
}

// --- claude code usage ------------------------------------------------------

// ClaudeCodeActor is the user or API key behind a Claude Code row.
type ClaudeCodeActor struct {
	Type         string  `json:"type"`
	EmailAddress *string `json:"email_address,omitempty"`
	APIKeyName   *string `json:"api_key_name,omitempty"`
}

// LinesOfCode is added/removed line counts.
type LinesOfCode struct {
	Added   int64 `json:"added"`
	Removed int64 `json:"removed"`
}

// ClaudeCodeCoreMetrics is the productivity block.
type ClaudeCodeCoreMetrics struct {
	CommitsByClaudeCode      int64       `json:"commits_by_claude_code"`
	LinesOfCode              LinesOfCode `json:"lines_of_code"`
	NumSessions              int64       `json:"num_sessions"`
	PullRequestsByClaudeCode int64       `json:"pull_requests_by_claude_code"`
}

// ToolActionCounts is accepted/rejected counts for one tool.
type ToolActionCounts struct {
	Accepted int64 `json:"accepted"`
	Rejected int64 `json:"rejected"`
}

// ModelTokens is the token breakdown for one model.
type ModelTokens struct {
	CacheCreation int64 `json:"cache_creation"`
	CacheRead     int64 `json:"cache_read"`
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
}

// EstimatedCost is a minor-unit amount with currency.
type EstimatedCost struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// ModelBreakdown is per-model usage and estimated cost.
type ModelBreakdown struct {
	EstimatedCost EstimatedCost `json:"estimated_cost"`
	Model         string        `json:"model"`
	Tokens        ModelTokens   `json:"tokens"`
}

// ClaudeCodeUsage is one actor-day row of the Claude Code report.
type ClaudeCodeUsage struct {
	Actor            ClaudeCodeActor             `json:"actor"`
	CoreMetrics      ClaudeCodeCoreMetrics       `json:"core_metrics"`
	CustomerType     string                      `json:"customer_type"`
	Date             string                      `json:"date"`
	IsRemote         bool                        `json:"is_remote"`
	ModelBreakdown   []ModelBreakdown            `json:"model_breakdown"`
	OrganizationID   string                      `json:"organization_id"`
	SubscriptionType *string                     `json:"subscription_type"`
	TerminalType     string                      `json:"terminal_type"`
	ToolActions      map[string]ToolActionCounts `json:"tool_actions"`
}

// --- enterprise analytics ---------------------------------------------------

// AnalyticsSummary is one day of organization-wide engagement metrics.
type AnalyticsSummary struct {
	StartingAt                         string  `json:"starting_at"`
	EndingAt                           string  `json:"ending_at"`
	AssignedSeatCount                  int64   `json:"assigned_seat_count"`
	PendingInviteCount                 int64   `json:"pending_invite_count"`
	DailyActiveUserCount               int64   `json:"daily_active_user_count"`
	WeeklyActiveUserCount              int64   `json:"weekly_active_user_count"`
	MonthlyActiveUserCount             int64   `json:"monthly_active_user_count"`
	DailyAdoptionRate                  float64 `json:"daily_adoption_rate"`
	WeeklyAdoptionRate                 float64 `json:"weekly_adoption_rate"`
	MonthlyAdoptionRate                float64 `json:"monthly_adoption_rate"`
	ChatDailyActiveUserCount           int64   `json:"chat_daily_active_user_count"`
	ChatWeeklyActiveUserCount          int64   `json:"chat_weekly_active_user_count"`
	ChatMonthlyActiveUserCount         int64   `json:"chat_monthly_active_user_count"`
	ClaudeCodeDailyActiveUserCount     int64   `json:"claude_code_daily_active_user_count"`
	ClaudeCodeWeeklyActiveUserCount    int64   `json:"claude_code_weekly_active_user_count"`
	ClaudeCodeMonthlyActiveUserCount   int64   `json:"claude_code_monthly_active_user_count"`
	CoworkDailyActiveUserCount         int64   `json:"cowork_daily_active_user_count"`
	CoworkWeeklyActiveUserCount        int64   `json:"cowork_weekly_active_user_count"`
	CoworkMonthlyActiveUserCount       int64   `json:"cowork_monthly_active_user_count"`
	ClaudeDesignDailyActiveUserCount   int64   `json:"claude_design_daily_active_user_count"`
	ClaudeDesignWeeklyActiveUserCount  int64   `json:"claude_design_weekly_active_user_count"`
	ClaudeDesignMonthlyActiveUserCount int64   `json:"claude_design_monthly_active_user_count"`
	OfficeAgentDailyActiveUserCount    int64   `json:"office_agent_daily_active_user_count"`
	OfficeAgentWeeklyActiveUserCount   int64   `json:"office_agent_weekly_active_user_count"`
	OfficeAgentMonthlyActiveUserCount  int64   `json:"office_agent_monthly_active_user_count"`
	ScienceDailyActiveUserCount        int64   `json:"science_daily_active_user_count"`
	ScienceWeeklyActiveUserCount       int64   `json:"science_weekly_active_user_count"`
	ScienceMonthlyActiveUserCount      int64   `json:"science_monthly_active_user_count"`
	ScienceEntitledUserCount           int64   `json:"science_entitled_user_count"`
}

// --- compliance directory ---------------------------------------------------

// ComplianceOrganization is a linked organization under the parent.
type ComplianceOrganization struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// ComplianceUser is a member row from the compliance directory.
type ComplianceUser struct {
	ID               string `json:"id"`
	FullName         string `json:"full_name"`
	Email            string `json:"email"`
	OrganizationRole string `json:"organization_role"`
	CreatedAt        string `json:"created_at"`
}

// ComplianceRole is a custom role from the compliance directory.
type ComplianceRole struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ComplianceGroup is an RBAC or SCIM group with its role ids.
type ComplianceGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	SourceType  string   `json:"source_type"`
	Roles       []string `json:"roles"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// ComplianceGroupMember is a member row of a compliance group.
type ComplianceGroupMember struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// EffectiveSetting is one typed settings row.
type EffectiveSetting struct {
	Name  string          `json:"name"`
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// ComplianceAPIKeyInfo describes a Compliance Access Key (never its secret).
type ComplianceAPIKeyInfo struct {
	Type        string   `json:"type"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	IsActive    bool     `json:"is_active"`
	CreatedAt   string   `json:"created_at"`
	CreatedByID *string  `json:"created_by_id"`
	ExpiresAt   *string  `json:"expires_at"`
}

// EffectiveOrganizationSettings is the resolved settings for one organization.
type EffectiveOrganizationSettings struct {
	Type           string                 `json:"type"`
	OrganizationID string                 `json:"organization_id"`
	Settings       []EffectiveSetting     `json:"settings"`
	APIKeys        []ComplianceAPIKeyInfo `json:"api_keys"`
}
