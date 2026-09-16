resource "anthropic_federation_rule" "deploy_main" {
  name               = "deploy-main"
  issuer_id          = anthropic_federation_issuer.github_actions.id
  service_account_id = anthropic_service_account.ci_deploy.id
  oauth_scope        = "workspace:inference"
  workspace_id       = anthropic_workspace.production.id
  description        = "Tokens minted for deploys from the main branch"

  match = {
    subject_prefix = "repo:example-org/example-repo:ref:refs/heads/main"
    claims = {
      repository_owner = "example-org"
    }
  }
}
