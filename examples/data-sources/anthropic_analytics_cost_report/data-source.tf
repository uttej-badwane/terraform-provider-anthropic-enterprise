# Daily cost by cost type for one month; amounts are fractional cents.
data "anthropic_analytics_cost_report" "month" {
  starting_at = "2026-01-01T00:00:00Z"
  ending_at   = "2026-02-01T00:00:00Z"
  group_by    = ["cost_type"]
}

output "month_cost_usd" {
  value = tonumber(data.anthropic_analytics_cost_report.month.total_amount) / 100
}
