package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Each case is its own test function: a step that expects an error is re-run by
// the post-test destroy, so grouping them would replay the wrong configuration.
//
// The patterns match whitespace rather than a literal space, because Terraform
// wraps diagnostic text at a width that depends on the surrounding message.

func TestAccProviderCredentialMismatch_adminKeyInAPIKey(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {
  api_key = "sk-ant-admin01-notarealkey"
}
data "anthropic_organization" "test" {}
`,
			ExpectError: regexp.MustCompile(`expects\s+a\s+workspace\s+API\s+key`),
		}},
	})
}

func TestAccProviderCredentialMismatch_apiKeyInAdminKey(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {
  admin_api_key = "sk-ant-api03-notarealkey"
}
data "anthropic_organization" "test" {}
`,
			ExpectError: regexp.MustCompile(`expects\s+an\s+Admin\s+API\s+key`),
		}},
	})
}

// A credential of the right class must not be rejected. The mock issues keys
// with the same prefixes the real APIs use, so the default test configuration
// exercises the accepting path on every other test in this package; this one
// pins it explicitly so a future prefix change cannot silently start rejecting
// valid keys.
func TestAccProviderCredentialMismatch_correctClassesAccepted(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_organization" "test" {}
`,
		}},
	})
}
