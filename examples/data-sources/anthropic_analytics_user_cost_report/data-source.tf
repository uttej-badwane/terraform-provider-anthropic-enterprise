# Cost for two specific users over a week, by product.
data "anthropic_analytics_user_cost_report" "team" {
  starting_at = "2026-01-01T00:00:00Z"
  ending_at   = "2026-01-08T00:00:00Z"
  group_by    = ["product"]
  user_ids    = ["user_01ExampleUserId0000000000", "user_01ExampleUserId0000000001"]
  order_by    = "amount"
}
