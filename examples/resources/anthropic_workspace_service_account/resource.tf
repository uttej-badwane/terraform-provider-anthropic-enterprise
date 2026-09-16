resource "anthropic_workspace" "production" {
  name = "Production"
}

resource "anthropic_service_account" "ci_deploy" {
  name = "ci-deploy-bot"
}

resource "anthropic_workspace_service_account" "ci_deploy_production" {
  workspace_id       = anthropic_workspace.production.id
  service_account_id = anthropic_service_account.ci_deploy.id
  workspace_role     = "workspace_developer"
}
