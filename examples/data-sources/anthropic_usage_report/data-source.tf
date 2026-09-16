data "anthropic_usage_report" "last_week_by_model" {
  starting_at  = "2026-01-01T00:00:00Z"
  ending_at    = "2026-01-08T00:00:00Z"
  bucket_width = "1d"
  group_by     = ["model", "workspace_id"]
}

output "total_output_tokens" {
  value = data.anthropic_usage_report.last_week_by_model.total_output_tokens
}
