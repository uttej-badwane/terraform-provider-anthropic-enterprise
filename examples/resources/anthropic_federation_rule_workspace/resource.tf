# Enable an existing rule for a second workspace.
resource "anthropic_federation_rule_workspace" "deploy_main_staging" {
  federation_rule_id = anthropic_federation_rule.deploy_main.id
  workspace_id       = anthropic_workspace.staging.id
}
