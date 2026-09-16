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

func TestAccInviteResource(t *testing.T) {
	skipUnlessMock(t)
	email := acctest.RandomWithPrefix("tf-acc") + "@example.com"
	cfg := providerConfig + fmt.Sprintf(`
resource "anthropic_invite" "test" {
  email = %q
  role  = "developer"
}
`, email)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_invite.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^invite_`))),
					statecheck.ExpectKnownValue("anthropic_invite.test", tfjsonpath.New("status"), knownvalue.StringExact("pending")),
					statecheck.ExpectKnownValue("anthropic_invite.test", tfjsonpath.New("accepted_at"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_invite.test", tfjsonpath.New("rbac_group_ids"), knownvalue.Null()),
				},
			},
			{
				ResourceName:      "anthropic_invite.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Accepted out of band: status updates, no diff, destroy is a no-op.
				PreConfig: func() {
					for _, inv := range testMock.Invites() {
						if inv.Email == email {
							testMock.AcceptInvite(inv.ID)
						}
					}
				},
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_invite.test", tfjsonpath.New("status"), knownvalue.StringExact("accepted")),
					statecheck.ExpectKnownValue("anthropic_invite.test", tfjsonpath.New("accepted_at"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestAccInviteResource_duplicate(t *testing.T) {
	skipUnlessMock(t)
	email := acctest.RandomWithPrefix("tf-acc") + "@example.com"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_invite" "a" {
  email = %q
  role  = "user"
}
resource "anthropic_invite" "b" {
  email      = anthropic_invite.a.email
  role       = "user"
  depends_on = [anthropic_invite.a]
}
`, email),
				ExpectError: regexp.MustCompile(`409`),
			},
		},
	})
}
