variable "github_token" {
  type      = string
  sensitive = true
}

resource "anthropic_deployment" "nightly_report" {
  name           = "nightly-incident-report"
  agent_id       = "agent_01ExampleAgentId0000000000"
  agent_version  = 3
  environment_id = "env_01ExampleEnvId000000000000"

  initial_events = jsonencode([
    {
      type    = "user.message"
      content = [{ type = "text", text = "Summarize yesterday's incidents and open a tracking issue." }]
    }
  ])

  schedule = {
    cron_expression = "0 6 * * 1-5"
    timezone        = "UTC"
  }

  github_repositories = [{
    url                 = "https://github.com/example-org/example-repo"
    authorization_token = var.github_token
    checkout_branch     = "main"
  }]

  vault_ids                  = [anthropic_vault.ci.id]
  budget_max_list_cost_cents = "50000"
}
