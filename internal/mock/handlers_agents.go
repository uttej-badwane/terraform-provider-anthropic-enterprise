package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// AgentsKey is the regular workspace API key accepted by the Managed Agents
// and Skills endpoints.
const AgentsKey = "sk-ant-api03-agents-test-0000"

type agentStore struct {
	agents        []*client.Agent
	agentHistory  map[string][]client.Agent
	environments  []*client.Environment
	vaults        []*client.Vault
	credentials   []*storedCredential
	deployments   []*client.Deployment
	memoryStores  []*client.MemoryStore
	skills        []*client.Skill
	skillVersions []*client.SkillVersion
}

type storedCredential struct {
	client.VaultCredential
	secrets map[string]string // never returned
}

func newAgentStore() *agentStore {
	return &agentStore{agentHistory: map[string][]client.Agent{}}
}

func (s *Server) seedAgents() {
	a := s.store.agentsStore
	for _, sk := range []struct{ id, name, desc string }{{"xlsx", "xlsx", "Create and edit spreadsheets"}, {"docx", "docx", "Create and edit documents"}} {
		a.skills = append(a.skills, &client.Skill{ID: sk.id, Type: "skill", DisplayName: sk.name, LatestVersionID: "20251013", Source: client.TypeOnly{Type: "anthropic"}, CreatedAt: now(), UpdatedAt: now()})
		a.skillVersions = append(a.skillVersions, &client.SkillVersion{ID: "20251013", Type: "skill_version", SkillID: sk.id, Name: sk.name, Description: sk.desc, CreatedAt: now()})
	}
}

// --- auth / headers -----------------------------------------------------------

type betaFamily int

const (
	familyAgents betaFamily = iota
	familyMemory
	familySkills
)

func (s *Server) requireAgents(w http.ResponseWriter, r *http.Request, fam betaFamily) bool {
	switch r.Header.Get("X-Api-Key") {
	case AgentsKey:
	case AdminKey, EnterpriseKey, ComplianceKey, AnalyticsKey:
		writeError(w, http.StatusUnauthorized, "authentication_error", "this endpoint requires a workspace API key")
		return false
	default:
		writeError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
		return false
	}
	betas := strings.Join(r.Header.Values("Anthropic-Beta"), ",")
	hasAgents := strings.Contains(betas, client.BetaManagedAgents)
	hasMemory := strings.Contains(betas, client.BetaAgentMemory)
	switch fam {
	case familyAgents:
		if !hasAgents {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "anthropic-beta: "+client.BetaManagedAgents+" is required")
			return false
		}
		if hasMemory {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "agent-memory and managed-agents betas cannot be combined")
			return false
		}
	case familyMemory:
		if !hasMemory || hasAgents {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "anthropic-beta: "+client.BetaAgentMemory+" is required (alone)")
			return false
		}
	}
	if ws := r.Header.Get("Anthropic-Workspace-Id"); ws != "" {
		w.Header().Set("anthropic-workspace-id", ws)
	}
	return true
}

func (s *Server) agentRoutes() {
	s.handle("GET /v1/agents", s.listAgents)
	s.handle("POST /v1/agents", s.createAgent)
	s.handle("GET /v1/agents/{id}", s.getAgent)
	s.handle("POST /v1/agents/{id}", s.updateAgent)
	s.handle("POST /v1/agents/{id}/archive", s.archiveAgent)
	s.handle("GET /v1/agents/{id}/versions", s.listAgentVersions)

	s.handle("GET /v1/environments", s.listEnvironments)
	s.handle("POST /v1/environments", s.createEnvironment)
	s.handle("GET /v1/environments/{id}", s.getEnvironment)
	s.handle("POST /v1/environments/{id}", s.updateEnvironment)
	s.handle("DELETE /v1/environments/{id}", s.deleteEnvironment)
	s.handle("POST /v1/environments/{id}/archive", s.archiveEnvironment)

	s.handle("GET /v1/vaults", s.listVaults)
	s.handle("POST /v1/vaults", s.createVault)
	s.handle("GET /v1/vaults/{id}", s.getVault)
	s.handle("POST /v1/vaults/{id}", s.updateVault)
	s.handle("DELETE /v1/vaults/{id}", s.deleteVault)
	s.handle("POST /v1/vaults/{id}/archive", s.archiveVault)
	s.handle("GET /v1/vaults/{id}/credentials", s.listCredentials)
	s.handle("POST /v1/vaults/{id}/credentials", s.createCredential)
	s.handle("GET /v1/vaults/{id}/credentials/{cid}", s.getCredential)
	s.handle("POST /v1/vaults/{id}/credentials/{cid}", s.updateCredential)
	s.handle("DELETE /v1/vaults/{id}/credentials/{cid}", s.deleteCredential)
	s.handle("POST /v1/vaults/{id}/credentials/{cid}/archive", s.archiveCredential)

	s.handle("GET /v1/deployments", s.listDeployments)
	s.handle("POST /v1/deployments", s.createDeployment)
	s.handle("GET /v1/deployments/{id}", s.getDeployment)
	s.handle("POST /v1/deployments/{id}", s.updateDeployment)
	s.handle("POST /v1/deployments/{id}/pause", s.pauseDeployment)
	s.handle("POST /v1/deployments/{id}/unpause", s.unpauseDeployment)
	s.handle("POST /v1/deployments/{id}/archive", s.archiveDeployment)

	s.handle("GET /v1/memory_stores", s.listMemoryStores)
	s.handle("POST /v1/memory_stores", s.createMemoryStore)
	s.handle("GET /v1/memory_stores/{id}", s.getMemoryStore)
	s.handle("POST /v1/memory_stores/{id}", s.updateMemoryStore)
	s.handle("DELETE /v1/memory_stores/{id}", s.deleteMemoryStore)
	s.handle("POST /v1/memory_stores/{id}/archive", s.archiveMemoryStore)

	s.handle("GET /v1/skills", s.listSkills)
	s.handle("POST /v1/skills", s.createSkill)
	s.handle("GET /v1/skills/{id}", s.getSkill)
	s.handle("DELETE /v1/skills/{id}", s.deleteSkill)
	s.handle("GET /v1/skills/{id}/versions", s.listSkillVersions)
	s.handle("POST /v1/skills/{id}/versions", s.createSkillVersion)
	s.handle("GET /v1/skills/{id}/versions/{v}", s.getSkillVersion)
	s.handle("DELETE /v1/skills/{id}/versions/{v}", s.deleteSkillVersion)
}

func applyMetadata(dst map[string]string, patch client.MetadataPatch) map[string]string {
	if dst == nil {
		dst = map[string]string{}
	}
	for k, v := range patch {
		if v == nil || *v == "" {
			delete(dst, k)
		} else {
			dst[k] = *v
		}
	}
	return dst
}

