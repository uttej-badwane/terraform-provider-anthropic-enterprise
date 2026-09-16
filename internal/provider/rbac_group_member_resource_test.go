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

func TestAccRBACGroupMemberResource(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	user := testMock.UserByEmail("user@example.com")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_rbac_group" "test" { name = %q }

resource "anthropic_rbac_group_member" "test" {
  group_id = anthropic_rbac_group.test.id
  user_id  = %q
}
`, name, user.ID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_rbac_group_member.test", tfjsonpath.New("user_id"), knownvalue.StringExact(user.ID)),
					statecheck.ExpectKnownValue("anthropic_rbac_group_member.test", tfjsonpath.New("email"), knownvalue.StringExact("user@example.com")),
					statecheck.ExpectKnownValue("anthropic_rbac_group_member.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^rbac_group_[^/]+/user_`))),
				},
			},
			{
				ResourceName:      "anthropic_rbac_group_member.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccRBACGroupMemberResource_scim(t *testing.T) {
	skipUnlessMock(t)
	user := testMock.UserByEmail("user@example.com")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_rbac_group_member" "scim" {
  group_id = %q
  user_id  = %q
}
`, testMock.SCIMGroupID(), user.ID),
				ExpectError: regexp.MustCompile(`SCIM-managed group membership cannot be modified`),
			},
		},
	})
}
