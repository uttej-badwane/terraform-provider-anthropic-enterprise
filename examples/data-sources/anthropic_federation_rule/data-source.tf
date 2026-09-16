data "anthropic_federation_rule" "deploy" {
  name = "gha-deploy"
}

output "deploy_rule_workspaces" {
  value = data.anthropic_federation_rule.deploy.workspace_ids
}
