# Every run of one scheduled deployment.
data "anthropic_deployment_runs" "nightly" {
  deployment_id = anthropic_deployment.nightly_report.id
}

# Only the failures, for alerting on a schedule that has started breaking.
data "anthropic_deployment_runs" "failures" {
  deployment_id = anthropic_deployment.nightly_report.id
  has_error     = true
}

output "recent_failure_count" {
  value = length(data.anthropic_deployment_runs.failures.runs)
}

# A run has either a session or an error, never both.
output "sessions" {
  value = [
    for r in data.anthropic_deployment_runs.nightly.runs : r.session_id
    if r.session_id != null
  ]
}
