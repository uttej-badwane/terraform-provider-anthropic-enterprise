# Every model the configured credential can call.
data "anthropic_models" "available" {}

# Fail the plan early if the model an agent is pinned to has been retired,
# rather than finding out when the apply reaches the API.
resource "anthropic_agent" "reviewer" {
  name  = "code-reviewer"
  model = "claude-opus-5"

  lifecycle {
    precondition {
      condition     = contains(data.anthropic_models.available.ids, self.model)
      error_message = "Model ${self.model} is not available to this credential."
    }
  }
}

# Pick the newest model that supports a capability the agent depends on.
locals {
  code_execution_models = [
    for m in data.anthropic_models.available.models : m.id
    if m.capabilities["code_execution"]
  ]
}
