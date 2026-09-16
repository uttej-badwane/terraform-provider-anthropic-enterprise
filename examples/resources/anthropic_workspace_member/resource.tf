data "anthropic_user" "jane" {
  email = "jane@example.com"
}

resource "anthropic_workspace_member" "jane_production" {
  workspace_id   = anthropic_workspace.production.id
  user_id        = data.anthropic_user.jane.id
  workspace_role = "workspace_developer"
}
