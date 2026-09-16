resource "anthropic_external_key" "production" {
  display_name       = "production-us"
  validate_on_create = true

  provider_config = {
    type    = "aws"
    kms_arn = "arn:aws:kms:us-east-1:123456789012:key/11111111-2222-3333-4444-555555555555"
  }
}

# Attach the key to a workspace (write-once).
resource "anthropic_workspace" "production" {
  name            = "Production"
  external_key_id = anthropic_external_key.production.id
}