func nonNilMeta(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

// --- agents ------------------------------------------------------------------

var permissionTypes = []string{"always_allow", "always_ask", "auto"}
var builtinTools = []string{"bash", "read", "write", "edit", "glob", "grep", "web_fetch", "web_search"}

// normalizeTools mirrors the server: fills default_config and per-config
// type/enabled/permission_policy. It returns the mcp server names referenced.
func normalizeTools(w http.ResponseWriter, raw json.RawMessage, mcpServers []client.MCPServer) (json.RawMessage, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage("[]")
	}
	var tools []map[string]any
	if err := json.Unmarshal(raw, &tools); err != nil {
		writeError(w, 400, "invalid_request_error", "tools must be an array")
		return nil, false
	}
	if len(tools) > 128 {
		writeError(w, 400, "invalid_request_error", "at most 128 tools")
		return nil, false
	}
	referenced := map[string]bool{}
	policy := func(v any) map[string]any {
		if m, ok := v.(map[string]any); ok {
			if t, _ := m["type"].(string); slices.Contains(permissionTypes, t) {
				return map[string]any{"type": t}
			}
		}
		return nil
	}
	for _, t := range tools {
		typ, _ := t["type"].(string)
		switch typ {
		case "agent_toolset_20260401", "mcp_toolset":
			dc, _ := t["default_config"].(map[string]any)
			if dc == nil {
				dc = map[string]any{}
			}
			if _, ok := dc["enabled"].(bool); !ok {
				dc["enabled"] = true
			}
			if p := policy(dc["permission_policy"]); p != nil {
				dc["permission_policy"] = p
			} else {
				dc["permission_policy"] = map[string]any{"type": "always_ask"}
			}
			t["default_config"] = dc
			if typ == "mcp_toolset" {
				name, _ := t["mcp_server_name"].(string)
				if name == "" {
					writeError(w, 400, "invalid_request_error", "mcp_toolset requires mcp_server_name")
					return nil, false
				}
				if !slices.ContainsFunc(mcpServers, func(m client.MCPServer) bool { return m.Name == name }) {
					writeError(w, 400, "invalid_request_error", "mcp_server_name does not match any mcp_servers entry: "+name)
					return nil, false
				}
				referenced[name] = true
			}
			cfgs, _ := t["configs"].([]any)
			for _, c := range cfgs {
				cm, _ := c.(map[string]any)
				if cm == nil {
					continue
				}
				name, _ := cm["name"].(string)
				if typ == "agent_toolset_20260401" {
					if !slices.Contains(builtinTools, name) {
						writeError(w, 400, "invalid_request_error", "unknown built-in tool: "+name)
						return nil, false
					}
					cm["type"] = name
					for _, key := range []string{"allowed_domains", "blocked_domains"} {
						if v, ok := cm[key]; ok {
							arr, _ := v.([]any)
							if len(arr) == 0 {
								writeError(w, 400, "invalid_request_error", key+" must not be empty; omit it instead")
								return nil, false
							}
						}
					}
					if _, a := cm["allowed_domains"]; a {
						if _, b := cm["blocked_domains"]; b {
							writeError(w, 400, "invalid_request_error", "allowed_domains and blocked_domains are mutually exclusive")
							return nil, false
						}
					}
				}
				if _, ok := cm["enabled"].(bool); !ok {
					cm["enabled"] = dc["enabled"]
				}
				if p := policy(cm["permission_policy"]); p != nil {
					cm["permission_policy"] = p
				} else {
					cm["permission_policy"] = dc["permission_policy"]
				}
			}
			if cfgs == nil {
				t["configs"] = []any{}
			}
		case "custom":
			name, _ := t["name"].(string)
			if name == "" || t["description"] == nil || t["input_schema"] == nil {
				writeError(w, 400, "invalid_request_error", "custom tools require name, description and input_schema")
				return nil, false
			}
		default:
			writeError(w, 400, "invalid_request_error", "unknown tool type: "+typ)
			return nil, false
		}
	}
	for _, m := range mcpServers {
		if !referenced[m.Name] {
			writeError(w, 400, "invalid_request_error", "mcp server "+m.Name+" is not referenced by any mcp_toolset")
			return nil, false
		}
	}
	out, _ := json.Marshal(tools)
	return out, true
}

