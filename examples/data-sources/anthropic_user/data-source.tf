data "anthropic_user" "by_email" {
  email = "jane@example.com"
}

data "anthropic_user" "by_id" {
  id = "user_01ExampleUserId0000000000"
}
