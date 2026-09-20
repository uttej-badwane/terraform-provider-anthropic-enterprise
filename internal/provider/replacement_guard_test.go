package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Changing an updatable attribute must plan as an update. A replacement
// destroys the object and creates a new one with a new id, which for most of
// these is not recoverable: workspaces, service accounts, federation issuers
// and rules, agents and deployments archive rather than delete, and anything
// referencing the old id is left pointing at nothing.
//
// This is not hypothetical. Renaming an anthropic_external_key replaced the
// registration in v0.5.0, because an optional-and-computed attribute under an
// object carrying RequiresReplace re-planned as unknown whenever anything else
// changed. The existing test renamed the key and asserted the new name, and
// passed, because the recreated key had the new name. Only the resource's
// Update showing zero coverage gave it away.
//
// Asserting the id is unchanged is the part that would have caught it: the name
// was right either way, the id was not.

type replacementGuardCase struct {
	// name is the resource address the plan check applies to.
	address string
	// base and updated differ by one updatable attribute.
	base    string
	updated string
}

func TestAccUpdatesDoNotReplace(t *testing.T) {
	skipUnlessMock(t)

	suffix := acctest.RandomWithPrefix("tf-acc")
	cases := map[string]replacementGuardCase{
		"workspace": {
			address: "anthropic_workspace.test",
			base:    fmt.Sprintf(`resource "anthropic_workspace" "test" { name = %q }`, suffix),
			updated: fmt.Sprintf(`resource "anthropic_workspace" "test" { name = "%s-renamed" }`, suffix),
		},
		"service_account": {
			address: "anthropic_service_account.test",
			base: fmt.Sprintf(`resource "anthropic_service_account" "test" {
  name        = %q
  description = "before"
}`, suffix),
			updated: fmt.Sprintf(`resource "anthropic_service_account" "test" {
  name        = %q
  description = "after"
}`, suffix),
		},
		"federation_issuer": {
			address: "anthropic_federation_issuer.test",
			base: fmt.Sprintf(`resource "anthropic_federation_issuer" "test" {
  name       = %q
  issuer_url = "https://token.actions.githubusercontent.com"
}`, suffix),
			updated: fmt.Sprintf(`resource "anthropic_federation_issuer" "test" {
  name       = "%s-renamed"
  issuer_url = "https://token.actions.githubusercontent.com"
}`, suffix),
		},
		"rbac_group": {
			address: "anthropic_rbac_group.test",
			base:    fmt.Sprintf(`resource "anthropic_rbac_group" "test" { name = %q }`, suffix),
			updated: fmt.Sprintf(`resource "anthropic_rbac_group" "test" { name = "%s-renamed" }`, suffix),
		},
		"memory_store": {
			address: "anthropic_memory_store.test",
			base: fmt.Sprintf(`resource "anthropic_memory_store" "test" {
  name        = %q
  description = "before"
}`, suffix),
			updated: fmt.Sprintf(`resource "anthropic_memory_store" "test" {
  name        = %q
  description = "after"
}`, suffix),
		},
		"environment": {
			address: "anthropic_environment.test",
			base: fmt.Sprintf(`resource "anthropic_environment" "test" {
  name        = %q
  description = "before"
  type        = "cloud"
}`, suffix),
			updated: fmt.Sprintf(`resource "anthropic_environment" "test" {
  name        = %q
  description = "after"
  type        = "cloud"
}`, suffix),
		},
		// Archive-only, so a replacement is permanent. The federation rule
		// also re-reads itself after a write, which is where an earlier bug
		// lived, so it is worth pinning.
		"federation_rule": {
			address: "anthropic_federation_rule.test",
			base:    federationRuleGuardConfig(suffix, "workspace:inference"),
			updated: federationRuleGuardConfig(suffix, "workspace:developer"),
		},
		"agent": {
			address: "anthropic_agent.test",
			base: fmt.Sprintf(`resource "anthropic_agent" "test" {
  name        = %q
  model       = "claude-sonnet-5"
  description = "before"
}`, suffix),
			updated: fmt.Sprintf(`resource "anthropic_agent" "test" {
  name        = %q
  model       = "claude-sonnet-5"
  description = "after"
}`, suffix),
		},
		"vault": {
			address: "anthropic_vault.test",
			base:    fmt.Sprintf(`resource "anthropic_vault" "test" { display_name = %q }`, suffix),
			updated: fmt.Sprintf(`resource "anthropic_vault" "test" { display_name = "%s-renamed" }`, suffix),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var firstID string
			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: providerConfig + tc.base,
						Check: resource.TestCheckResourceAttrWith(tc.address, "id", func(v string) error {
							firstID = v
							return nil
						}),
					},
					{
						Config: providerConfig + tc.updated,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectResourceAction(tc.address, plancheck.ResourceActionUpdate),
							},
						},
						// The id surviving is what a user actually depends on.
						// A replacement would satisfy every attribute check
						// while silently handing back a different object.
						Check: resource.TestCheckResourceAttrWith(tc.address, "id", func(v string) error {
							if v != firstID {
								return fmt.Errorf("id changed from %q to %q: the object was replaced, not updated", firstID, v)
							}
							return nil
						}),
					},
				},
			})
		})
	}
}

// federationRuleGuardConfig builds a rule with its issuer, service account and
// workspace, since a rule cannot exist without them.
func federationRuleGuardConfig(suffix, scope string) string {
	return fmt.Sprintf(`
resource "anthropic_workspace" "rule" { name = "%[1]s-ws" }

resource "anthropic_service_account" "rule" {
  name = "%[1]s-sa"
}

resource "anthropic_federation_issuer" "rule" {
  name       = "%[1]s-iss"
  issuer_url = "https://token.actions.githubusercontent.com"
}

resource "anthropic_federation_rule" "test" {
  name               = "%[1]s-rule"
  issuer_id          = anthropic_federation_issuer.rule.id
  service_account_id = anthropic_service_account.rule.id
  oauth_scope        = %[2]q
  workspace_id       = anthropic_workspace.rule.id

  match = {
    subject_prefix = "repo:example-org/example-repo:ref:refs/heads/main"
  }
}
`, suffix, scope)
}