func (s *Server) resolveModel(w http.ResponseWriter, raw json.RawMessage, prev *client.ModelConfig) (*client.ModelConfig, bool) {
	var id string
	if err := json.Unmarshal(raw, &id); err == nil {
		if id == "" {
			writeError(w, 400, "invalid_request_error", "model is required")
			return nil, false
		}
		m := &client.ModelConfig{ID: id, Effort: &client.TypeOnly{Type: "high"}, Speed: ptr("standard")}
		if prev != nil && prev.ID == id && prev.Effort != nil {
			m.Effort = prev.Effort
		}
		return m, true
	}
	var obj struct {
		ID           string          `json:"id"`
		Effort       json.RawMessage `json:"effort"`
		InferenceGeo *string         `json:"inference_geo"`
		Speed        *string         `json:"speed"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil || obj.ID == "" {
		writeError(w, 400, "invalid_request_error", "model must be a string or an object with id")
		return nil, false
	}
	m := &client.ModelConfig{ID: obj.ID, Effort: &client.TypeOnly{Type: "high"}, Speed: ptr("standard"), InferenceGeo: obj.InferenceGeo}
	if len(obj.Effort) > 0 && string(obj.Effort) != "null" {
		var es string
		var eo client.TypeOnly
		if json.Unmarshal(obj.Effort, &es) == nil {
			m.Effort = &client.TypeOnly{Type: es}
		} else if json.Unmarshal(obj.Effort, &eo) == nil {
			m.Effort = &eo
		}
		if !slices.Contains([]string{"low", "medium", "high", "xhigh", "max"}, m.Effort.Type) {
			writeError(w, 400, "invalid_request_error", "invalid effort")
			return nil, false
		}
	} else if prev != nil && prev.ID == obj.ID && prev.Effort != nil {
		m.Effort = prev.Effort
	}
	if obj.Speed != nil {
		if *obj.Speed != "standard" && *obj.Speed != "fast" {
			writeError(w, 400, "invalid_request_error", "invalid speed")
			return nil, false
		}
		m.Speed = obj.Speed
	}
	return m, true
}

func (s *Server) findAgent(id string) *client.Agent {
	for _, a := range s.store.agentsStore.agents {
		if a.ID == id {
			return a
		}
	}
	return nil
}

func (s *Server) resolveSkills(w http.ResponseWriter, in []client.SkillRef) ([]client.SkillRef, bool) {
	out := []client.SkillRef{}
	for _, sk := range in {
		if sk.Type != "anthropic" && sk.Type != "custom" {
			writeError(w, 400, "invalid_request_error", "skill type must be anthropic or custom")
			return nil, false
		}
		v := "1"
		for _, s := range s.store.agentsStore.skills {
			if s.ID == sk.SkillID {
				v = s.LatestVersionID
			}
		}
		if sk.Version != nil && *sk.Version != "" && *sk.Version != "latest" {
			v = *sk.Version
		}
		out = append(out, client.SkillRef{Type: sk.Type, SkillID: sk.SkillID, Version: ptr(v)})
	}
	return out, true
}

func (s *Server) resolveMultiagent(w http.ResponseWriter, raw json.RawMessage, selfID string, selfVersion int64) (json.RawMessage, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("null"), true
	}
	var in struct {
		Type   string            `json:"type"`
		Agents []json.RawMessage `json:"agents"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || in.Type != "coordinator" || len(in.Agents) == 0 || len(in.Agents) > 20 {
		writeError(w, 400, "invalid_request_error", "multiagent must be {type: coordinator, agents: [1-20]}")
		return nil, false
	}
	var out []map[string]any
	seen := map[string]bool{}
	for _, e := range in.Agents {
		var id string
		entry := map[string]any{}
		if json.Unmarshal(e, &id) != nil {
			if err := json.Unmarshal(e, &entry); err != nil {
				writeError(w, 400, "invalid_request_error", "invalid roster entry")
				return nil, false
			}
		}
		switch {
		case id != "":
			entry = map[string]any{"type": "agent", "id": id}
		case entry["type"] == "self":
			entry = map[string]any{"type": "agent", "id": selfID, "version": selfVersion}
		case entry["type"] == "advisor":
			if m, _ := entry["model"].(string); m == "" {
				writeError(w, 400, "invalid_request_error", "advisor requires model")
				return nil, false
			}
			out = append(out, entry)
			continue
		}
		aid, _ := entry["id"].(string)
		if aid != selfID {
			ref := s.findAgent(aid)
			if ref == nil || ref.ArchivedAt != nil {
				writeError(w, 400, "invalid_request_error", "roster agent not found or archived: "+aid)
				return nil, false
			}
			if len(ref.Multiagent) > 0 && string(ref.Multiagent) != "null" {
				writeError(w, 400, "invalid_request_error", "roster agents cannot themselves be coordinators")
				return nil, false
			}
			if _, ok := entry["version"]; !ok {
				entry["version"] = ref.Version
			}
		} else if _, ok := entry["version"]; !ok {
			entry["version"] = selfVersion
		}
		if seen[aid] {
			writeError(w, 400, "invalid_request_error", "roster entries must reference distinct agents")
			return nil, false
		}
		seen[aid] = true
		out = append(out, entry)
	}
	b, _ := json.Marshal(map[string]any{"type": "coordinator", "agents": out})
	return b, true
}

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	inc := r.URL.Query().Get("include_archived") == "true"
	out := []*client.Agent{}
	for _, a := range s.store.agentsStore.agents {
		if a.ArchivedAt == nil || inc {
			out = append(out, a)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	var in struct {
		client.AgentCreate
		Model json.RawMessage `json:"model"`
	}
	if !decode(w, r, &in) {
		return
	}
	if l := len(in.Name); l < 1 || l > 256 {
		writeError(w, 400, "invalid_request_error", "name must be 1-256 characters")
		return
	}
	model, ok := s.resolveModel(w, in.Model, nil)
	if !ok {
		return
	}
	if len(in.MCPServers) > 20 {
		writeError(w, 400, "invalid_request_error", "at most 20 mcp_servers")
		return
	}
	tools, ok := normalizeTools(w, in.Tools, in.MCPServers)
	if !ok {
		return
	}
	skills, ok := s.resolveSkills(w, in.Skills)
	if !ok {
		return
	}
	id := s.nextID("agent")
	multi, ok := s.resolveMultiagent(w, in.Multiagent, id, 1)
	if !ok {
		return
	}
	servers := in.MCPServers
	if servers == nil {
		servers = []client.MCPServer{}
	}
	for i := range servers {
		servers[i].Type = "url"
	}
	a := &client.Agent{ID: id, Type: "agent", Name: in.Name, Description: in.Description, System: in.System, Model: *model, Tools: tools,
		MCPServers: servers, Skills: skills, Multiagent: multi, Metadata: nonNilMeta(in.Metadata), Version: 1, CreatedAt: now(), UpdatedAt: now()}
	s.store.agentsStore.agents = append(s.store.agentsStore.agents, a)
	s.store.agentsStore.agentHistory[id] = []client.Agent{*a}
	writeJSON(w, a)
}

func (s *Server) getAgent(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	a := s.findAgent(r.PathValue("id"))
	if a == nil {
		notFound(w, "agent")
		return
	}
	writeJSON(w, a)
}

func (s *Server) updateAgent(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	a := s.findAgent(r.PathValue("id"))
	if a == nil {
		notFound(w, "agent")
		return
	}
	if a.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "archived agents cannot be updated")
		return
	}
	var in struct {
		Name        *string              `json:"name"`
		Model       json.RawMessage      `json:"model"`
		Description client.Opt[string]   `json:"description"`
		System      client.Opt[string]   `json:"system"`
		Tools       json.RawMessage      `json:"tools"`
		MCPServers  *[]client.MCPServer  `json:"mcp_servers"`
		Skills      *[]client.SkillRef   `json:"skills"`
		Multiagent  json.RawMessage      `json:"multiagent"`
		Metadata    client.MetadataPatch `json:"metadata"`
		Version     *int64               `json:"version"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version != nil && *in.Version != a.Version {
		writeError(w, http.StatusConflict, "invalid_request_error", fmt.Sprintf("version mismatch: current version is %d", a.Version))
		return
	}
	next := *a
	if in.Name != nil {
		if l := len(*in.Name); l < 1 || l > 256 {
			writeError(w, 400, "invalid_request_error", "name must be 1-256 characters")
			return
		}
		next.Name = *in.Name
	}
	if len(in.Model) > 0 && string(in.Model) != "null" {
		m, ok := s.resolveModel(w, in.Model, &a.Model)
		if !ok {
			return
		}
		next.Model = *m
	}
	if in.Description.Set {
		if in.Description.Null || in.Description.Value == "" {
			next.Description = nil
		} else {
			next.Description = ptr(in.Description.Value)
		}
	}
	if in.System.Set {
		if in.System.Null || in.System.Value == "" {
			next.System = nil
		} else {
			next.System = ptr(in.System.Value)
		}
	}
	servers := next.MCPServers
	if in.MCPServers != nil {
		servers = *in.MCPServers
		for i := range servers {
			servers[i].Type = "url"
		}
	}
	toolsRaw := next.Tools
	if len(in.Tools) > 0 {
		toolsRaw = in.Tools
	}
	if in.MCPServers != nil || len(in.Tools) > 0 {
		tools, ok := normalizeTools(w, toolsRaw, servers)
		if !ok {
			return
		}
		next.Tools, next.MCPServers = tools, servers
	}
	if in.Skills != nil {
		skills, ok := s.resolveSkills(w, *in.Skills)
		if !ok {
			return
		}
		next.Skills = skills
	}
	if len(in.Multiagent) > 0 {
		multi, ok := s.resolveMultiagent(w, in.Multiagent, a.ID, a.Version+1)
		if !ok {
			return
		}
		next.Multiagent = multi
	}
	meta := map[string]string{}
	for k, v := range a.Metadata {
		meta[k] = v
	}
	next.Metadata = applyMetadata(meta, in.Metadata)

	same := next.Name == a.Name && reflect.DeepEqual(next.Model, a.Model) && reflect.DeepEqual(next.Description, a.Description) &&
		reflect.DeepEqual(next.System, a.System) && string(next.Tools) == string(a.Tools) && reflect.DeepEqual(next.MCPServers, a.MCPServers) &&
		reflect.DeepEqual(next.Skills, a.Skills) && string(next.Multiagent) == string(a.Multiagent) && reflect.DeepEqual(next.Metadata, a.Metadata)
	if same {
		writeJSON(w, a)
		return
	}
	next.Version = a.Version + 1
	next.UpdatedAt = now()
	*a = next
	s.store.agentsStore.agentHistory[a.ID] = append(s.store.agentsStore.agentHistory[a.ID], next)
	writeJSON(w, a)
}

func (s *Server) archiveAgent(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	a := s.findAgent(r.PathValue("id"))
	if a == nil {
		notFound(w, "agent")
		return
	}
	if a.ArchivedAt == nil {
		a.ArchivedAt = ptr(now())
		for _, d := range s.store.agentsStore.deployments {
			if d.Agent.ID == a.ID && d.ArchivedAt == nil {
				d.ArchivedAt = ptr(now())
			}
		}
	}
	writeJSON(w, a)
}

func (s *Server) listAgentVersions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	a := s.findAgent(r.PathValue("id"))
	if a == nil {
		notFound(w, "agent")
		return
	}
	hist := s.store.agentsStore.agentHistory[a.ID]
	out := make([]client.Agent, len(hist))
	copy(out, hist)
	slices.Reverse(out)
	tokenList(w, r, out)
}

// --- environments -----------------------------------------------------------------

func (s *Server) findEnvironment(id string) *client.Environment {
	for _, e := range s.store.agentsStore.environments {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func validateEnvConfig(w http.ResponseWriter, c *client.EnvironmentConfig, prev *client.EnvironmentConfig) (*client.EnvironmentConfig, bool) {
	out := client.EnvironmentConfig{Type: c.Type}
	if prev != nil && c.Type == "" {
		out.Type = prev.Type
	}
	switch out.Type {
	case "self_hosted":
		return &out, true
	case "cloud":
	default:
		writeError(w, 400, "invalid_request_error", "config.type must be cloud or self_hosted")
		return nil, false
	}
	net := &client.Networking{Type: "unrestricted"}
	if prev != nil && prev.Networking != nil {
		cp := *prev.Networking
		net = &cp
	}
	if c.Networking != nil {
		if c.Networking.Type != "" {
			net.Type = c.Networking.Type
		}
		if c.Networking.AllowedHosts != nil {
			net.AllowedHosts = c.Networking.AllowedHosts
		}
		if c.Networking.AllowMCPServers != nil {
			net.AllowMCPServers = c.Networking.AllowMCPServers
		}
		if c.Networking.AllowPackageManagers != nil {
			net.AllowPackageManagers = c.Networking.AllowPackageManagers
		}
	}
	switch net.Type {
	case "unrestricted":
		net = &client.Networking{Type: "unrestricted"}
	case "limited":
		if net.AllowedHosts == nil {
			net.AllowedHosts = []string{}
		}
		if net.AllowMCPServers == nil {
			net.AllowMCPServers = ptr(false)
		}
		if net.AllowPackageManagers == nil {
			net.AllowPackageManagers = ptr(false)
		}
	default:
		writeError(w, 400, "invalid_request_error", "networking.type must be unrestricted or limited")
		return nil, false
	}
	out.Networking = net
	pk := &client.Packages{Type: "packages", Apt: []string{}, Cargo: []string{}, Gem: []string{}, Go: []string{}, Npm: []string{}, Pip: []string{}}
	if prev != nil && prev.Packages != nil {
		cp := *prev.Packages
		pk = &cp
	}
	if c.Packages != nil {
		for src, dst := range map[*[]string]*[]string{&c.Packages.Apt: &pk.Apt, &c.Packages.Cargo: &pk.Cargo, &c.Packages.Gem: &pk.Gem, &c.Packages.Go: &pk.Go, &c.Packages.Npm: &pk.Npm, &c.Packages.Pip: &pk.Pip} {
			if *src != nil {
				*dst = *src
			}
		}
	}
	total := len(pk.Apt) + len(pk.Cargo) + len(pk.Gem) + len(pk.Go) + len(pk.Npm) + len(pk.Pip)
	if total > 0 && net.Type == "limited" && !*net.AllowPackageManagers {
		writeError(w, 400, "invalid_request_error", "packages under limited networking require allow_package_managers: true")
		return nil, false
	}
	out.Packages = pk
	return &out, true
}

func (s *Server) listEnvironments(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	inc := r.URL.Query().Get("include_archived") == "true"
	out := []*client.Environment{}
	for _, e := range s.store.agentsStore.environments {
		if e.ArchivedAt == nil || inc {
			out = append(out, e)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) createEnvironment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	var in client.EnvironmentCreate
	if !decode(w, r, &in) {
		return
	}
	if l := len(in.Name); l < 1 || l > 256 {
		writeError(w, 400, "invalid_request_error", "name must be 1-256 characters")
		return
	}
	for _, e := range s.store.agentsStore.environments {
		if e.Name == in.Name && e.ArchivedAt == nil {
			writeError(w, http.StatusConflict, "invalid_request_error", "an environment with this name already exists")
			return
		}
	}
	cfg, ok := validateEnvConfig(w, &in.Config, nil)
	if !ok {
		return
	}
	scope := "organization"
	if in.Scope != nil {
		if *in.Scope != "organization" && *in.Scope != "account" {
			writeError(w, 400, "invalid_request_error", "scope must be organization or account")
			return
		}
		scope = *in.Scope
	}
	e := &client.Environment{ID: s.nextID("env"), Type: "environment", Name: in.Name, Description: in.Description, Config: *cfg,
		Metadata: nonNilMeta(in.Metadata), Scope: scope, CreatedAt: now(), UpdatedAt: now()}
	s.store.agentsStore.environments = append(s.store.agentsStore.environments, e)
	writeJSON(w, e)
}

func (s *Server) getEnvironment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	e := s.findEnvironment(r.PathValue("id"))
	if e == nil {
		notFound(w, "environment")
		return
	}
	writeJSON(w, e)
}

func (s *Server) updateEnvironment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	e := s.findEnvironment(r.PathValue("id"))
	if e == nil {
		notFound(w, "environment")
		return
	}
	if e.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "archived environments cannot be updated")
		return
	}
	var in client.EnvironmentUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.Name != nil {
		for _, o := range s.store.agentsStore.environments {
			if o.ID != e.ID && o.Name == *in.Name && o.ArchivedAt == nil {
				writeError(w, http.StatusConflict, "invalid_request_error", "an environment with this name already exists")
				return
			}
		}
		e.Name = *in.Name
	}
	if in.Description.Set {
		if in.Description.Null {
			e.Description = nil
		} else {
			e.Description = ptr(in.Description.Value)
		}
	}
	if in.Config != nil {
		cfg, ok := validateEnvConfig(w, in.Config, &e.Config)
		if !ok {
			return
		}
		e.Config = *cfg
	}
	e.Metadata = applyMetadata(e.Metadata, in.Metadata)
	e.UpdatedAt = now()
	writeJSON(w, e)
}

func (s *Server) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	id := r.PathValue("id")
	for i, e := range s.store.agentsStore.environments {
		if e.ID == id {
			s.store.agentsStore.environments = slices.Delete(s.store.agentsStore.environments, i, i+1)
			deleted(w, id, "environment_deleted")
			return
		}
	}
	notFound(w, "environment")
}

func (s *Server) archiveEnvironment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	e := s.findEnvironment(r.PathValue("id"))
	if e == nil {
		notFound(w, "environment")
		return
	}
	if e.ArchivedAt == nil {
		e.ArchivedAt = ptr(now())
	}
	writeJSON(w, e)
}

// --- vaults --------------------------------------------------------------------------

func (s *Server) findVault(id string) *client.Vault {
	for _, v := range s.store.agentsStore.vaults {
		if v.ID == id {
			return v
		}
	}
	return nil
}

func (s *Server) listVaults(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	inc := r.URL.Query().Get("include_archived") == "true"
	out := []*client.Vault{}
	for _, v := range s.store.agentsStore.vaults {
		if v.ArchivedAt == nil || inc {
			out = append(out, v)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) createVault(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	var in client.VaultCreate
	if !decode(w, r, &in) {
		return
	}
	if l := len(in.DisplayName); l < 1 || l > 255 {
		writeError(w, 400, "invalid_request_error", "display_name must be 1-255 characters")
		return
	}
	v := &client.Vault{ID: s.nextID("vlt"), Type: "vault", DisplayName: in.DisplayName, Metadata: nonNilMeta(in.Metadata), CreatedAt: now(), UpdatedAt: now()}
	s.store.agentsStore.vaults = append(s.store.agentsStore.vaults, v)
	writeJSON(w, v)
}

func (s *Server) getVault(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	v := s.findVault(r.PathValue("id"))
	if v == nil {
		notFound(w, "vault")
		return
	}
	writeJSON(w, v)
}

func (s *Server) updateVault(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	v := s.findVault(r.PathValue("id"))
	if v == nil {
		notFound(w, "vault")
		return
	}
	var in client.VaultUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.DisplayName != nil {
		v.DisplayName = *in.DisplayName
	}
	v.Metadata = applyMetadata(v.Metadata, in.Metadata)
	v.UpdatedAt = now()
	writeJSON(w, v)
}

func (s *Server) deleteVault(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	id := r.PathValue("id")
	for i, v := range s.store.agentsStore.vaults {
		if v.ID == id {
			s.store.agentsStore.vaults = slices.Delete(s.store.agentsStore.vaults, i, i+1)
			s.store.agentsStore.credentials = slices.DeleteFunc(s.store.agentsStore.credentials, func(c *storedCredential) bool { return c.VaultID == id })
			deleted(w, id, "vault_deleted")
			return
		}
	}
	notFound(w, "vault")
}

func (s *Server) archiveVault(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	v := s.findVault(r.PathValue("id"))
	if v == nil {
		notFound(w, "vault")
		return
	}
	if v.ArchivedAt == nil {
		v.ArchivedAt = ptr(now())
		for _, c := range s.store.agentsStore.credentials {
			if c.VaultID == v.ID && c.ArchivedAt == nil {
				c.ArchivedAt = ptr(now())
				c.secrets = nil
			}
		}
	}
	writeJSON(w, v)
}

// credential auth handling: parse the union, strip secrets, enforce immutables.
func credentialKey(auth map[string]any) string {
	if v, ok := auth["mcp_server_url"].(string); ok {
		return "url:" + v
	}
	if v, ok := auth["secret_name"].(string); ok {
		return "name:" + v
	}
	return ""
}

func sanitizeAuth(w http.ResponseWriter, raw json.RawMessage, prev map[string]any) (map[string]any, map[string]string, bool) {
	var auth map[string]any
	if err := json.Unmarshal(raw, &auth); err != nil || auth == nil {
		writeError(w, 400, "invalid_request_error", "auth must be an object")
		return nil, nil, false
	}
	secrets := map[string]string{}
	typ, _ := auth["type"].(string)
	if prev != nil {
		if pt, _ := prev["type"].(string); pt != typ {
			writeError(w, 400, "invalid_request_error", "auth.type cannot change")
			return nil, nil, false
		}
		for _, k := range []string{"mcp_server_url", "secret_name"} {
			if pv, ok := prev[k]; ok {
				if nv, ok2 := auth[k]; ok2 && nv != pv {
					writeError(w, 400, "invalid_request_error", k+" is immutable")
					return nil, nil, false
				}
				auth[k] = pv
			}
		}
	}
	take := func(m map[string]any, k string) {
		if v, ok := m[k].(string); ok {
			if v != "" {
				secrets[k] = v
			}
			delete(m, k)
		}
	}
	switch typ {
	case "static_bearer":
		if u, _ := auth["mcp_server_url"].(string); u == "" {
			writeError(w, 400, "invalid_request_error", "mcp_server_url is required")
			return nil, nil, false
		}
		take(auth, "token")
		if prev == nil && secrets["token"] == "" {
			writeError(w, 400, "invalid_request_error", "token is required")
			return nil, nil, false
		}
	case "environment_variable":
		if n, _ := auth["secret_name"].(string); n == "" {
			writeError(w, 400, "invalid_request_error", "secret_name is required")
			return nil, nil, false
		}
		take(auth, "secret_value")
		if prev == nil && secrets["secret_value"] == "" {
			writeError(w, 400, "invalid_request_error", "secret_value is required")
			return nil, nil, false
		}
		if _, ok := auth["networking"]; !ok {
			if prev != nil {
				auth["networking"] = prev["networking"]
			} else {
				writeError(w, 400, "invalid_request_error", "networking is required")
				return nil, nil, false
			}
		}
		il, present := auth["injection_location"].(map[string]any)
		if !present {
			if prev != nil {
				auth["injection_location"] = prev["injection_location"]
			} else {
				auth["injection_location"] = map[string]any{"header": true, "body": true}
			}
		} else {
			h, _ := il["header"].(bool)
			b, _ := il["body"].(bool)
			if !h && !b {
				writeError(w, 400, "invalid_request_error", "injection_location must enable header or body")
				return nil, nil, false
			}
			auth["injection_location"] = map[string]any{"header": h, "body": b}
		}
	case "mcp_oauth":
		if u, _ := auth["mcp_server_url"].(string); u == "" {
			writeError(w, 400, "invalid_request_error", "mcp_server_url is required")
			return nil, nil, false
		}
		take(auth, "access_token")
		if prev == nil && secrets["access_token"] == "" {
			writeError(w, 400, "invalid_request_error", "access_token is required")
			return nil, nil, false
		}
		if ref, ok := auth["refresh"].(map[string]any); ok {
			take(ref, "refresh_token")
			if tea, ok := ref["token_endpoint_auth"].(map[string]any); ok {
				take(tea, "client_secret")
				ref["token_endpoint_auth"] = map[string]any{"type": tea["type"]}
			}
		}
	default:
		writeError(w, 400, "invalid_request_error", "auth.type must be static_bearer, environment_variable or mcp_oauth")
		return nil, nil, false
	}
	return auth, secrets, true
}

func (s *Server) findCredential(vaultID, id string) (int, *storedCredential) {
	for i, c := range s.store.agentsStore.credentials {
		if c.VaultID == vaultID && c.ID == id {
			return i, c
		}
	}
	return -1, nil
}

func (s *Server) listCredentials(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	if s.findVault(r.PathValue("id")) == nil {
		notFound(w, "vault")
		return
	}
	inc := r.URL.Query().Get("include_archived") == "true"
	out := []client.VaultCredential{}
	for _, c := range s.store.agentsStore.credentials {
		if c.VaultID == r.PathValue("id") && (c.ArchivedAt == nil || inc) {
			out = append(out, c.VaultCredential)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) createCredential(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	v := s.findVault(r.PathValue("id"))
	if v == nil {
		notFound(w, "vault")
		return
	}
	var in client.VaultCredentialCreate
	if !decode(w, r, &in) {
		return
	}
	auth, secrets, ok := sanitizeAuth(w, in.Auth, nil)
	if !ok {
		return
	}
	live := 0
	for _, c := range s.store.agentsStore.credentials {
		if c.VaultID == v.ID && c.ArchivedAt == nil {
			live++
			var existing map[string]any
			_ = json.Unmarshal(c.Auth, &existing)
			if credentialKey(existing) == credentialKey(auth) {
				writeError(w, http.StatusConflict, "invalid_request_error", "a credential with this key already exists in the vault")
				return
			}
		}
	}
	if live >= 20 {
		writeError(w, 400, "invalid_request_error", "a vault holds at most 20 credentials")
		return
	}
	raw, _ := json.Marshal(auth)
	c := &storedCredential{VaultCredential: client.VaultCredential{ID: s.nextID("vcrd"), Type: "vault_credential", VaultID: v.ID, DisplayName: in.DisplayName,
		Metadata: nonNilMeta(in.Metadata), Auth: raw, CreatedAt: now(), UpdatedAt: now()}, secrets: secrets}
	s.store.agentsStore.credentials = append(s.store.agentsStore.credentials, c)
	writeJSON(w, c.VaultCredential)
}

func (s *Server) getCredential(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	_, c := s.findCredential(r.PathValue("id"), r.PathValue("cid"))
	if c == nil {
		notFound(w, "vault_credential")
		return
	}
	writeJSON(w, c.VaultCredential)
}

func (s *Server) updateCredential(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	_, c := s.findCredential(r.PathValue("id"), r.PathValue("cid"))
	if c == nil {
		notFound(w, "vault_credential")
		return
	}
	if c.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "archived credentials cannot be updated")
		return
	}
	var in client.VaultCredentialUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.DisplayName.Set {
		if in.DisplayName.Null {
			c.DisplayName = nil
		} else {
			c.DisplayName = ptr(in.DisplayName.Value)
		}
	}
	if len(in.Auth) > 0 {
		var prev map[string]any
		_ = json.Unmarshal(c.Auth, &prev)
		auth, secrets, ok := sanitizeAuth(w, in.Auth, prev)
		if !ok {
			return
		}
		for k, v := range secrets {
			c.secrets[k] = v
		}
		raw, _ := json.Marshal(auth)
		c.Auth = raw
	}
	c.Metadata = applyMetadata(c.Metadata, in.Metadata)
	c.UpdatedAt = now()
	writeJSON(w, c.VaultCredential)
}

func (s *Server) deleteCredential(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	i, c := s.findCredential(r.PathValue("id"), r.PathValue("cid"))
	if c == nil {
		notFound(w, "vault_credential")
		return
	}
	s.store.agentsStore.credentials = slices.Delete(s.store.agentsStore.credentials, i, i+1)
	deleted(w, c.ID, "vault_credential_deleted")
}

func (s *Server) archiveCredential(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	_, c := s.findCredential(r.PathValue("id"), r.PathValue("cid"))
	if c == nil {
		notFound(w, "vault_credential")
		return
	}
	if c.ArchivedAt == nil {
		c.ArchivedAt = ptr(now())
		c.secrets = nil
	}
	writeJSON(w, c.VaultCredential)
}

// --- deployments ----------------------------------------------------------------

var cronRe = regexp.MustCompile(`^\S+\s+\S+\s+\S+\s+\S+\s+\S+$`)

func (s *Server) findDeployment(id string) *client.Deployment {
	for _, d := range s.store.agentsStore.deployments {
		if d.ID == id {
			return d
		}
	}
	return nil
}

func (s *Server) resolveAgentRef(w http.ResponseWriter, raw json.RawMessage) (*client.AgentRef, bool) {
	var id string
	ref := client.AgentRef{Type: "agent"}
	if json.Unmarshal(raw, &id) == nil {
		ref.ID = id
	} else if err := json.Unmarshal(raw, &ref); err != nil || ref.ID == "" {
		writeError(w, 400, "invalid_request_error", "agent must be an id or {type: agent, id, version}")
		return nil, false
	}
	a := s.findAgent(ref.ID)
	if a == nil || a.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "agent not found or archived")
		return nil, false
	}
	if ref.Version == nil {
		ref.Version = ptr(a.Version)
	} else if *ref.Version < 1 || *ref.Version > a.Version {
		writeError(w, 400, "invalid_request_error", "unknown agent version")
		return nil, false
	}
	ref.Type = "agent"
	return &ref, true
}

func validateSchedule(w http.ResponseWriter, sch *client.Schedule) bool {
	if sch == nil {
		return true
	}
	if sch.Type != "cron" || !cronRe.MatchString(strings.TrimSpace(sch.Expression)) || sch.Timezone == "" {
		writeError(w, 400, "invalid_request_error", "schedule must be {type: cron, expression: 5-field, timezone}")
		return false
	}
	if _, err := time.LoadLocation(sch.Timezone); err != nil {
		writeError(w, 400, "invalid_request_error", "unknown timezone")
		return false
	}
	return true
}

func upcomingRuns(sch *client.Schedule, status string) []string {
	if sch == nil || status != "active" {
		return []string{}
	}
	base := time.Now().UTC().Truncate(time.Hour).Add(time.Hour)
	out := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		out = append(out, base.Add(time.Duration(i)*24*time.Hour).Format(time.RFC3339))
	}
	return out
}

func validateEvents(w http.ResponseWriter, raw json.RawMessage) bool {
	var events []map[string]any
	if err := json.Unmarshal(raw, &events); err != nil || len(events) < 1 || len(events) > 50 {
		writeError(w, 400, "invalid_request_error", "initial_events must be an array of 1-50 events")
		return false
	}
	for _, e := range events {
		t, _ := e["type"].(string)
		if !slices.Contains([]string{"user.message", "user.define_outcome", "system.message"}, t) {
			writeError(w, 400, "invalid_request_error", "unsupported initial event type: "+t)
			return false
		}
	}
	return true
}

func stripResourceSecrets(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return json.RawMessage("[]")
	}
	var res []map[string]any
	if json.Unmarshal(raw, &res) != nil {
		return raw
	}
	for _, r := range res {
		delete(r, "authorization_token")
	}
	b, _ := json.Marshal(res)
	return b
}

func (s *Server) listDeployments(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	q := r.URL.Query()
	inc := q.Get("include_archived") == "true"
	out := []*client.Deployment{}
	for _, d := range s.store.agentsStore.deployments {
		if d.ArchivedAt != nil && !inc {
			continue
		}
		if a := q.Get("agent_id"); a != "" && d.Agent.ID != a {
			continue
		}
		if st := q.Get("status"); st != "" && d.Status != st {
			continue
		}
		out = append(out, d)
	}
	tokenList(w, r, out)
}

func (s *Server) createDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	var in struct {
		client.DeploymentCreate
		Agent json.RawMessage `json:"agent"`
	}
	if !decode(w, r, &in) {
		return
	}
	if l := len(in.Name); l < 1 || l > 256 {
		writeError(w, 400, "invalid_request_error", "name must be 1-256 characters")
		return
	}
	ref, ok := s.resolveAgentRef(w, in.Agent)
	if !ok {
		return
	}
	env := s.findEnvironment(in.EnvironmentID)
	if env == nil || env.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "environment not found or archived")
		return
	}
	if !validateEvents(w, in.InitialEvents) || !validateSchedule(w, in.Schedule) {
		return
	}
	for _, vid := range in.VaultIDs {
		if v := s.findVault(vid); v == nil || v.ArchivedAt != nil {
			writeError(w, 400, "invalid_request_error", "vault not found or archived: "+vid)
			return
		}
	}
	vaults := in.VaultIDs
	if vaults == nil {
		vaults = []string{}
	}
	d := &client.Deployment{ID: s.nextID("depl"), Type: "deployment", Name: in.Name, Agent: *ref, EnvironmentID: env.ID, InitialEvents: in.InitialEvents,
		Schedule: in.Schedule, Resources: stripResourceSecrets(in.Resources), VaultIDs: vaults, Budget: in.Budget, Description: in.Description,
		Metadata: nonNilMeta(in.Metadata), Status: "active", CreatedAt: now(), UpdatedAt: now()}
	d.UpcomingRuns = upcomingRuns(d.Schedule, d.Status)
	s.store.agentsStore.deployments = append(s.store.agentsStore.deployments, d)
	writeJSON(w, d)
}

func (s *Server) getDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	d := s.findDeployment(r.PathValue("id"))
	if d == nil {
		notFound(w, "deployment")
		return
	}
	writeJSON(w, d)
}

func (s *Server) updateDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	d := s.findDeployment(r.PathValue("id"))
	if d == nil {
		notFound(w, "deployment")
		return
	}
	if d.ArchivedAt != nil {
		writeError(w, 400, "invalid_request_error", "archived deployments cannot be updated")
		return
	}
	var in struct {
		Name          *string                      `json:"name"`
		Agent         json.RawMessage              `json:"agent"`
		EnvironmentID *string                      `json:"environment_id"`
		InitialEvents json.RawMessage              `json:"initial_events"`
		Schedule      client.Opt[*client.Schedule] `json:"schedule"`
		Resources     json.RawMessage              `json:"resources"`
		VaultIDs      *[]string                    `json:"vault_ids"`
		Budget        client.Opt[*client.Budget]   `json:"budget"`
		Description   client.Opt[string]           `json:"description"`
		Metadata      client.MetadataPatch         `json:"metadata"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Name != nil {
		d.Name = *in.Name
	}
	if len(in.Agent) > 0 && string(in.Agent) != "null" {
		ref, ok := s.resolveAgentRef(w, in.Agent)
		if !ok {
			return
		}
		d.Agent = *ref
	}
	if in.EnvironmentID != nil {
		env := s.findEnvironment(*in.EnvironmentID)
		if env == nil || env.ArchivedAt != nil {
			writeError(w, 400, "invalid_request_error", "environment not found or archived")
			return
		}
		d.EnvironmentID = env.ID
	}
	if len(in.InitialEvents) > 0 && string(in.InitialEvents) != "null" {
		if !validateEvents(w, in.InitialEvents) {
			return
		}
		d.InitialEvents = in.InitialEvents
	}
	if in.Schedule.Set {
		if in.Schedule.Null {
			d.Schedule = nil
		} else {
			if !validateSchedule(w, in.Schedule.Value) {
				return
			}
			d.Schedule = in.Schedule.Value
		}
	}
	if len(in.Resources) > 0 {
		d.Resources = stripResourceSecrets(in.Resources)
	}
	if in.VaultIDs != nil {
		for _, vid := range *in.VaultIDs {
			if v := s.findVault(vid); v == nil || v.ArchivedAt != nil {
				writeError(w, 400, "invalid_request_error", "vault not found or archived: "+vid)
				return
			}
		}
		d.VaultIDs = *in.VaultIDs
	}
	if in.Budget.Set {
		if in.Budget.Null {
			d.Budget = nil
		} else {
			d.Budget = in.Budget.Value
		}
	}
	if in.Description.Set {
		if in.Description.Null {
			d.Description = nil
		} else {
			d.Description = ptr(in.Description.Value)
		}
	}
	d.Metadata = applyMetadata(d.Metadata, in.Metadata)
	d.UpcomingRuns = upcomingRuns(d.Schedule, d.Status)
	d.UpdatedAt = now()
	writeJSON(w, d)
}

func (s *Server) pauseDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	d := s.findDeployment(r.PathValue("id"))
	if d == nil {
		notFound(w, "deployment")
		return
	}
	d.Status = "paused"
	d.PausedReason = &client.PausedReason{Type: "manual"}
	d.UpcomingRuns = []string{}
	writeJSON(w, d)
}

