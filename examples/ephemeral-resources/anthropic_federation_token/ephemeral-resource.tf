# Exchange a GitHub Actions OIDC token for a short-lived Anthropic credential,
# so the pipeline never holds a long-lived key. The minted token is never
# written to state or to the plan.
variable "github_oidc_token" {
  description = "The OIDC token the platform minted for this job."
  type        = string
  sensitive   = true
}

ephemeral "anthropic_federation_token" "ci" {
  federation_rule_id = anthropic_federation_rule.ci_main.id
  organization_id    = data.anthropic_organization.current.id
  service_account_id = anthropic_service_account.ci.id
  assertion          = var.github_oidc_token
}

# A second provider instance authenticated with the minted token.
provider "anthropic" {
  alias       = "federated"
  oauth_token = ephemeral.anthropic_federation_token.ci.access_token
}
