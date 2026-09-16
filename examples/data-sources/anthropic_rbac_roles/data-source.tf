data "anthropic_rbac_roles" "all" {}

output "role_names" {
  value = data.anthropic_rbac_roles.all.roles[*].name
}