func (s *Server) unpauseDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	d := s.findDeployment(r.PathValue("id"))
	if d == nil {
		notFound(w, "deployment")
		return
	}
	d.Status = "active"
	d.PausedReason = nil
	d.UpcomingRuns = upcomingRuns(d.Schedule, d.Status)
	writeJSON(w, d)
}

func (s *Server) archiveDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyAgents) {
		return
	}
	d := s.findDeployment(r.PathValue("id"))
	if d == nil {
		notFound(w, "deployment")
		return
	}
	if d.ArchivedAt == nil {
		d.ArchivedAt = ptr(now())
		d.UpcomingRuns = []string{}
	}
	writeJSON(w, d)
}

// --- memory stores ------------------------------------------------------------------

func (s *Server) findMemoryStore(id string) *client.MemoryStore {
	for _, m := range s.store.agentsStore.memoryStores {
		if m.ID == id {
			return m
		}
	}
	return nil
}

func (s *Server) listMemoryStores(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyMemory) {
		return
	}
	inc := r.URL.Query().Get("include_archived") == "true"
	out := []*client.MemoryStore{}
	for _, m := range s.store.agentsStore.memoryStores {
		if m.ArchivedAt == nil || inc {
			out = append(out, m)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) createMemoryStore(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyMemory) {
		return
	}
	var in client.MemoryStoreCreate
	if !decode(w, r, &in) {
		return
	}
	if l := len(in.Name); l < 1 || l > 255 {
		writeError(w, 400, "invalid_request_error", "name must be 1-255 characters")
		return
	}
	m := &client.MemoryStore{ID: s.nextID("memstore"), Type: "memory_store", Name: in.Name, Metadata: nonNilMeta(in.Metadata), CreatedAt: now(), UpdatedAt: now()}
	if in.Description != nil {
		m.Description = *in.Description
	}
	s.store.agentsStore.memoryStores = append(s.store.agentsStore.memoryStores, m)
	writeJSON(w, m)
}

