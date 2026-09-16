data "anthropic_users" "developers" {
  roles = ["developer", "claude_code_user"]
}

output "developer_emails" {
  value = data.anthropic_users.developers.users[*].email
}
