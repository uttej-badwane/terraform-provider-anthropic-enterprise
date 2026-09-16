package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const deploymentEvents = `[{"type":"user.message","content":[{"type":"text","text":"Summarize yesterday's incidents."}]}]`

func TestAccDeploymentResource(t *testing.T) {
	skipUnlessMock(t)
	agent := testMock.SeedAgent(acctest.RandomWithPrefix("tf-acc"))
	env := testMock.SeedEnvironment(acctest.RandomWithPrefix("tf-acc"))
	name := acctest.RandomWithPrefix("tf-acc")
	config := func(paused bool, schedule bool) string {
		sched := ""
		if schedule {
			sched = `
  schedule = {
    cron_expression = "0 6 * * *"
    timezone        = "UTC"
  }`
		}
		return providerConfig + fmt.Sprintf(`
resource "anthropic_vault" "test" { display_name = %q }

resource "anthropic_deployment" "test" {
  name           = %q
  agent_id       = %q
  environment_id = %q
  initial_events = jsonencode(%s)
  paused         = %t
  vault_ids      = [anthropic_vault.test.id]
  github_repositories = [{
    url                 = "https://github.com/example-org/example-repo"
    authorization_token = "example-github-token"
    checkout_branch     = "main"
  }]
  budget_max_list_cost_cents = "50000"
  metadata                   = { owner = "platform" }%s
}

data "anthropic_deployment"  "one" { id = anthropic_deployment.test.id }
data "anthropic_deployments" "all" {
  agent_id   = %q
  depends_on = [anthropic_deployment.test]
}
`, name, name, agent.ID, env.ID, deploymentEvents, paused, sched, agent.ID)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(false, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^depl_`))),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("status"), knownvalue.StringExact("active")),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("agent_version"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("upcoming_runs_at"), knownvalue.ListSizeExact(3)),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("github_repositories").AtSliceIndex(0).AtMapKey("authorization_token"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("github_repositories").AtSliceIndex(0).AtMapKey("checkout_branch"), knownvalue.StringExact("main")),
					statecheck.ExpectKnownValue("data.anthropic_deployment.one", tfjsonpath.New("schedule_cron_expression"), knownvalue.StringExact("0 6 * * *")),
					statecheck.ExpectKnownValue("data.anthropic_deployments.all", tfjsonpath.New("deployments"), knownvalue.ListSizeExact(1)),
				},
			},
			{
				Config:           config(false, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: config(true, true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("status"), knownvalue.StringExact("paused")),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("paused_reason_type"), knownvalue.StringExact("manual")),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("upcoming_runs_at"), knownvalue.ListSizeExact(0)),
				},
			},
			{
				Config: config(false, false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("status"), knownvalue.StringExact("active")),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("schedule"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("upcoming_runs_at"), knownvalue.ListSizeExact(0)),
				},
			},
			{
				ResourceName:            "anthropic_deployment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"archive_on_destroy", "initial_events", "github_repositories"},
			},
		},
	})
}

func TestAccDeploymentResource_validation(t *testing.T) {
	skipUnlessMock(t)
	agent := testMock.SeedAgent(acctest.RandomWithPrefix("tf-acc"))
	env := testMock.SeedEnvironment(acctest.RandomWithPrefix("tf-acc"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`
resource "anthropic_deployment" "bad" {
  name           = "bad"
  agent_id       = %q
  environment_id = %q
  initial_events = "[]"
}
`, agent.ID, env.ID),
			ExpectError: regexp.MustCompile(`initial_events must be an array of 1-50 events`),
		}},
	})
}
