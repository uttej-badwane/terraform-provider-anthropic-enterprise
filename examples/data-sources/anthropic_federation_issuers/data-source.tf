data "anthropic_federation_issuers" "all" {}

output "issuer_urls" {
  value = data.anthropic_federation_issuers.all.issuers[*].issuer_url
}
