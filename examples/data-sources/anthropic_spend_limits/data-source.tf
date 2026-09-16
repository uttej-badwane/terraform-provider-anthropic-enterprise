data "anthropic_spend_limits" "monthly" {
  periods = ["monthly"]
}

output "users_over_default" {
  value = [
    for row in data.anthropic_spend_limits.monthly.spend_limits : row.email
    if row.source_type == "user"
  ]
}
