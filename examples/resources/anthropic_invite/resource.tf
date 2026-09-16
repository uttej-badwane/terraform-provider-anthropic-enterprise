resource "anthropic_invite" "jane" {
  email = "jane@example.com"
  role  = "developer"
}

# Claude Enterprise organizations can pre-assign RBAC groups.
resource "anthropic_invite" "contractor" {
  email          = "contractor@example.com"
  role           = "user"
  rbac_group_ids = [anthropic_rbac_group.contractors.id]
}
