data "anthropic_cost_report" "january" {
  starting_at = "2026-01-01T00:00:00Z"
  ending_at   = "2026-02-01T00:00:00Z"
  group_by    = ["workspace_id"]
}

# Amounts are decimal strings in cents.
output "january_total_usd" {
  value = tonumber(data.anthropic_cost_report.january.total_amount) / 100
}
