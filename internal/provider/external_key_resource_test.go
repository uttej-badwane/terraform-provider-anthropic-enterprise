package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const testKMSARN = "arn:aws:kms:us-east-1:123456789012:key/11111111-2222-3333-4444-555555555555"

func TestAccExternalKeyResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_external_key" "test" {
  display_name = %q
  provider_config = {
    type    = "aws"
    kms_arn = %q
  }
}
`, name, testKMSARN),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_external_key.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^ekey_`))),
					statecheck.ExpectKnownValue("anthropic_external_key.test", tfjsonpath.New("geo"), knownvalue.StringExact("us")),
					statecheck.ExpectKnownValue("anthropic_external_key.test", tfjsonpath.New("attachment_type"), knownvalue.StringExact("unattached")),
					statecheck.ExpectKnownValue("anthropic_external_key.test", tfjsonpath.New("provider_config").AtMapKey("region"), knownvalue.StringExact("us-east-1")),
					statecheck.ExpectKnownValue("anthropic_external_key.test", tfjsonpath.New("provider_config").AtMapKey("kms_arn"), knownvalue.StringExact(testKMSARN)),
					statecheck.ExpectKnownValue("anthropic_external_key.test", tfjsonpath.New("validate_on_create"), knownvalue.Bool(false)),
				},
			},
			{
				ResourceName:            "anthropic_external_key.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"validate_on_create"},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_external_key" "test" {
  display_name = "%s-renamed"
  provider_config = {
    type    = "aws"
    kms_arn = %q
  }
}
data "anthropic_external_keys" "all" { depends_on = [anthropic_external_key.test] }
`, name, testKMSARN),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_external_key.test", tfjsonpath.New("display_name"), knownvalue.StringExact(name+"-renamed")),
					statecheck.ExpectKnownValue("data.anthropic_external_keys.all", tfjsonpath.New("external_keys"), knownvalue.ListPartial(map[int]knownvalue.Check{})),
				},
			},
		},
	})
}

func TestAccExternalKeyResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "anthropic_external_key" "bad" {
  provider_config = { type = "gcp" }
}`,
				ExpectError: regexp.MustCompile(`provider_config.key_name is required when provider_config.type is "gcp"`),
			},
		},
	})
}

func TestAccExternalKeyResource_validateOnCreateFailure(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_external_key" "bad" {
  display_name       = "invalid-key"
  validate_on_create = true
  provider_config = {
    type    = "aws"
    kms_arn = %q
  }
}`, testKMSARN),
				ExpectError: regexp.MustCompile(`External key validation failed`),
			},
		},
	})
}
