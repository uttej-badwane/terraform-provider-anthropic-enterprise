# Top 20 users by total tokens over a week.
data "anthropic_analytics_user_usage_report" "top" {
  starting_at           = "2026-01-01T00:00:00Z"
  ending_at             = "2026-01-08T00:00:00Z"
  order_by              = "total_tokens"
  order                 = "desc"
  exclude_deleted_users = true
  limit_rows            = 20
}
