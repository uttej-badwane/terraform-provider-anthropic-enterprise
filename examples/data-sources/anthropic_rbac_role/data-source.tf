data "anthropic_rbac_role" "editor" {
  id = "rbac_role_01ExampleRoleId0000000"
}

output "editor_permissions" {
  value = data.anthropic_rbac_role.editor.permissions
}
