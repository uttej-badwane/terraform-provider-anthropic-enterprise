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

func environmentConfig(name string) string {
	return providerConfig + fmt.Sprintf(`
resource "anthropic_environment" "test" {
  name = %q
  type = "cloud"
  networking = {
    type                   = "limited"
    allowed_hosts          = ["*.example.com"]
    allow_package_managers = true
  }
  packages = {
    pip = ["requests"]
  }
}
`, name)
}

func TestAccEnvironmentResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: environmentConfig(name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_environment.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^env_`))),
					statecheck.ExpectKnownValue("anthropic_environment.test", tfjsonpath.New("scope"), knownvalue.StringExact("organization")),
					statecheck.ExpectKnownValue("anthropic_environment.test", tfjsonpath.New("networking").AtMapKey("allow_mcp_servers"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("anthropic_environment.test", tfjsonpath.New("packages").AtMapKey("pip"), knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("requests")})),
					statecheck.ExpectKnownValue("anthropic_environment.test", tfjsonpath.New("packages").AtMapKey("npm"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_environment.test", tfjsonpath.New("delete_on_destroy"), knownvalue.Bool(true)),
				},
			},
			{Config: environmentConfig(name), PlanOnly: true},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_environment" "test" {
  name        = "%s-renamed"
  description = "CI runners"
  type        = "cloud"
}
data "anthropic_environment" "by_name"   { name = anthropic_environment.test.name }
data "anthropic_environments" "all"      {}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_environment.test", tfjsonpath.New("name"), knownvalue.StringExact(name+"-renamed")),
					statecheck.ExpectKnownValue("anthropic_environment.test", tfjsonpath.New("description"), knownvalue.StringExact("CI runners")),
					statecheck.ExpectKnownValue("data.anthropic_environment.by_name", tfjsonpath.New("networking_type"), knownvalue.StringExact("limited")),
				},
			},
			{
				ResourceName:            "anthropic_environment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_on_destroy", "packages"},
			},
		},
	})
}

func TestAccEnvironmentResource_selfHosted(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_environment" "sh" {
  name              = %q
  type              = "self_hosted"
  delete_on_destroy = false
}`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_environment.sh", tfjsonpath.New("networking"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_environment.sh", tfjsonpath.New("packages"), knownvalue.Null()),
				},
			},
			{Config: providerConfig + fmt.Sprintf(`
resource "anthropic_environment" "sh" {
  name              = %q
  type              = "self_hosted"
  delete_on_destroy = false
}`, name), PlanOnly: true},
		},
	})
}

func TestAccEnvironmentResource_duplicateName(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`
resource "anthropic_environment" "a" {
  name = %[1]q
  type = "cloud"
}
resource "anthropic_environment" "b" {
  name       = %[1]q
  type       = "cloud"
  depends_on = [anthropic_environment.a]
}`, name),
			ExpectError: regexp.MustCompile(`already exists`),
		}},
	})
}

func TestAccEnvironmentResource_packagesNeedAllowance(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_environment" "bad" {
  name       = "tf-acc-bad-packages"
  type       = "cloud"
  networking = { type = "limited" }
  packages   = { npm = ["left-pad"] }
}`,
			ExpectError: regexp.MustCompile(`allow_package_managers`),
		}},
	})
}
