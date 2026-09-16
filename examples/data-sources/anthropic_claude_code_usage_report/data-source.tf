data "anthropic_claude_code_usage_report" "yesterday" {
  date = "2026-01-15"
}

output "sessions_by_actor" {
  value = {
    for r in data.anthropic_claude_code_usage_report.yesterday.records :
    coalesce(r.actor_email, r.actor_api_key_name) => r.num_sessions
  }
}
