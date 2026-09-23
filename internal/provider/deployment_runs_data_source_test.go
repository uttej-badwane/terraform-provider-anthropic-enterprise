package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccDeploymentRunsDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_deployment_runs" "all" {}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.all", tfjsonpath.New("runs"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.all", tfjsonpath.New("runs").AtSliceIndex(0).AtMapKey("session_id"), knownvalue.StringExact("sesn_01Example00000000000000")),
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.all", tfjsonpath.New("runs").AtSliceIndex(0).AtMapKey("agent_version"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.all", tfjsonpath.New("runs").AtSliceIndex(0).AtMapKey("trigger_type"), knownvalue.StringExact("schedule")),
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.all", tfjsonpath.New("runs").AtSliceIndex(0).AtMapKey("trigger_scheduled_at"), knownvalue.StringExact("2026-09-23T01:58:00Z")),
				// A successful run has no error, and a JSON null must read as
				// null rather than the literal string "null".
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.all", tfjsonpath.New("runs").AtSliceIndex(0).AtMapKey("error_json"), knownvalue.Null()),
				// A manual trigger carries no scheduled_at.
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.all", tfjsonpath.New("runs").AtSliceIndex(1).AtMapKey("trigger_scheduled_at"), knownvalue.Null()),
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.all", tfjsonpath.New("runs").AtSliceIndex(1).AtMapKey("session_id"), knownvalue.Null()),
			},
		}},
	})
}

// has_error is tri-state. Unset must mean both, which is the case that breaks
// if a bool is forwarded by value rather than by pointer.
func TestAccDeploymentRunsDataSource_hasErrorFilter(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
data "anthropic_deployment_runs" "failed" {
  has_error = true
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_deployment_runs.failed", tfjsonpath.New("runs"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("data.anthropic_deployment_runs.failed", tfjsonpath.New("runs").AtSliceIndex(0).AtMapKey("id"), knownvalue.StringExact("drun_01Failed00000000000000000")),
				},
			},
			{
				Config: providerConfig + `
data "anthropic_deployment_runs" "ok" {
  has_error = false
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_deployment_runs.ok", tfjsonpath.New("runs"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("data.anthropic_deployment_runs.ok", tfjsonpath.New("runs").AtSliceIndex(0).AtMapKey("id"), knownvalue.StringExact("drun_01Succeeded0000000000000")),
				},
			},
		},
	})
}

// The endpoint treats a well-formed id that matches nothing differently from a
// malformed one: the first is an empty list, the second a 400. Checked against
// the live API, where "depl_01DoesNotExist00000000" is rejected outright even
// though it is the right length.
//
// Both cases matter. If a malformed id came back empty, a typo in a
// deployment_id would read as "this schedule has not run yet".
func TestAccDeploymentRunsDataSource_wellFormedUnknownIsEmpty(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_deployment_runs" "none" {
  deployment_id = "depl_016rbVsowJFCKppKiit87jQX"
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_deployment_runs.none", tfjsonpath.New("runs"), knownvalue.ListSizeExact(0)),
			},
		}},
	})
}

func TestAccDeploymentRunsDataSource_malformedDeploymentIDFails(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_deployment_runs" "bad" {
  deployment_id = "not-a-deployment-id"
}
`,
			ExpectError: regexp.MustCompile(`Invalid deployment ID`),
		}},
	})
}

func TestAccDeploymentRunDataSource_error(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_deployment_run" "failed" {
  id = "drun_01Failed00000000000000000"
}

output "failure_type" {
  value = jsondecode(data.anthropic_deployment_run.failed.error_json).type
}
`,
			Check: resource.TestCheckOutput("failure_type", "environment_error"),
		}},
	})
}
