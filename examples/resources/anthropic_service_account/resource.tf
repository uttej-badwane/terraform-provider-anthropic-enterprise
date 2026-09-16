resource "anthropic_service_account" "ci_deploy" {
  name              = "ci-deploy-bot"
  description       = "Deploys from the main branch"
  organization_role = "developer"
}
