# Secret values are never returned; only metadata about each credential.
data "anthropic_vault_credentials" "ci" {
  vault_id = "vlt_01ExampleVaultId000000000"
}
