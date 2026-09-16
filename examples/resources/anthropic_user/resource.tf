# Members join through invites or SSO. Import an existing member, then manage
# the organization role here.
resource "anthropic_user" "jane" {
  role = "developer"
}

# Opt in to removing the member from the organization on destroy.
resource "anthropic_user" "contractor" {
  role              = "user"
  remove_on_destroy = true
}
