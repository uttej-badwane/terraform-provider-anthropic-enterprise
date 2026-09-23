resource "anthropic_agent" "support" {
  name   = "support-triage"
  model  = "claude-sonnet-5"
  system = file("${path.module}/prompts/support.md")
}

# Count the agent's system prompt against the model that will serve it. At
# least one message is required, even when only the system prompt matters; a
# one-word message adds only a few tokens.
data "anthropic_count_tokens" "support_prompt" {
  model    = anthropic_agent.support.model
  system   = anthropic_agent.support.system
  messages = [{ role = "user", content = "hi" }]
}

# Stop the plan if the prompt has grown past its budget, rather than finding out
# from a slow or expensive session.
check "support_prompt_budget" {
  assert {
    condition     = data.anthropic_count_tokens.support_prompt.input_tokens < 8000
    error_message = "The support system prompt is ${data.anthropic_count_tokens.support_prompt.input_tokens} tokens; the budget is 8000."
  }
}

# Tools can dominate a count, so include them when they will be sent.
data "anthropic_count_tokens" "with_tools" {
  model    = "claude-sonnet-5"
  messages = [{ role = "user", content = "What is the weather in Paris?" }]
  tools_json = jsonencode([{
    name         = "get_weather"
    description  = "Get the current weather for a city"
    input_schema = { type = "object", properties = { city = { type = "string" } } }
  }])
}
