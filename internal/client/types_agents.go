package client

import "encoding/json"

// Beta header values for the Managed Agents control plane.
const (
	BetaManagedAgents = "managed-agents-2026-04-01"
	BetaAgentMemory   = "agent-memory-2026-07-22"
)

// --- shared ----------------------------------------------------------------------

// MetadataPatch is the update form of metadata: nil value deletes the key.
type MetadataPatch map[string]*string

// TypeOnly is a {"type": "..."} object.
type TypeOnly struct {
	Type string `json:"type"`
}

// --- agents ------------------------------------------------------------------

// ModelConfig is the resolved model block on an agent.
type ModelConfig struct {
	ID           string    `json:"id"`
	Effort       *TypeOnly `json:"effort,omitempty"`
	InferenceGeo *string   `json:"inference_geo,omitempty"`
	Speed        *string   `json:"speed,omitempty"`
}

// ModelParams is the request form of the model block. When only ID is set it
// marshals as the bare model string.
type ModelParams struct {
	ID           string  `json:"id"`
	Effort       *string `json:"effort,omitempty"`
	InferenceGeo *string `json:"inference_geo,omitempty"`
	Speed        *string `json:"speed,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (m ModelParams) MarshalJSON() ([]byte, error) {
	if m.Effort == nil && m.InferenceGeo == nil && m.Speed == nil {
		return json.Marshal(m.ID)
	}
	type plain ModelParams
	return json.Marshal(plain(m))
}

// MCPServer is an agent's MCP server definition.
type MCPServer struct {
	Type string `json:"type"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// SkillRef references an Anthropic or custom skill.
type SkillRef struct {
	Type    string  `json:"type"`
	SkillID string  `json:"skill_id"`
	Version *string `json:"version,omitempty"`
}

// Agent is a Managed Agents agent definition (one version).
type Agent struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	System      *string           `json:"system"`
	Model       ModelConfig       `json:"model"`
	Tools       json.RawMessage   `json:"tools"`
	MCPServers  []MCPServer       `json:"mcp_servers"`
	Skills      []SkillRef        `json:"skills"`
	Multiagent  json.RawMessage   `json:"multiagent"`
	Metadata    map[string]string `json:"metadata"`
	Version     int64             `json:"version"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ArchivedAt  *string           `json:"archived_at"`
}

// AgentCreate is the create body.
type AgentCreate struct {
	Name        string            `json:"name"`
	Model       ModelParams       `json:"model"`
	Description *string           `json:"description,omitempty"`
	System      *string           `json:"system,omitempty"`
	Tools       json.RawMessage   `json:"tools,omitempty"`
	MCPServers  []MCPServer       `json:"mcp_servers,omitempty"`
	Skills      []SkillRef        `json:"skills,omitempty"`
	Multiagent  json.RawMessage   `json:"multiagent,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// AgentUpdate is the update body. Slices are full replacements; a non-nil
// empty slice clears. Version enables optimistic concurrency when set.
type AgentUpdate struct {
	Name        *string         `json:"name,omitempty"`
	Model       *ModelParams    `json:"model,omitempty"`
	Description Opt[string]     `json:"description,omitzero"`
	System      Opt[string]     `json:"system,omitzero"`
	Tools       json.RawMessage `json:"tools,omitempty"`
	MCPServers  *[]MCPServer    `json:"mcp_servers,omitempty"`
	Skills      *[]SkillRef     `json:"skills,omitempty"`
	Multiagent  json.RawMessage `json:"multiagent,omitempty"`
	Metadata    MetadataPatch   `json:"metadata,omitempty"`
	Version     *int64          `json:"version,omitempty"`
}

// --- environments -----------------------------------------------------------------

// Networking is the cloud environment network policy.
type Networking struct {
	Type                 string   `json:"type"`
	AllowedHosts         []string `json:"allowed_hosts,omitempty"`
	AllowMCPServers      *bool    `json:"allow_mcp_servers,omitempty"`
	AllowPackageManagers *bool    `json:"allow_package_managers,omitempty"`
}

// Packages lists pre-installed packages per ecosystem.
type Packages struct {
	Type  string   `json:"type,omitempty"`
	Apt   []string `json:"apt,omitempty"`
	Cargo []string `json:"cargo,omitempty"`
	Gem   []string `json:"gem,omitempty"`
	Go    []string `json:"go,omitempty"`
	Npm   []string `json:"npm,omitempty"`
	Pip   []string `json:"pip,omitempty"`
}

// EnvironmentConfig is the environment config union (cloud | self_hosted).
type EnvironmentConfig struct {
	Type       string      `json:"type"`
	Networking *Networking `json:"networking,omitempty"`
	Packages   *Packages   `json:"packages,omitempty"`
}

// Environment is a session workspace definition.
type Environment struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	Config      EnvironmentConfig `json:"config"`
	Metadata    map[string]string `json:"metadata"`
	Scope       string            `json:"scope"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ArchivedAt  *string           `json:"archived_at"`
}

// EnvironmentCreate is the create body.
type EnvironmentCreate struct {
	Name        string            `json:"name"`
	Description *string           `json:"description,omitempty"`
	Config      EnvironmentConfig `json:"config"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Scope       *string           `json:"scope,omitempty"`
}

// EnvironmentUpdate is the update body (config is a partial merge).
type EnvironmentUpdate struct {
	Name        *string            `json:"name,omitempty"`
	Description Opt[string]        `json:"description,omitzero"`
	Config      *EnvironmentConfig `json:"config,omitempty"`
	Metadata    MetadataPatch      `json:"metadata,omitempty"`
}

