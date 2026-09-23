package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// The API reports who created, last updated and archived service accounts,
// federation issuers and federation rules. Checked against a live
// organization before exposing them: across 21 service accounts, 20 issuers
// and 19 rules, all three fields were present and populated.
//
// The client already decoded these; the provider simply never surfaced them.
//
// archived_by_actor_id must be null on a live object. Asserting that matters as
// much as asserting the other two are set: a value there would mean the field
// was being filled from somewhere it should not be.
var actorIDPattern = regexp.MustCompile(`^(user|svac)_`)

func actorChecks(addr string) []statecheck.StateCheck {
	return []statecheck.StateCheck{
		statecheck.ExpectKnownValue(addr, tfjsonpath.New("created_by_actor_id"), knownvalue.StringRegexp(actorIDPattern)),
		statecheck.ExpectKnownValue(addr, tfjsonpath.New("updated_by_actor_id"), knownvalue.StringRegexp(actorIDPattern)),
		statecheck.ExpectKnownValue(addr, tfjsonpath.New("archived_by_actor_id"), knownvalue.Null()),
	}
}

func TestAccAuditActorFields(t *testing.T) {
	skipUnlessMock(t)
	suffix := acctest.RandomWithPrefix("tf-acc")

	var checks []statecheck.StateCheck
	for _, addr := range []string{
		"anthropic_service_account.rule",
		"anthropic_federation_issuer.rule",
		"anthropic_federation_rule.test",
	} {
		checks = append(checks, actorChecks(addr)...)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            providerConfig + federationRuleGuardConfig(suffix, "workspace:inference"),
				ConfigStateChecks: checks,
			},
			// An update must leave the fields readable rather than stuck at a
			// stale or unknown value, and must still not populate archived_by.
			{
				Config:            providerConfig + federationRuleGuardConfig(suffix, "workspace:developer"),
				ConfigStateChecks: actorChecks("anthropic_federation_rule.test"),
			},
		},
	})
}

// The data sources share their object shape with the resources, so reading
// through them must expose the same three fields.
func TestAccAuditActorFields_dataSources(t *testing.T) {
	skipUnlessMock(t)
	suffix := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + federationRuleGuardConfig(suffix, "workspace:inference") + `
data "anthropic_service_account" "sa" {
  id = anthropic_service_account.rule.id
}

data "anthropic_federation_issuer" "iss" {
  id = anthropic_federation_issuer.rule.id
}

data "anthropic_federation_rule" "rule" {
  id = anthropic_federation_rule.test.id
}
`,
			ConfigStateChecks: append(append(
				actorChecks("data.anthropic_service_account.sa"),
				actorChecks("data.anthropic_federation_issuer.iss")...),
				actorChecks("data.anthropic_federation_rule.rule")...),
		}},
	})
}
