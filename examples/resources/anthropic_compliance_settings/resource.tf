# Singleton: declare at most one instance per organization.
resource "anthropic_compliance_settings" "org" {
  state = "enabled"
}
