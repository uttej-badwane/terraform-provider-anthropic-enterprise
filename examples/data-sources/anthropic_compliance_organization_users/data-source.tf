data "anthropic_compliance_organization_users" "example" {
  organization_uuid = "00000000-0000-4000-8000-000000000001"
}

output "admins" {
  value = [for u in data.anthropic_compliance_organization_users.example.users : u.email if u.organization_role == "admin"]
}
