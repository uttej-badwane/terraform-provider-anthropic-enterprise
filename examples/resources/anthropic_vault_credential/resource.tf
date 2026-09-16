variable "github_token" {
  type      = string
  sensitive = true
}

variable "mcp_bearer_token" {
  type      = string
  sensitive = true
}

# A secret exposed to the sandbox as an environment variable and substituted
# at egress only for the listed hosts. The value is write-only: it never lands
# in state. Change secret_version to rotate it.
resource "anthropic_vault_credential" "github" {
  vault_id       = anthropic_vault.ci.id
  display_name   = "GitHub token"
  secret_version = "2026-09"

  environment_variable = {
    secret_name     = "GH_TOKEN"
    secret_value    = var.github_token
    networking_type = "limited"
    allowed_hosts   = ["api.github.com"]
  }
}

# A fixed bearer token for one MCP server.
resource "anthropic_vault_credential" "mcp" {
  vault_id = anthropic_vault.ci.id

  static_bearer = {
    mcp_server_url = "https://mcp.example.com/sse"
    token          = var.mcp_bearer_token
  }
}
