data "anthropic_agents" "all" {
  include_archived = false
}

output "agent_names" {
  value = data.anthropic_agents.all.agents[*].name
}