// --- vaults ----------------------------------------------------------------------

// Vault groups credentials.
type Vault struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ArchivedAt  *string           `json:"archived_at"`
}

// VaultCreate is the create body.
type VaultCreate struct {
	DisplayName string            `json:"display_name"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// VaultUpdate is the update body.
type VaultUpdate struct {
	DisplayName *string       `json:"display_name,omitempty"`
	Metadata    MetadataPatch `json:"metadata,omitempty"`
}

// VaultCredential is a stored credential; secret values are never returned.
type VaultCredential struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	VaultID     string            `json:"vault_id"`
	DisplayName *string           `json:"display_name"`
	Metadata    map[string]string `json:"metadata"`
	Auth        json.RawMessage   `json:"auth"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ArchivedAt  *string           `json:"archived_at"`
}

// VaultCredentialCreate is the create body; Auth is the union object.
type VaultCredentialCreate struct {
	DisplayName *string           `json:"display_name,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Auth        json.RawMessage   `json:"auth"`
}

// VaultCredentialUpdate is the update body.
type VaultCredentialUpdate struct {
	DisplayName Opt[string]     `json:"display_name,omitzero"`
	Metadata    MetadataPatch   `json:"metadata,omitempty"`
	Auth        json.RawMessage `json:"auth,omitempty"`
}

// --- deployments ----------------------------------------------------------------

// AgentRef pins an agent version.
type AgentRef struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version *int64 `json:"version,omitempty"`
}

// Schedule is a cron schedule.
type Schedule struct {
	Type       string `json:"type"`
	Expression string `json:"expression"`
	Timezone   string `json:"timezone"`
}

// Money is an amount in minor units with a currency.
type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

// Budget caps a deployment run.
type Budget struct {
	Type        string `json:"type"`
	MaxListCost Money  `json:"max_list_cost"`
}

// PausedReason explains why a deployment is paused.
type PausedReason struct {
	Type  string    `json:"type"`
	Error *TypeOnly `json:"error,omitempty"`
}

// Deployment is a scheduled or manual agent run definition.
type Deployment struct {
	ID            string            `json:"id"`
	Type          string            `json:"type"`
	Name          string            `json:"name"`
	Agent         AgentRef          `json:"agent"`
	EnvironmentID string            `json:"environment_id"`
	InitialEvents json.RawMessage   `json:"initial_events"`
	Schedule      *Schedule         `json:"schedule"`
	Resources     json.RawMessage   `json:"resources"`
	VaultIDs      []string          `json:"vault_ids"`
	Budget        *Budget           `json:"budget"`
	Description   *string           `json:"description"`
	Metadata      map[string]string `json:"metadata"`
	Status        string            `json:"status"`
	PausedReason  *PausedReason     `json:"paused_reason"`
	LastRunAt     *string           `json:"last_run_at"`
	UpcomingRuns  []string          `json:"upcoming_runs_at"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
	ArchivedAt    *string           `json:"archived_at"`
}

// DeploymentCreate is the create body.
type DeploymentCreate struct {
	Name          string            `json:"name"`
	Agent         AgentRef          `json:"agent"`
	EnvironmentID string            `json:"environment_id"`
	InitialEvents json.RawMessage   `json:"initial_events"`
	Schedule      *Schedule         `json:"schedule,omitempty"`
	Resources     json.RawMessage   `json:"resources,omitempty"`
	VaultIDs      []string          `json:"vault_ids,omitempty"`
	Budget        *Budget           `json:"budget,omitempty"`
	Description   *string           `json:"description,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// DeploymentUpdate is the update body.
type DeploymentUpdate struct {
	Name          *string         `json:"name,omitempty"`
	Agent         *AgentRef       `json:"agent,omitempty"`
	EnvironmentID *string         `json:"environment_id,omitempty"`
	InitialEvents json.RawMessage `json:"initial_events,omitempty"`
	Schedule      Opt[*Schedule]  `json:"schedule,omitzero"`
	Resources     json.RawMessage `json:"resources,omitempty"`
	VaultIDs      *[]string       `json:"vault_ids,omitempty"`
	Budget        Opt[*Budget]    `json:"budget,omitzero"`
	Description   Opt[string]     `json:"description,omitzero"`
	Metadata      MetadataPatch   `json:"metadata,omitempty"`
}

// --- memory stores ------------------------------------------------------------------

// MemoryStore is a persistent agent memory namespace.
type MemoryStore struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	ArchivedAt  *string           `json:"archived_at"`
}

// MemoryStoreCreate is the create body.
type MemoryStoreCreate struct {
	Name        string            `json:"name"`
	Description *string           `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// MemoryStoreUpdate is the update body.
type MemoryStoreUpdate struct {
	Name        *string       `json:"name,omitempty"`
	Description *string       `json:"description,omitempty"`
	Metadata    MetadataPatch `json:"metadata,omitempty"`
}

// --- skills ----------------------------------------------------------------------------

// Skill is an uploaded skill.
type Skill struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"`
	DisplayName     string   `json:"display_name"`
	LatestVersionID string   `json:"latest_version_id"`
	Source          TypeOnly `json:"source"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// SkillVersion is one immutable upload of a skill.
type SkillVersion struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	SkillID     string `json:"skill_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}
