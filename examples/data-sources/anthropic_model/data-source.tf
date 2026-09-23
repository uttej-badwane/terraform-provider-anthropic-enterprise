data "anthropic_model" "opus" {
  id = "claude-opus-5"
}

output "context_window" {
  value = data.anthropic_model.opus.max_input_tokens
}

# The flat capabilities map carries each capability's own support flag.
output "supports_thinking" {
  value = data.anthropic_model.opus.capabilities["thinking"]
}

# Some capabilities nest sub-capabilities that a map of booleans cannot
# express, so the payload is also exposed verbatim.
output "supports_compaction" {
  value = jsondecode(data.anthropic_model.opus.capabilities_json).context_management.compact_20260112.supported
}
