terraform {
  required_providers {
    anthropic = {
      source = "uttej-badwane/anthropic-enterprise"
    }
  }
}

# Credentials fall back to ANTHROPIC_ADMIN_API_KEY, ANTHROPIC_AUTH_TOKEN and
# ANTHROPIC_ENTERPRISE_API_KEY when the attributes are omitted.
provider "anthropic" {}
