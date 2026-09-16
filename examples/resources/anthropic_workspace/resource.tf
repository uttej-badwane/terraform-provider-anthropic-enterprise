resource "anthropic_workspace" "production" {
  name          = "Production"
  display_color = "#6C5BB9"
  tags = {
    env  = "prod"
    team = "platform"
  }
}

# Restrict inference to US regions and attach a customer-managed key.
resource "anthropic_workspace" "regulated" {
  name                   = "Regulated"
  allowed_inference_geos = ["us"]
  default_inference_geo  = "us"
  external_key_id        = anthropic_external_key.prod.id
}
