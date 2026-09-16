data "anthropic_deployments" "for_agent" {
  agent_id = "agent_01ExampleAgentId0000000000"
  status   = "active"
}
