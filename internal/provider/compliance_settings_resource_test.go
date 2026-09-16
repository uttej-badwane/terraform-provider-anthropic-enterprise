package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccComplianceSettings(t *testing.T) {
	skipUnlessMock(t) // toggling compliance on a live organization is never safe in a test
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "anthropic_compliance_settings" "org" { state = "disabled" }
data "anthropic_compliance_settings" "read" { depends_on = [anthropic_compliance_settings.org] }
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_compliance_settings.org", tfjsonpath.New("state"), knownvalue.StringExact("disabled")),
					statecheck.ExpectKnownValue("anthropic_compliance_settings.org", tfjsonpath.New("id"), knownvalue.StringExact("compliance_settings")),
					statecheck.ExpectKnownValue("data.anthropic_compliance_settings.read", tfjsonpath.New("state"), knownvalue.StringExact("disabled")),
				},
			},
			{
				ResourceName:      "anthropic_compliance_settings.org",
				ImportState:       true,
				ImportStateId:     "compliance_settings",
				ImportStateVerify: true,
			},
			{
				Config: providerConfig + `resource "anthropic_compliance_settings" "org" { state = "enabled" }`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_compliance_settings.org", tfjsonpath.New("state"), knownvalue.StringExact("enabled")),
				},
			},
		},
	})
}

func TestAccComplianceSettings_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      providerConfig + `resource "anthropic_compliance_settings" "org" { state = "maybe" }`,
			ExpectError: regexp.MustCompile(`value must be one of`),
		}},
	})
}
