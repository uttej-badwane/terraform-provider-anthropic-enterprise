package client

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

var (
	betaAgents = withBeta(BetaManagedAgents)
	betaMemory = withBeta(BetaAgentMemory)
)

func archivedQuery(includeArchived bool) url.Values {
	q := url.Values{}
	if includeArchived {
		q.Set("include_archived", "true")
	}
	return q
}

// --- agents ------------------------------------------------------------------

// ListAgents lists agent definitions.
func (c *Client) ListAgents(ctx context.Context, includeArchived bool) ([]Agent, error) {
	return listToken[Agent](ctx, c, CredAPIKey, "/v1/agents", archivedQuery(includeArchived), betaAgents)
}

// CreateAgent creates an agent (version 1).
func (c *Client) CreateAgent(ctx context.Context, in AgentCreate) (*Agent, error) {
	var out Agent
	if err := c.create(ctx, CredAPIKey, "/v1/agents", in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAgent returns the current version of an agent.
func (c *Client) GetAgent(ctx context.Context, id string) (*Agent, error) {
	var out Agent
	if err := c.get(ctx, CredAPIKey, "/v1/agents/"+url.PathEscape(id), nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateAgent updates an agent; an effective change creates a new version.
func (c *Client) UpdateAgent(ctx context.Context, id string, in AgentUpdate) (*Agent, error) {
	var out Agent
	if err := c.post(ctx, CredAPIKey, "/v1/agents/"+url.PathEscape(id), in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveAgent archives an agent (terminal).
func (c *Client) ArchiveAgent(ctx context.Context, id string) (*Agent, error) {
	var out Agent
	if err := c.post(ctx, CredAPIKey, "/v1/agents/"+url.PathEscape(id)+"/archive", nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAgentVersions lists every version of an agent.
func (c *Client) ListAgentVersions(ctx context.Context, id string) ([]Agent, error) {
	return listToken[Agent](ctx, c, CredAPIKey, "/v1/agents/"+url.PathEscape(id)+"/versions", nil, betaAgents)
}

// --- environments -----------------------------------------------------------------

// ListEnvironments lists environments.
func (c *Client) ListEnvironments(ctx context.Context, includeArchived bool) ([]Environment, error) {
	return listToken[Environment](ctx, c, CredAPIKey, "/v1/environments", archivedQuery(includeArchived), betaAgents)
}

// CreateEnvironment creates an environment.
func (c *Client) CreateEnvironment(ctx context.Context, in EnvironmentCreate) (*Environment, error) {
	var out Environment
	if err := c.create(ctx, CredAPIKey, "/v1/environments", in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEnvironment returns one environment.
func (c *Client) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	var out Environment
	if err := c.get(ctx, CredAPIKey, "/v1/environments/"+url.PathEscape(id), nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateEnvironment updates an environment (config partial merge).
func (c *Client) UpdateEnvironment(ctx context.Context, id string, in EnvironmentUpdate) (*Environment, error) {
	var out Environment
	if err := c.post(ctx, CredAPIKey, "/v1/environments/"+url.PathEscape(id), in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEnvironment deletes an environment.
func (c *Client) DeleteEnvironment(ctx context.Context, id string) error {
	return c.delete(ctx, CredAPIKey, "/v1/environments/"+url.PathEscape(id), betaAgents)
}

// ArchiveEnvironment archives an environment (terminal).
func (c *Client) ArchiveEnvironment(ctx context.Context, id string) (*Environment, error) {
	var out Environment
	if err := c.post(ctx, CredAPIKey, "/v1/environments/"+url.PathEscape(id)+"/archive", nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- vaults --------------------------------------------------------------------------

// ListVaults lists vaults.
func (c *Client) ListVaults(ctx context.Context, includeArchived bool) ([]Vault, error) {
	return listToken[Vault](ctx, c, CredAPIKey, "/v1/vaults", archivedQuery(includeArchived), betaAgents)
}

// CreateVault creates a vault.
func (c *Client) CreateVault(ctx context.Context, in VaultCreate) (*Vault, error) {
	var out Vault
	if err := c.create(ctx, CredAPIKey, "/v1/vaults", in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVault returns one vault.
func (c *Client) GetVault(ctx context.Context, id string) (*Vault, error) {
	var out Vault
	if err := c.get(ctx, CredAPIKey, "/v1/vaults/"+url.PathEscape(id), nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateVault updates a vault.
func (c *Client) UpdateVault(ctx context.Context, id string, in VaultUpdate) (*Vault, error) {
	var out Vault
	if err := c.post(ctx, CredAPIKey, "/v1/vaults/"+url.PathEscape(id), in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteVault hard-deletes a vault and its credentials.
func (c *Client) DeleteVault(ctx context.Context, id string) error {
	return c.delete(ctx, CredAPIKey, "/v1/vaults/"+url.PathEscape(id), betaAgents)
}

// ArchiveVault archives a vault and purges its secrets (terminal).
func (c *Client) ArchiveVault(ctx context.Context, id string) (*Vault, error) {
	var out Vault
	if err := c.post(ctx, CredAPIKey, "/v1/vaults/"+url.PathEscape(id)+"/archive", nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

func credentialPath(vaultID, id string) string {
	return "/v1/vaults/" + url.PathEscape(vaultID) + "/credentials/" + url.PathEscape(id)
}

// ListVaultCredentials lists a vault's credentials (no secret values).
func (c *Client) ListVaultCredentials(ctx context.Context, vaultID string, includeArchived bool) ([]VaultCredential, error) {
	return listToken[VaultCredential](ctx, c, CredAPIKey, "/v1/vaults/"+url.PathEscape(vaultID)+"/credentials", archivedQuery(includeArchived), betaAgents)
}

// CreateVaultCredential stores a credential.
func (c *Client) CreateVaultCredential(ctx context.Context, vaultID string, in VaultCredentialCreate) (*VaultCredential, error) {
	var out VaultCredential
	if err := c.create(ctx, CredAPIKey, "/v1/vaults/"+url.PathEscape(vaultID)+"/credentials", in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVaultCredential returns one credential (no secret values).
func (c *Client) GetVaultCredential(ctx context.Context, vaultID, id string) (*VaultCredential, error) {
	var out VaultCredential
	if err := c.get(ctx, CredAPIKey, credentialPath(vaultID, id), nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateVaultCredential rotates secrets or updates mutable fields.
func (c *Client) UpdateVaultCredential(ctx context.Context, vaultID, id string, in VaultCredentialUpdate) (*VaultCredential, error) {
	var out VaultCredential
	if err := c.post(ctx, CredAPIKey, credentialPath(vaultID, id), in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteVaultCredential hard-deletes a credential.
func (c *Client) DeleteVaultCredential(ctx context.Context, vaultID, id string) error {
	return c.delete(ctx, CredAPIKey, credentialPath(vaultID, id), betaAgents)
}

// ArchiveVaultCredential archives a credential and purges its secret.
func (c *Client) ArchiveVaultCredential(ctx context.Context, vaultID, id string) (*VaultCredential, error) {
	var out VaultCredential
	if err := c.post(ctx, CredAPIKey, credentialPath(vaultID, id)+"/archive", nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- deployments ----------------------------------------------------------------

// DeploymentListOptions filters ListDeployments.
type DeploymentListOptions struct {
	AgentID         string
	Status          string
	IncludeArchived bool
}

// ListDeployments lists deployments.
func (c *Client) ListDeployments(ctx context.Context, opts DeploymentListOptions) ([]Deployment, error) {
	q := archivedQuery(opts.IncludeArchived)
	if opts.AgentID != "" {
		q.Set("agent_id", opts.AgentID)
	}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	return listToken[Deployment](ctx, c, CredAPIKey, "/v1/deployments", q, betaAgents)
}

// CreateDeployment creates a deployment.
func (c *Client) CreateDeployment(ctx context.Context, in DeploymentCreate) (*Deployment, error) {
	var out Deployment
	if err := c.create(ctx, CredAPIKey, "/v1/deployments", in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDeployment returns one deployment.
func (c *Client) GetDeployment(ctx context.Context, id string) (*Deployment, error) {
	var out Deployment
	if err := c.get(ctx, CredAPIKey, "/v1/deployments/"+url.PathEscape(id), nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateDeployment updates a deployment.
func (c *Client) UpdateDeployment(ctx context.Context, id string, in DeploymentUpdate) (*Deployment, error) {
	var out Deployment
	if err := c.post(ctx, CredAPIKey, "/v1/deployments/"+url.PathEscape(id), in, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// PauseDeployment pauses scheduled runs.
func (c *Client) PauseDeployment(ctx context.Context, id string) (*Deployment, error) {
	var out Deployment
	if err := c.post(ctx, CredAPIKey, "/v1/deployments/"+url.PathEscape(id)+"/pause", nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// UnpauseDeployment resumes scheduled runs.
func (c *Client) UnpauseDeployment(ctx context.Context, id string) (*Deployment, error) {
	var out Deployment
	if err := c.post(ctx, CredAPIKey, "/v1/deployments/"+url.PathEscape(id)+"/unpause", nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// ArchiveDeployment archives a deployment (terminal; there is no delete).
func (c *Client) ArchiveDeployment(ctx context.Context, id string) (*Deployment, error) {
	var out Deployment
	if err := c.post(ctx, CredAPIKey, "/v1/deployments/"+url.PathEscape(id)+"/archive", nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- memory stores ------------------------------------------------------------------

// ListMemoryStores lists memory stores.
func (c *Client) ListMemoryStores(ctx context.Context, includeArchived bool) ([]MemoryStore, error) {
	return listToken[MemoryStore](ctx, c, CredAPIKey, "/v1/memory_stores", archivedQuery(includeArchived), betaMemory)
}

// CreateMemoryStore creates a memory store.
func (c *Client) CreateMemoryStore(ctx context.Context, in MemoryStoreCreate) (*MemoryStore, error) {
	var out MemoryStore
	if err := c.create(ctx, CredAPIKey, "/v1/memory_stores", in, &out, betaMemory); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMemoryStore returns one memory store.
func (c *Client) GetMemoryStore(ctx context.Context, id string) (*MemoryStore, error) {
	var out MemoryStore
	if err := c.get(ctx, CredAPIKey, "/v1/memory_stores/"+url.PathEscape(id), nil, &out, betaMemory); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateMemoryStore updates a memory store.
func (c *Client) UpdateMemoryStore(ctx context.Context, id string, in MemoryStoreUpdate) (*MemoryStore, error) {
	var out MemoryStore
	if err := c.post(ctx, CredAPIKey, "/v1/memory_stores/"+url.PathEscape(id), in, &out, betaMemory); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteMemoryStore deletes a memory store and its memories.
func (c *Client) DeleteMemoryStore(ctx context.Context, id string) error {
	return c.delete(ctx, CredAPIKey, "/v1/memory_stores/"+url.PathEscape(id), betaMemory)
}

// ArchiveMemoryStore archives a memory store (terminal).
func (c *Client) ArchiveMemoryStore(ctx context.Context, id string) (*MemoryStore, error) {
	var out MemoryStore
	if err := c.post(ctx, CredAPIKey, "/v1/memory_stores/"+url.PathEscape(id)+"/archive", nil, &out, betaMemory); err != nil {
		return nil, err
	}
	return &out, nil
}

// --- skills ------------------------------------------------------------------------------

// SkillFile is one file of a skill upload; Path is relative and must live
// under a single top-level directory that contains SKILL.md. The live API
// requires the multipart field name "files[]".
type SkillFile struct {
	Path string
	Data []byte
}

func skillParts(files []SkillFile) []FilePart {
	parts := make([]FilePart, 0, len(files))
	for _, f := range files {
		parts = append(parts, FilePart{Field: "files[]", Filename: f.Path, Data: f.Data})
	}
	return parts
}

// ListSkills lists skills; source filters by custom or anthropic.
func (c *Client) ListSkills(ctx context.Context, source string) ([]Skill, error) {
	q := url.Values{}
	if source != "" {
		q.Set("source", source)
	}
	return listToken[Skill](ctx, c, CredAPIKey, "/v1/skills", q)
}

// CreateSkill uploads a new skill (multipart).
func (c *Client) CreateSkill(ctx context.Context, displayName *string, files []SkillFile) (*Skill, error) {
	fields := map[string]string{}
	if displayName != nil {
		fields["display_name"] = *displayName
	}
	var out Skill
	if err := c.postMultipart(ctx, CredAPIKey, "/v1/skills", fields, skillParts(files), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSkill returns one skill.
func (c *Client) GetSkill(ctx context.Context, id string) (*Skill, error) {
	var out Skill
	if err := c.get(ctx, CredAPIKey, "/v1/skills/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSkill deletes a skill and all its versions.
func (c *Client) DeleteSkill(ctx context.Context, id string) error {
	return c.delete(ctx, CredAPIKey, "/v1/skills/"+url.PathEscape(id))
}

// CreateSkillVersion uploads a new version of a skill (multipart).
func (c *Client) CreateSkillVersion(ctx context.Context, skillID string, files []SkillFile) (*SkillVersion, error) {
	var out SkillVersion
	if err := c.postMultipart(ctx, CredAPIKey, "/v1/skills/"+url.PathEscape(skillID)+"/versions", nil, skillParts(files), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSkillVersions lists a skill's versions.
func (c *Client) ListSkillVersions(ctx context.Context, skillID string) ([]SkillVersion, error) {
	return listToken[SkillVersion](ctx, c, CredAPIKey, "/v1/skills/"+url.PathEscape(skillID)+"/versions", nil)
}

// GetSkillVersion returns one version; version may be "latest".
func (c *Client) GetSkillVersion(ctx context.Context, skillID, version string) (*SkillVersion, error) {
	var out SkillVersion
	if err := c.get(ctx, CredAPIKey, "/v1/skills/"+url.PathEscape(skillID)+"/versions/"+url.PathEscape(version), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteSkillVersion deletes one version.
func (c *Client) DeleteSkillVersion(ctx context.Context, skillID, version string) error {
	return c.delete(ctx, CredAPIKey, "/v1/skills/"+url.PathEscape(skillID)+"/versions/"+url.PathEscape(version))
}

// --- deployment runs ---------------------------------------------------------

// DeploymentRun is one firing of a deployment.
//
// The shape here was taken from a live run rather than the reference: a run
// carries no status field and no start or end timestamps. It has either a
// SessionID or an Error, which is what the has_error filter selects on.
type DeploymentRun struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	DeploymentID   string          `json:"deployment_id"`
	SessionID      *string         `json:"session_id"`
	CreatedAt      string          `json:"created_at"`
	Agent          DeploymentAgent `json:"agent"`
	TriggerContext TriggerContext  `json:"trigger_context"`
	// Error is kept raw. Every run observed had it null, so its populated
	// shape is unconfirmed and decoding into a guessed struct would drop
	// whatever it actually contains.
	Error json.RawMessage `json:"error"`
}

// DeploymentAgent is the agent a run executed, pinned to a version.
type DeploymentAgent struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Version int64  `json:"version"`
}

// TriggerContext records what caused a run. ScheduledAt is set for schedule
// triggers and absent otherwise.
type TriggerContext struct {
	Type        string  `json:"type"`
	ScheduledAt *string `json:"scheduled_at,omitempty"`
}

// DeploymentRunListOptions filters ListDeploymentRuns. The CreatedAt bounds
// are RFC 3339 timestamps; HasError is tri-state, so it is a pointer.
type DeploymentRunListOptions struct {
	DeploymentID string
	TriggerType  string
	HasError     *bool
	CreatedAtGt  string
	CreatedAtGte string
	CreatedAtLt  string
	CreatedAtLte string
}

// ListDeploymentRuns lists deployment runs, newest first.
func (c *Client) ListDeploymentRuns(ctx context.Context, opts DeploymentRunListOptions) ([]DeploymentRun, error) {
	q := url.Values{}
	for key, v := range map[string]string{
		"deployment_id":  opts.DeploymentID,
		"trigger_type":   opts.TriggerType,
		"created_at_gt":  opts.CreatedAtGt,
		"created_at_gte": opts.CreatedAtGte,
		"created_at_lt":  opts.CreatedAtLt,
		"created_at_lte": opts.CreatedAtLte,
	} {
		if v != "" {
			q.Set(key, v)
		}
	}
	if opts.HasError != nil {
		q.Set("has_error", strconv.FormatBool(*opts.HasError))
	}
	return listToken[DeploymentRun](ctx, c, CredAPIKey, "/v1/deployment_runs", q, betaAgents)
}

// GetDeploymentRun returns one deployment run.
func (c *Client) GetDeploymentRun(ctx context.Context, id string) (*DeploymentRun, error) {
	var out DeploymentRun
	if err := c.get(ctx, CredAPIKey, "/v1/deployment_runs/"+url.PathEscape(id), nil, &out, betaAgents); err != nil {
		return nil, err
	}
	return &out, nil
}
