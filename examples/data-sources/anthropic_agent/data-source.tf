data "anthropic_agent" "reviewer" {
  id = "agent_01ExampleAgentId0000000000"
}

output "reviewer_version" {
  value = data.anthropic_agent.reviewer.version
}
