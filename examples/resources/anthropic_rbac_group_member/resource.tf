resource "anthropic_rbac_group" "engineering" {
  name = "Engineering"
}

data "anthropic_user" "alice" {
  email = "alice@example.com"
}

resource "anthropic_rbac_group_member" "alice" {
  group_id = anthropic_rbac_group.engineering.id
  user_id  = data.anthropic_user.alice.id
}
