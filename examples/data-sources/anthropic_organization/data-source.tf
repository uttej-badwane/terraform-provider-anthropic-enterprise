data "anthropic_organization" "current" {}

output "organization_id" {
  value = data.anthropic_organization.current.id
}
