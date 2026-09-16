package client

// --- per-user activity --------------------------------------------------------

// AnalyticsUserRef identifies the user of an activity row.
type AnalyticsUserRef struct {
	Type         string `json:"type"`
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

// ChatMetrics are claude.ai chat counters.
type ChatMetrics struct {
	ConnectorsUsedCount                int64 `json:"connectors_used_count"`
	DistinctArtifactsCreatedCount      int64 `json:"distinct_artifacts_created_count"`
	DistinctConnectorsUsedCount        int64 `json:"distinct_connectors_used_count"`
	DistinctConversationCount          int64 `json:"distinct_conversation_count"`
	DistinctFilesUploadedCount         int64 `json:"distinct_files_uploaded_count"`
	DistinctProjectsCreatedCount       int64 `json:"distinct_projects_created_count"`
	DistinctProjectsUsedCount          int64 `json:"distinct_projects_used_count"`
	DistinctSharedArtifactsViewedCount int64 `json:"distinct_shared_artifacts_viewed_count"`
	DistinctSkillsUsedCount            int64 `json:"distinct_skills_used_count"`
	MessageCount                       int64 `json:"message_count"`
	SharedConversationsViewedCount     int64 `json:"shared_conversations_viewed_count"`
	ThinkingMessageCount               int64 `json:"thinking_message_count"`
}

// CountPair is accepted/rejected counts for a Claude Code tool.
type CountPair struct {
	AcceptedCount int64 `json:"accepted_count"`
	RejectedCount int64 `json:"rejected_count"`
}

// ClaudeCodeMetrics are Claude Code counters on an activity row.
type ClaudeCodeMetrics struct {
	CoreMetrics struct {
		ArtifactsCreatedCount int64 `json:"artifacts_created_count"`
		CommitCount           int64 `json:"commit_count"`
		DistinctSessionCount  int64 `json:"distinct_session_count"`
		LinesOfCode           struct {
			AddedCount   int64 `json:"added_count"`
			RemovedCount int64 `json:"removed_count"`
		} `json:"lines_of_code"`
		PullRequestCount int64 `json:"pull_request_count"`
	} `json:"core_metrics"`
	ToolActions map[string]CountPair `json:"tool_actions"`
}

// CoworkMetrics are Cowork counters.
type CoworkMetrics struct {
	ActionCount                 int64 `json:"action_count"`
	ArtifactsCreatedCount       int64 `json:"artifacts_created_count"`
	ConnectorsUsedCount         int64 `json:"connectors_used_count"`
	DispatchTurnCount           int64 `json:"dispatch_turn_count"`
	DistinctConnectorsUsedCount int64 `json:"distinct_connectors_used_count"`
	DistinctSessionCount        int64 `json:"distinct_session_count"`
	DistinctSkillsUsedCount     int64 `json:"distinct_skills_used_count"`
	MessageCount                int64 `json:"message_count"`
	SkillsUsedCount             int64 `json:"skills_used_count"`
	DistinctPluginsUsedCount    int64 `json:"distinct_plugins_used_count"`
	EditToolCount               int64 `json:"edit_tool_count"`
	FileEditCount               int64 `json:"file_edit_count"`
	MultiEditToolCount          int64 `json:"multi_edit_tool_count"`
	NotebookEditToolCount       int64 `json:"notebook_edit_tool_count"`
	PluginsUsedCount            int64 `json:"plugins_used_count"`
	SessionsWithFileEditsCount  int64 `json:"sessions_with_file_edits_count"`
	WriteToolCount              int64 `json:"write_tool_count"`
}

// DesignMetrics are Claude Design counters.
type DesignMetrics struct {
	DistinctProjectsCreatedCount int64 `json:"distinct_projects_created_count"`
	DistinctProjectsUsedCount    int64 `json:"distinct_projects_used_count"`
	DistinctSessionCount         int64 `json:"distinct_session_count"`
	MessageCount                 int64 `json:"message_count"`
}

// OfficeProductMetrics are counters for one Office application.
type OfficeProductMetrics struct {
	ConnectorsUsedCount         int64 `json:"connectors_used_count"`
	DistinctConnectorsUsedCount int64 `json:"distinct_connectors_used_count"`
	DistinctSessionCount        int64 `json:"distinct_session_count"`
	DistinctSkillsUsedCount     int64 `json:"distinct_skills_used_count"`
	MessageCount                int64 `json:"message_count"`
	SkillsUsedCount             int64 `json:"skills_used_count"`
}

// OfficeMetrics groups Office Agent counters by application.
type OfficeMetrics struct {
	Excel      OfficeProductMetrics `json:"excel"`
	Outlook    OfficeProductMetrics `json:"outlook"`
	PowerPoint OfficeProductMetrics `json:"powerpoint"`
	Word       OfficeProductMetrics `json:"word"`
}

// ScienceMetrics are Claude for Science counters.
type ScienceMetrics struct {
	DelegationCount       int64 `json:"delegation_count"`
	DistinctSessionCount  int64 `json:"distinct_session_count"`
	MessageCount          int64 `json:"message_count"`
	RemoteComputeJobCount int64 `json:"remote_compute_job_count"`
	SkillsUsedCount       int64 `json:"skills_used_count"`
}

// AnalyticsUserActivity is one row of /analytics/users.
type AnalyticsUserActivity struct {
	User              *AnalyticsUserRef `json:"user"`
	ChatMetrics       ChatMetrics       `json:"chat_metrics"`
	ClaudeCodeMetrics ClaudeCodeMetrics `json:"claude_code_metrics"`
	CoworkMetrics     CoworkMetrics     `json:"cowork_metrics"`
	DesignMetrics     DesignMetrics     `json:"design_metrics"`
	OfficeMetrics     OfficeMetrics     `json:"office_metrics"`
	ScienceMetrics    ScienceMetrics    `json:"science_metrics"`
	WebSearchCount    int64             `json:"web_search_count"`
	DistinctUserCount *int64            `json:"distinct_user_count"`
	LastActivityDate  *string           `json:"last_activity_date"`
	RBACGroupID       *string           `json:"rbac_group_id"`
	RBACGroupName     *string           `json:"rbac_group_name"`
}

// --- skills / connectors / plugins / artifacts ---------------------------------

// SessionCount is a nullable distinct-session counter block.
type SessionCount struct {
	DistinctSessionSkillUsedCount     *int64 `json:"distinct_session_skill_used_count,omitempty"`
	DistinctSessionConnectorUsedCount *int64 `json:"distinct_session_connector_used_count,omitempty"`
	DistinctSessionPluginUsedCount    *int64 `json:"distinct_session_plugin_used_count,omitempty"`
}

// OfficeSessionCounts groups per-application session counters.
type OfficeSessionCounts struct {
	Excel      SessionCount `json:"excel"`
	Outlook    SessionCount `json:"outlook"`
	PowerPoint SessionCount `json:"powerpoint"`
	Word       SessionCount `json:"word"`
}

// AnalyticsSkillUsage is one row of /analytics/skills.
type AnalyticsSkillUsage struct {
	SkillName             string  `json:"skill_name"`
	SkillDisplayName      *string `json:"skill_display_name"`
	DistinctUserCount     int64   `json:"distinct_user_count"`
	InvocationCount       *int64  `json:"invocation_count"`
	EnableCount           *int64  `json:"enable_count"`
	EstimatedOverageSpend *string `json:"estimated_overage_spend"`
	AttributedListPrice   *string `json:"attributed_list_price"`
	Currency              *string `json:"currency"`
	ShareStatus           *string `json:"share_status"`
	ChatMetrics           struct {
		DistinctConversationSkillUsedCount *int64 `json:"distinct_conversation_skill_used_count"`
	} `json:"chat_metrics"`
	ClaudeCodeMetrics SessionCount        `json:"claude_code_metrics"`
	CoworkMetrics     SessionCount        `json:"cowork_metrics"`
	OfficeMetrics     OfficeSessionCounts `json:"office_metrics"`
	Product           *string             `json:"product"`
	RBACGroupID       *string             `json:"rbac_group_id"`
	RBACGroupName     *string             `json:"rbac_group_name"`
	UserID            *string             `json:"user_id"`
}

// AnalyticsConnectorUsage is one row of /analytics/connectors.
type AnalyticsConnectorUsage struct {
	ConnectorName                   string  `json:"connector_name"`
	ConnectorDisplayName            *string `json:"connector_display_name"`
	DistinctUserCount               int64   `json:"distinct_user_count"`
	IndividualAuthDistinctUserCount *int64  `json:"individual_auth_distinct_user_count"`
	ManagedAuthDistinctUserCount    *int64  `json:"managed_auth_distinct_user_count"`
	ReadCallCount                   *int64  `json:"read_call_count"`
	WriteCallCount                  *int64  `json:"write_call_count"`
	UnclassifiedCallCount           *int64  `json:"unclassified_call_count"`
	ChatMetrics                     struct {
		DistinctConversationConnectorUsedCount *int64 `json:"distinct_conversation_connector_used_count"`
	} `json:"chat_metrics"`
	ClaudeCodeMetrics SessionCount        `json:"claude_code_metrics"`
	CoworkMetrics     SessionCount        `json:"cowork_metrics"`
	OfficeMetrics     OfficeSessionCounts `json:"office_metrics"`
	Product           *string             `json:"product"`
	RBACGroupID       *string             `json:"rbac_group_id"`
	RBACGroupName     *string             `json:"rbac_group_name"`
	UserID            *string             `json:"user_id"`
}

// AnalyticsPluginUsage is one row of /analytics/plugins.
type AnalyticsPluginUsage struct {
	PluginName        string       `json:"plugin_name"`
	PluginID          *string      `json:"plugin_id"`
	DistinctUserCount int64        `json:"distinct_user_count"`
	InstallCount      *int64       `json:"install_count"`
	InvocationCount   int64        `json:"invocation_count"`
	ClaudeCodeMetrics SessionCount `json:"claude_code_metrics"`
	CoworkMetrics     SessionCount `json:"cowork_metrics"`
	Product           *string      `json:"product"`
	RBACGroupID       *string      `json:"rbac_group_id"`
	RBACGroupName     *string      `json:"rbac_group_name"`
	UserID            *string      `json:"user_id"`
}

// AnalyticsArtifactUsage is one row of /analytics/artifacts.
type AnalyticsArtifactUsage struct {
	ArtifactType                   string  `json:"artifact_type"`
	IsShared                       bool    `json:"is_shared"`
	ArtifactsCreatedCount          int64   `json:"artifacts_created_count"`
	PublishedArtifactsCreatedCount int64   `json:"published_artifacts_created_count"`
	DistinctUserCount              int64   `json:"distinct_user_count"`
	Product                        *string `json:"product"`
	RBACGroupID                    *string `json:"rbac_group_id"`
	RBACGroupName                  *string `json:"rbac_group_name"`
	UserID                         *string `json:"user_id"`
}

// --- enterprise usage / cost -----------------------------------------------------

// AnalyticsDims are the nullable group-by dimensions on usage/cost rows.
type AnalyticsDims struct {
	Model             *string `json:"model"`
	Product           *string `json:"product"`
	ContextWindow     *string `json:"context_window"`
	InferenceGeo      *string `json:"inference_geo"`
	Speed             *string `json:"speed"`
	RBACGroupID       *string `json:"rbac_group_id"`
	ClaudeTagCategory *string `json:"claude_tag_category"`
	ClaudeTagUserID   *string `json:"claude_tag_user_id"`
	SlackChannelID    *string `json:"slack_channel_id"`
}

// AnalyticsUsageResult is one row inside an Enterprise usage bucket.
type AnalyticsUsageResult struct {
	AnalyticsDims
	CacheCreation        CacheCreation `json:"cache_creation"`
	CacheReadInputTokens int64         `json:"cache_read_input_tokens"`
	UncachedInputTokens  int64         `json:"uncached_input_tokens"`
	OutputTokens         int64         `json:"output_tokens"`
	Requests             *int64        `json:"requests"`
	ServerToolUse        ServerToolUse `json:"server_tool_use"`
}

// AnalyticsUsageBucket is one time bucket of the Enterprise usage report.
type AnalyticsUsageBucket struct {
	StartingAt string                 `json:"starting_at"`
	EndingAt   string                 `json:"ending_at"`
	Results    []AnalyticsUsageResult `json:"results"`
}

// AnalyticsCostResult is one row inside an Enterprise cost bucket.
type AnalyticsCostResult struct {
	AnalyticsDims
	Amount     string  `json:"amount"`
	ListAmount string  `json:"list_amount"`
	Currency   string  `json:"currency"`
	Requests   *int64  `json:"requests"`
	CostType   *string `json:"cost_type"`
	TokenType  *string `json:"token_type"`
}

// AnalyticsCostBucket is one time bucket of the Enterprise cost report.
type AnalyticsCostBucket struct {
	StartingAt string                `json:"starting_at"`
	EndingAt   string                `json:"ending_at"`
	Results    []AnalyticsCostResult `json:"results"`
}

// AnalyticsActor is the user on per-user usage/cost rows.
type AnalyticsActor struct {
	Type    string  `json:"type"`
	UserID  string  `json:"user_id"`
	Email   *string `json:"email"`
	Name    *string `json:"name"`
	Deleted bool    `json:"deleted"`
}

// AnalyticsUserUsageRow is one row of /analytics/user_usage_report.
type AnalyticsUserUsageRow struct {
	Actor      AnalyticsActor `json:"actor"`
	StartingAt *string        `json:"starting_at"`
	EndingAt   *string        `json:"ending_at"`
	AnalyticsUsageResult
	TotalTokens int64 `json:"total_tokens"`
}

// AnalyticsUserCostRow is one row of /analytics/user_cost_report.
type AnalyticsUserCostRow struct {
	Actor      AnalyticsActor `json:"actor"`
	StartingAt *string        `json:"starting_at"`
	EndingAt   *string        `json:"ending_at"`
	AnalyticsCostResult
}

// AnalyticsReport wraps a paginated analytics report with its refresh metadata.
type AnalyticsReport[T any] struct {
	Data            []T
	DataRefreshedAt *string
	OrganizationID  string
}
