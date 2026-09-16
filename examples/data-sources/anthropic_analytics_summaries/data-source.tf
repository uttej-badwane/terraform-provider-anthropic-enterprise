data "anthropic_analytics_summaries" "january" {
  starting_date = "2026-01-01"
  ending_date   = "2026-02-01"
}

output "latest_monthly_active_users" {
  value = data.anthropic_analytics_summaries.january.summaries[length(data.anthropic_analytics_summaries.january.summaries) - 1].monthly_active_user_count
}
