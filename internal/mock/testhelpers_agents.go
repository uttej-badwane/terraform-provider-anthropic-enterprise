package mock

import (
	"encoding/json"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/client"
)

// SeedAgent adds a minimal normalized agent as if created through the API.
func (s *Server) SeedAgent(name string) *client.Agent {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := &client.Agent{ID: s.nextID("agent"), Type: "agent", Name: name,
		Model: client.ModelConfig{ID: "claude-sonnet-5", Effort: &client.TypeOnly{Type: "high"}, Speed: ptr("standard")},
		Tools: json.RawMessage("[]"), MCPServers: []client.MCPServer{}, Skills: []client.SkillRef{}, Multiagent: json.RawMessage("null"),
		Metadata: map[string]string{}, Version: 1, CreatedAt: now(), UpdatedAt: now()}
	s.store.agentsStore.agents = append(s.store.agentsStore.agents, a)
	s.store.agentsStore.agentHistory[a.ID] = []client.Agent{*a}
	c := *a
	return &c
}

// SeedEnvironment adds a minimal cloud environment as if created through the API.
func (s *Server) SeedEnvironment(name string) *client.Environment {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := &client.Environment{ID: s.nextID("env"), Type: "environment", Name: name, Scope: "organization",
		Config: client.EnvironmentConfig{Type: "cloud", Networking: &client.Networking{Type: "unrestricted"},
			Packages: &client.Packages{Type: "packages", Apt: []string{}, Cargo: []string{}, Gem: []string{}, Go: []string{}, Npm: []string{}, Pip: []string{}}},
		Metadata: map[string]string{}, CreatedAt: now(), UpdatedAt: now()}
	s.store.agentsStore.environments = append(s.store.agentsStore.environments, e)
	c := *e
	return &c
}