func (s *Server) getMemoryStore(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyMemory) {
		return
	}
	m := s.findMemoryStore(r.PathValue("id"))
	if m == nil {
		notFound(w, "memory_store")
		return
	}
	writeJSON(w, m)
}

func (s *Server) updateMemoryStore(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyMemory) {
		return
	}
	m := s.findMemoryStore(r.PathValue("id"))
	if m == nil {
		notFound(w, "memory_store")
		return
	}
	var in client.MemoryStoreUpdate
	if !decode(w, r, &in) {
		return
	}
	if in.Name != nil {
		m.Name = *in.Name
	}
	if in.Description != nil {
		m.Description = *in.Description
	}
	m.Metadata = applyMetadata(m.Metadata, in.Metadata)
	m.UpdatedAt = now()
	writeJSON(w, m)
}

func (s *Server) deleteMemoryStore(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyMemory) {
		return
	}
	id := r.PathValue("id")
	for i, m := range s.store.agentsStore.memoryStores {
		if m.ID == id {
			s.store.agentsStore.memoryStores = slices.Delete(s.store.agentsStore.memoryStores, i, i+1)
			deleted(w, id, "memory_store_deleted")
			return
		}
	}
	notFound(w, "memory_store")
}

func (s *Server) archiveMemoryStore(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familyMemory) {
		return
	}
	m := s.findMemoryStore(r.PathValue("id"))
	if m == nil {
		notFound(w, "memory_store")
		return
	}
	if m.ArchivedAt == nil {
		m.ArchivedAt = ptr(now())
	}
	writeJSON(w, m)
}

