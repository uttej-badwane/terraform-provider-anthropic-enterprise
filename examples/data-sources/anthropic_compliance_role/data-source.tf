data "anthropic_compliance_role" "reviewer" {
  organization_uuid = "00000000-0000-4000-8000-000000000001"
  id                = "rbac_role_01ExampleRoleId0000000"
}

output "reviewer_actions" {
  value = data.anthropic_compliance_role.reviewer.permissions[*].action
}
