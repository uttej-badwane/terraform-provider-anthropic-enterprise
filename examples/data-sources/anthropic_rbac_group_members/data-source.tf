data "anthropic_rbac_group_members" "engineering" {
  group_id = "rbac_group_01ExampleGroupId00000000"
}

output "engineering_emails" {
  value = data.anthropic_rbac_group_members.engineering.members[*].email
}
