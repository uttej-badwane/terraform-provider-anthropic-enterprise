data "anthropic_compliance_settings" "org" {}

output "compliance_api_state" {
  value = data.anthropic_compliance_settings.org.state
}
