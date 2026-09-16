data "anthropic_environments" "all" {}

output "environment_names" {
  value = data.anthropic_environments.all.environments[*].name
}
