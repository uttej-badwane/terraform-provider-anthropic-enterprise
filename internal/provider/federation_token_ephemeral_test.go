package provider

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
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

// Runtimes before 1.12 surface the ephemeral open itself in the machine-readable
// plan that follows an apply, so the framework sees a non-empty plan even though
// the runtime prints "No changes". OpenTofu 1.11 does this; 1.12 and current
// Terraform do not.
//
// Rather than skip those runtimes, the expectation follows the version under
// test, so the assertions still run everywhere. The axis is the version, not the
// runtime: an earlier attempt keyed on OpenTofu versus Terraform and then failed
// on OpenTofu 1.12.
func planIncludesEphemeralOpen(t *testing.T) bool {
	t.Helper()
	bin := os.Getenv("TF_ACC_TERRAFORM_PATH")
	if bin == "" {
		bin = "terraform"
	}
	// G702: bin is TF_ACC_TERRAFORM_PATH, the test framework's own contract for
	// locating the binary, which the framework then executes itself. Anything
	// able to set it has already chosen what this suite runs.
	out, err := exec.Command(bin, "version").Output() //nolint:gosec
	if err != nil {
		t.Logf("could not read the runtime version from %q (%v); assuming a current plan shape", bin, err)
		return false
	}
	m := regexp.MustCompile(`v(\d+)\.(\d+)\.`).FindSubmatch(out)
	if m == nil {
		t.Logf("could not parse a version from %q; assuming a current plan shape", out)
		return false
	}
	major, _ := strconv.Atoi(string(m[1]))
	minor, _ := strconv.Atoi(string(m[2]))
	return major == 1 && minor < 12
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

# Every open mints a new token, so echoing the token itself would differ on
# the next plan. OpenTofu 1.11 re-opens an ephemeral resource for the plan that
# follows an apply and would see that change, while 1.12 and Terraform do not.
# Recording facts about the token rather than the token keeps the plan empty on
# every runtime, and still proves the exchange returned what it should.
provider "echo" {
  data = {
    token_type   = ephemeral.anthropic_federation_token.test.token_type
    scope        = ephemeral.anthropic_federation_token.test.scope
    expires_in   = ephemeral.anthropic_federation_token.test.expires_in
    token_minted = startswith(ephemeral.anthropic_federation_token.test.access_token, "sk-ant-oat01-")
  }
}

resource "echo" "test" {}
`,
			ExpectNonEmptyPlan: planIncludesEphemeralOpen(t),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("token_type"), knownvalue.StringExact("Bearer")),
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("scope"), knownvalue.StringExact("workspace:inference")),
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("expires_in"), knownvalue.Int64Exact(3600)),
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("token_minted"),
					knownvalue.Bool(true)),
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

// The resource documents that it needs no provider credential, because the
// assertion authenticates the exchange. A configuration whose only use of the
// provider is minting a token must therefore configure with nothing set.
func TestAccFederationTokenEphemeral_withoutProviderCredentials(t *testing.T) {
	skipUnlessMock(t)
	for _, k := range []string{
		"ANTHROPIC_ADMIN_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_ENTERPRISE_API_KEY",
		"ANTHROPIC_COMPLIANCE_API_KEY", "ANTHROPIC_ANALYTICS_API_KEY", "ANTHROPIC_API_KEY",
	} {
		t.Setenv(k, "")
	}
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)},
		ProtoV6ProviderFactories: withEcho(),
		Steps: []resource.TestStep{{
			Config: `
provider "anthropic" {}

ephemeral "anthropic_federation_token" "test" {
  federation_rule_id = "fdrl_01ExampleRuleId0000000000"
  organization_id    = "00000000-0000-0000-0000-000000000000"
  service_account_id = "svac_01ExampleServiceAcct000000"
  assertion          = "` + mock.MockFederationAssertion + `"
}

provider "echo" {
  data = {
    token_minted = startswith(ephemeral.anthropic_federation_token.test.access_token, "sk-ant-oat01-")
  }
}
resource "echo" "test" {}
`,
			ExpectNonEmptyPlan: planIncludesEphemeralOpen(t),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("token_minted"),
					knownvalue.Bool(true)),
			},
		}},
	})
}
