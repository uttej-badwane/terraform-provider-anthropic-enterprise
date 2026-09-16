# Per-user activity for one day, broken out by RBAC group.
data "anthropic_analytics_users" "yesterday" {
  date     = "2026-01-15"
  group_by = ["rbac_group_id"]
}

output "active_chat_users" {
  value = [for r in data.anthropic_analytics_users.yesterday.rows : r.email_address if r.chat_message_count > 0]
}
