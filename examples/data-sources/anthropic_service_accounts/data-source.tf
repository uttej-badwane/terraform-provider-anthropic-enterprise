data "anthropic_service_accounts" "all" {
  include_archived = false
}

output "service_account_names" {
  value = data.anthropic_service_accounts.all.service_accounts[*].name
}
