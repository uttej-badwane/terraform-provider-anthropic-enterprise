data "anthropic_compliance_effective_settings" "example" {
  organization_uuid = "00000000-0000-4000-8000-000000000001"
}

# Attest the security baseline: fail the plan if content redaction is off.
check "content_redaction" {
  assert {
    condition     = jsondecode(lookup(data.anthropic_compliance_effective_settings.example.settings_map, "content_redaction_enabled", "false"))
    error_message = "Content redaction must be enabled."
  }
}