// --- skills ------------------------------------------------------------------------------

var skillNameRe = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
var frontmatterRe = regexp.MustCompile(`(?s)^---\n(.*?)\n---`)

type skillUpload struct {
	name, description string
}

func parseSkillUpload(w http.ResponseWriter, r *http.Request) (*skillUpload, string, bool) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, 400, "invalid_request_error", "expected multipart/form-data: "+err.Error())
		return nil, "", false
	}
	files := r.MultipartForm.File["files[]"] // the live API requires the bracketed field name
	if len(files) == 0 {
		writeError(w, 400, "invalid_request_error", "files[]: Field required")
		return nil, "", false
	}
	if len(files) > 20 {
		writeError(w, 400, "invalid_request_error", "at most 20 files")
		return nil, "", false
	}
	var top string
	var skillMD []byte
	for _, fh := range files {
		name := fh.Filename
		if _, params, err := mime.ParseMediaType(fh.Header.Get("Content-Disposition")); err == nil && params["filename"] != "" {
			name = params["filename"] // Go strips directories from fh.Filename; the raw header keeps them
		}
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			writeError(w, 400, "invalid_request_error", "all files must be inside one top-level directory")
			return nil, "", false
		}
		if top == "" {
			top = parts[0]
		} else if top != parts[0] {
			writeError(w, 400, "invalid_request_error", "all files must share the same top-level directory")
			return nil, "", false
		}
		if parts[1] == "SKILL.md" {
			f, err := fh.Open()
			if err != nil {
				writeError(w, 400, "invalid_request_error", "cannot read SKILL.md")
				return nil, "", false
			}
			skillMD, _ = io.ReadAll(f)
			_ = f.Close()
		}
	}
	if skillMD == nil {
		writeError(w, 400, "invalid_request_error", "SKILL.md is required at the root of the skill directory")
		return nil, "", false
	}
	m := frontmatterRe.FindSubmatch(skillMD)
	if m == nil {
		writeError(w, 400, "invalid_request_error", "SKILL.md must start with YAML frontmatter")
		return nil, "", false
	}
	up := &skillUpload{}
	for _, line := range strings.Split(string(m[1]), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		switch strings.TrimSpace(k) {
		case "name":
			up.name = v
		case "description":
			up.description = v
		}
	}
	if !skillNameRe.MatchString(up.name) || strings.Contains(up.name, "anthropic") || strings.Contains(up.name, "claude") {
		writeError(w, 400, "invalid_request_error", "SKILL.md name must be 1-64 lowercase letters, digits or hyphens and may not contain anthropic or claude")
		return nil, "", false
	}
	if up.description == "" || len(up.description) > 1024 {
		writeError(w, 400, "invalid_request_error", "SKILL.md description is required (max 1024)")
		return nil, "", false
	}
	return up, r.FormValue("display_name"), true
}

