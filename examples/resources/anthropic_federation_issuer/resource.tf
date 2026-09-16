# OIDC discovery (default key source)
resource "anthropic_federation_issuer" "github_actions" {
  name       = "github-actions"
  issuer_url = "https://token.actions.githubusercontent.com"
}

# Explicit JWKS endpoint with a shorter token lifetime
resource "anthropic_federation_issuer" "internal_ci" {
  name                     = "internal-ci"
  issuer_url               = "https://ci.example.com"
  max_jwt_lifetime_seconds = 600

  jwks = {
    type = "explicit_url"
    url  = "https://ci.example.com/.well-known/jwks.json"
  }
}
