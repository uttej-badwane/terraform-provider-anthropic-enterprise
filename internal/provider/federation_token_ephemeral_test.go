package provider

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/mock"
)

// skipOnOpenTofu skips a test that depends on when the runtime re-opens an
// ephemeral resource. OpenTofu 1.11 re-opens it for the plan that follows an
// apply and so sees the freshly minted token as a change; 1.12 and Terraform do
// not. The provider behaves identically either way, so rather than encode a
// version comparison the echo-based assertion runs under Terraform, and the
// error path below still runs everywhere.
func skipOnOpenTofu(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC_PROVIDER_HOST") == "registry.opentofu.org" {
		t.Skip("plan emptiness after an ephemeral open differs across OpenTofu versions")
	}
}

// withEcho adds the echo provider alongside this one, so a test can read an
// ephemeral value that never reaches state.
func withEcho() map[string]func() (tfprotov6.ProviderServer, error) {
	m := map[string]func() (tfprotov6.ProviderServer, error){"echo": echoprovider.NewProviderServer()}
	for k, v := range testAccProtoV6ProviderFactories {
		m[k] = v
	}
	return m
}

// An ephemeral value cannot be asserted on directly, because it never reaches
// state. The echo provider exists for exactly this: it copies what it is given
// into its own state so a check can read it.
func TestAccFederationTokenEphemeral(t *testing.T) {
	skipUnlessMock(t)
	skipOnOpenTofu(t)
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)},
		ProtoV6ProviderFactories: withEcho(),
		Steps: []resource.TestStep{{
			Config: providerConfig + `
ephemeral "anthropic_federation_token" "test" {
  federation_rule_id = "fdrl_01ExampleRuleId0000000000"
  organization_id    = "00000000-0000-0000-0000-000000000000"
  service_account_id = "svac_01ExampleServiceAcct000000"
  assertion          = "` + mock.MockFederationAssertion + `"
}

provider "echo" {
  data = ephemeral.anthropic_federation_token.test
}

resource "echo" "test" {}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("token_type"), knownvalue.StringExact("Bearer")),
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("scope"), knownvalue.StringExact("workspace:inference")),
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("expires_in"), knownvalue.Int64Exact(3600)),
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("access_token"),
					knownvalue.StringRegexp(regexp.MustCompile(`^sk-ant-oat01-`))),
			},
		}},
	})
}

// A denied exchange must surface as an error rather than an empty token.
func TestAccFederationTokenEphemeral_deniedAssertion(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
ephemeral "anthropic_federation_token" "test" {
  federation_rule_id = "fdrl_01ExampleRuleId0000000000"
  organization_id    = "00000000-0000-0000-0000-000000000000"
  service_account_id = "svac_01ExampleServiceAcct000000"
  assertion          = "not.the.expected.assertion"
}
`,
			ExpectError: regexp.MustCompile(`Authentication\s+failed`),
		}},
	})
}
