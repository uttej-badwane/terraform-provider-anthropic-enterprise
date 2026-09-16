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

func TestAccWorkspaceMemberResource(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	userID := testMock.UserByEmail("dev@example.com").ID
	cfg := func(role string) string {
		return providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" { name = %q }

resource "anthropic_workspace_member" "test" {
  workspace_id   = anthropic_workspace.test.id
  user_id        = %q
  workspace_role = %q
}
`, name, userID, role)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("workspace_user"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace_member.test", tfjsonpath.New("workspace_role"), knownvalue.StringExact("workspace_user")),
					statecheck.ExpectKnownValue("anthropic_workspace_member.test", tfjsonpath.New("user_id"), knownvalue.StringExact(userID)),
					statecheck.ExpectKnownValue("anthropic_workspace_member.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^wrkspc_.*/user_`))),
				},
			},
			{
				ResourceName:      "anthropic_workspace_member.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: cfg("workspace_developer"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace_member.test", tfjsonpath.New("workspace_role"), knownvalue.StringExact("workspace_developer")),
				},
			},
		},
	})
}

func TestAccWorkspaceMemberResource_unknownUser(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `resource "anthropic_workspace_member" "bad" {
  workspace_id   = "wrkspc_x"
  user_id        = "user_x"
  workspace_role = "workspace_billing"
}`,
				ExpectError: regexp.MustCompile(`workspace_role value must be one of`),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" { name = %q }

resource "anthropic_workspace_member" "test" {
  workspace_id   = anthropic_workspace.test.id
  user_id        = "user_doesnotexist"
  workspace_role = "workspace_user"
}
`, name),
				ExpectError: regexp.MustCompile(`(?s)404 user not\s+found`),
			},
		},
	})
}
