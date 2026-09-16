data "anthropic_spend_limit_increase_requests" "pending" {
  statuses = ["pending"]
}

output "pending_requesters" {
  value = [for r in data.anthropic_spend_limit_increase_requests.pending.requests : r.actor_email]
}