func (s *Server) findSkill(id string) *client.Skill {
	for _, sk := range s.store.agentsStore.skills {
		if sk.ID == id {
			return sk
		}
	}
	return nil
}

func (s *Server) listSkills(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familySkills) {
		return
	}
	src := r.URL.Query().Get("source")
	out := []*client.Skill{}
	for _, sk := range s.store.agentsStore.skills {
		if src == "" || sk.Source.Type == src {
			out = append(out, sk)
		}
	}
	tokenList(w, r, out)
}

func (s *Server) createSkill(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familySkills) {
		return
	}
	up, display, ok := parseSkillUpload(w, r)
	if !ok {
		return
	}
	if display == "" {
		display = up.name
	}
	if len(display) > 255 {
		writeError(w, 400, "invalid_request_error", "display_name max 255 characters")
		return
	}
	sk := &client.Skill{ID: s.nextID("skill"), Type: "skill", DisplayName: display, Source: client.TypeOnly{Type: "custom"}, CreatedAt: now(), UpdatedAt: now()}
	v := &client.SkillVersion{ID: s.nextID("skver"), Type: "skill_version", SkillID: sk.ID, Name: up.name, Description: up.description, CreatedAt: now()}
	sk.LatestVersionID = v.ID
	s.store.agentsStore.skills = append(s.store.agentsStore.skills, sk)
	s.store.agentsStore.skillVersions = append(s.store.agentsStore.skillVersions, v)
	writeJSON(w, sk)
}

