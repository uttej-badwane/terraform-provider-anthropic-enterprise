# Daily token usage by model and product for one week.
data "anthropic_analytics_usage_report" "week" {
  starting_at  = "2026-01-01T00:00:00Z"
  ending_at    = "2026-01-08T00:00:00Z"
  bucket_width = "1d"
  group_by     = ["model", "product"]
  products     = ["chat", "claude_code"]
}

output "output_tokens_week" {
  value = data.anthropic_analytics_usage_report.week.total_output_tokens
}
