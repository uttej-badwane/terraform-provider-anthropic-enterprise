resource "anthropic_agent" "reviewer" {
  name         = "code-reviewer"
  model        = "claude-opus-5"
  model_effort = "high"
  description  = "Reviews pull requests and leaves inline comments."
  system       = "You review code changes carefully and explain your reasoning briefly."

  tools = jsonencode([
    {
      type = "agent_toolset_20260401"
      configs = [
        { name = "bash", permission_policy = { type = "always_ask" } },
        { name = "web_fetch", allowed_domains = ["docs.example.com"] },
      ]
    },
    { type = "mcp_toolset", mcp_server_name = "issue-tracker" },
  ])

  mcp_servers = [
    { name = "issue-tracker", url = "https://mcp.example.com/sse" },
  ]

  skills = [
    { type = "anthropic", skill_id = "xlsx" },
  ]

  metadata = { team = "platform" }
}
