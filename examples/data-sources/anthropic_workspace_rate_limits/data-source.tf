data "anthropic_workspace_rate_limits" "production" {
  workspace_id = anthropic_workspace.production.id
  group_type   = "model_group"
}
