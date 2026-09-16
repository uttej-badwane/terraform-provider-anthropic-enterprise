data "anthropic_compliance_organizations" "all" {}

output "linked_organization_uuids" {
  value = data.anthropic_compliance_organizations.all.organizations[*].uuid
}
