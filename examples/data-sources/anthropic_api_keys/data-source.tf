data "anthropic_api_keys" "active_production" {
  status       = "active"
  workspace_id = anthropic_workspace.production.id
}

output "production_key_names" {
  value = data.anthropic_api_keys.active_production.api_keys[*].name
}
