data "anthropic_federation_rules" "github_actions" {
  issuer_id = "fdis_01ExampleIssuerId00000000"
}

output "rule_names" {
  value = data.anthropic_federation_rules.github_actions.rules[*].name
}
