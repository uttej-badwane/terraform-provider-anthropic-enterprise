data "anthropic_environment" "ci" {
  name = "ci-runners"
}

output "ci_environment_id" {
  value = data.anthropic_environment.ci.id
}
