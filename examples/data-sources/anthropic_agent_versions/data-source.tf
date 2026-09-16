data "anthropic_agent_versions" "history" {
  agent_id = "agent_01ExampleAgentId0000000000"
}

output "latest_version" {
  value = data.anthropic_agent_versions.history.versions[0].version
}
