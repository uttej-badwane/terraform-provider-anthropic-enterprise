package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccUserResource_import(t *testing.T) {
	skipUnlessMock(t)
	dev := testMock.UserByEmail("dev@example.com")
	cfg := func(role string) string {
		return providerConfig + fmt.Sprintf(`resource "anthropic_user" "test" { role = %q }`, role)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             cfg("developer"),
				ResourceName:       "anthropic_user.test",
				ImportState:        true,
				ImportStateId:      dev.ID,
				ImportStatePersist: true,
			},
			{
				Config: cfg("developer"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_user.test", tfjsonpath.New("email"), knownvalue.StringExact("dev@example.com")),
					statecheck.ExpectKnownValue("anthropic_user.test", tfjsonpath.New("remove_on_destroy"), knownvalue.Bool(false)),
				},
			},
			{
				Config: cfg("user"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_user.test", tfjsonpath.New("role"), knownvalue.StringExact("user")),
				},
			},
			{
				Config: cfg("developer"),
			},
		},
	})
}

func TestAccUserResource_createErrors(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig + `resource "anthropic_user" "new" { role = "user" }`,
				ExpectError: regexp.MustCompile(`Users cannot be created`),
			},
		},
	})
}

func TestAccUserDataSources(t *testing.T) {
	skipUnlessMock(t)
	dev := testMock.UserByEmail("dev@example.com")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
data "anthropic_user" "by_id"    { id    = %q }
data "anthropic_user" "by_email" { email = "DEV@example.com" }
data "anthropic_users" "all"     {}
data "anthropic_users" "devs"    { roles = ["developer"] }
`, dev.ID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_user.by_id", tfjsonpath.New("email"), knownvalue.StringExact("dev@example.com")),
					statecheck.ExpectKnownValue("data.anthropic_user.by_email", tfjsonpath.New("id"), knownvalue.StringExact(dev.ID)),
					statecheck.ExpectKnownValue("data.anthropic_users.devs", tfjsonpath.New("users").AtSliceIndex(0).AtMapKey("role"), knownvalue.StringExact("developer")),
				},
			},
			{
				Config:      providerConfig + `data "anthropic_user" "missing" { email = "nobody@example.com" }`,
				ExpectError: regexp.MustCompile(`User not found`),
			},
		},
	})
}
