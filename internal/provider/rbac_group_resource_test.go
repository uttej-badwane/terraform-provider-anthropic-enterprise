package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccRBACGroupResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`resource "anthropic_rbac_group" "test" { name = %q }`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_rbac_group.test", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("anthropic_rbac_group.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^rbac_group_`))),
					statecheck.ExpectKnownValue("anthropic_rbac_group.test", tfjsonpath.New("source_type"), knownvalue.StringExact("direct")),
					statecheck.ExpectKnownValue("anthropic_rbac_group.test", tfjsonpath.New("roles"), knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
			{
				ResourceName:      "anthropic_rbac_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: providerConfig + fmt.Sprintf(`resource "anthropic_rbac_group" "test" { name = "%s-2" }`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_rbac_group.test", tfjsonpath.New("name"), knownvalue.StringExact(name+"-2")),
				},
			},
		},
	})
}

func TestAccRBACGroupResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig + `resource "anthropic_rbac_group" "bad" { name = "" }`,
				ExpectError: regexp.MustCompile(`string length must be between 1 and 255`),
			},
		},
	})
}

func TestAccRBACGroupResource_scimReadOnly(t *testing.T) {
	skipUnlessMock(t)
	scimID := testMock.SCIMGroupID()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             providerConfig + `resource "anthropic_rbac_group" "scim" { name = "SCIM Synced" }`,
				ResourceName:       "anthropic_rbac_group.scim",
				ImportState:        true,
				ImportStateId:      scimID,
				ImportStatePersist: true,
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 || states[0].Attributes["source_type"] != "scim" {
						return fmt.Errorf("expected imported scim group, got %+v", states)
					}
					return nil
				},
			},
			{
				Config:      providerConfig + `resource "anthropic_rbac_group" "scim" { name = "Renamed" }`,
				ExpectError: regexp.MustCompile(`SCIM-managed group is read-only`),
			},
			{
				// Forget the SCIM group without deleting it so the post-test destroy has nothing to do.
				Config: providerConfig + `
removed {
  from = anthropic_rbac_group.scim
  lifecycle {
    destroy = false
  }
}`,
			},
		},
	})
}

func TestAccRBACGroupsDataSource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_rbac_group" "test" { name = %q }

data "anthropic_rbac_groups" "all" {
  depends_on = [anthropic_rbac_group.test]
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_rbac_groups.all", tfjsonpath.New("groups"), knownvalue.ListPartial(map[int]knownvalue.Check{
						0: knownvalue.ObjectPartial(map[string]knownvalue.Check{"source_type": knownvalue.StringExact("scim")}),
					})),
				},
			},
		},
	})
}
