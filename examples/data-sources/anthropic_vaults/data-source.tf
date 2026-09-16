data "anthropic_vaults" "all" {
  include_archived = false
}

output "vault_names" {
  value = data.anthropic_vaults.all.vaults[*].display_name
}