func (s *Server) getSkill(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familySkills) {
		return
	}
	sk := s.findSkill(r.PathValue("id"))
	if sk == nil {
		notFound(w, "skill")
		return
	}
	writeJSON(w, sk)
}

func (s *Server) deleteSkill(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familySkills) {
		return
	}
	id := r.PathValue("id")
	for i, sk := range s.store.agentsStore.skills {
		if sk.ID == id {
			if sk.Source.Type != "custom" {
				writeError(w, 400, "invalid_request_error", "only custom skills can be deleted")
				return
			}
			s.store.agentsStore.skills = slices.Delete(s.store.agentsStore.skills, i, i+1)
			s.store.agentsStore.skillVersions = slices.DeleteFunc(s.store.agentsStore.skillVersions, func(v *client.SkillVersion) bool { return v.SkillID == id })
			deleted(w, id, "skill_deleted")
			return
		}
	}
	notFound(w, "skill")
}

func (s *Server) listSkillVersions(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familySkills) {
		return
	}
	sk := s.findSkill(r.PathValue("id"))
	if sk == nil {
		notFound(w, "skill")
		return
	}
	out := []*client.SkillVersion{}
	for _, v := range s.store.agentsStore.skillVersions {
		if v.SkillID == sk.ID {
			out = append(out, v)
		}
	}
	slices.Reverse(out)
	tokenList(w, r, out)
}

func (s *Server) createSkillVersion(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familySkills) {
		return
	}
	sk := s.findSkill(r.PathValue("id"))
	if sk == nil {
		notFound(w, "skill")
		return
	}
	if sk.Source.Type != "custom" {
		writeError(w, 400, "invalid_request_error", "only custom skills accept new versions")
		return
	}
	up, _, ok := parseSkillUpload(w, r)
	if !ok {
		return
	}
	for _, v := range s.store.agentsStore.skillVersions {
		if v.SkillID == sk.ID && v.Name != up.name {
			writeError(w, 400, "invalid_request_error", "SKILL.md name must match the skill's existing name: "+v.Name)
			return
		}
	}
	v := &client.SkillVersion{ID: s.nextID("skver"), Type: "skill_version", SkillID: sk.ID, Name: up.name, Description: up.description, CreatedAt: now()}
	sk.LatestVersionID = v.ID
	sk.UpdatedAt = now()
	s.store.agentsStore.skillVersions = append(s.store.agentsStore.skillVersions, v)
	writeJSON(w, v)
}

func (s *Server) getSkillVersion(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familySkills) {
		return
	}
	sk := s.findSkill(r.PathValue("id"))
	if sk == nil {
		notFound(w, "skill")
		return
	}
	want := r.PathValue("v")
	if want == "latest" {
		want = sk.LatestVersionID
	}
	for _, v := range s.store.agentsStore.skillVersions {
		if v.SkillID == sk.ID && v.ID == want {
			writeJSON(w, v)
			return
		}
	}
	notFound(w, "skill_version")
}

func (s *Server) deleteSkillVersion(w http.ResponseWriter, r *http.Request) {
	if !s.requireAgents(w, r, familySkills) {
		return
	}
	sk := s.findSkill(r.PathValue("id"))
	if sk == nil {
		notFound(w, "skill")
		return
	}
	want := r.PathValue("v")
	before := len(s.store.agentsStore.skillVersions)
	s.store.agentsStore.skillVersions = slices.DeleteFunc(s.store.agentsStore.skillVersions, func(v *client.SkillVersion) bool { return v.SkillID == sk.ID && v.ID == want })
	if len(s.store.agentsStore.skillVersions) == before {
		notFound(w, "skill_version")
		return
	}
	if sk.LatestVersionID == want {
		sk.LatestVersionID = ""
		for _, v := range s.store.agentsStore.skillVersions {
			if v.SkillID == sk.ID {
				sk.LatestVersionID = v.ID
			}
		}
	}
	deleted(w, want, "skill_version_deleted")
}
