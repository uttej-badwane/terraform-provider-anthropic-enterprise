package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// Live Managed Agents tier. Runs only with ANTHROPIC_ACC_LIVE=1 and a regular
// workspace API key in ANTHROPIC_API_KEY. It never starts a session and never
// schedules a deployment, so it consumes no inference credits: control-plane
// calls are free. Every object is prefixed tf-acc- and destroyed at the end.

func skipUnlessLiveAgents(t *testing.T) {
	t.Helper()
	skipUnlessLive(t)
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("needs ANTHROPIC_API_KEY (regular workspace key) for Managed Agents")
	}
}

func writeLiveSkill(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tf-acc-release-notes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := "---\nname: tf-acc-release-notes\ndescription: Draft release notes from a changelog. Acceptance test fixture.\n---\n# Release notes\n\nSummarize the changelog entries.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAccLiveAgentsLifecycle(t *testing.T) {
	skipUnlessLiveAgents(t)
	suffix := acctest.RandString(6)
	skillDir := writeLiveSkill(t)

	config := func(system string) string {
		return providerConfig + fmt.Sprintf(`
resource "anthropic_environment" "test" {
  name = "tf-acc-env-%[1]s"
  type = "cloud"
  networking = {
    type          = "limited"
    allowed_hosts = ["*.example.com"]
  }
}

resource "anthropic_memory_store" "test" {
  name        = "tf-acc-memory-%[1]s"
  description = "acceptance test"
}

resource "anthropic_vault" "test" {
  display_name = "tf-acc-vault-%[1]s"
}

resource "anthropic_vault_credential" "test" {
  vault_id     = anthropic_vault.test.id
  display_name = "tf-acc-credential"
  environment_variable = {
    secret_name     = "TF_ACC_TOKEN"
    secret_value    = "not-a-real-secret-%[1]s"
    networking_type = "limited"
    allowed_hosts   = ["api.example.com"]
  }
}

resource "anthropic_skill" "test" {
  source_dir = %[3]q
}

resource "anthropic_agent" "test" {
  name   = "tf-acc-agent-%[1]s"
  model  = "claude-sonnet-5"
  system = %[2]q
  tools  = jsonencode([{ type = "agent_toolset_20260401", configs = [{ name = "bash", permission_policy = { type = "always_ask" } }] }])
  skills = [{ type = "custom", skill_id = anthropic_skill.test.id }]
  metadata = { managed_by = "terraform-acceptance-test" }
}

# No schedule and paused: this deployment can never run, so it costs nothing.
resource "anthropic_deployment" "test" {
  name                       = "tf-acc-deploy-%[1]s"
  agent_id                   = anthropic_agent.test.id
  agent_version              = anthropic_agent.test.version
  environment_id             = anthropic_environment.test.id
  initial_events             = jsonencode([{ type = "user.message", content = [{ type = "text", text = "Acceptance test placeholder." }] }])
  vault_ids                  = [anthropic_vault.test.id]
  budget_max_list_cost_cents = "100"
  paused                     = true
  memory_stores = [{
    memory_store_id = anthropic_memory_store.test.id
    access          = "read_only"
  }]
}

data "anthropic_agent_versions" "test" { agent_id = anthropic_agent.test.id }
`, suffix, system, skillDir)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("You are an acceptance test agent."),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("status"), knownvalue.StringExact("paused")),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("agent_version"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("anthropic_skill.test", tfjsonpath.New("latest_version_id"), knownvalue.NotNull()),
				},
			},
			{
				// The whole graph must be stable against the real API's normalization.
				Config:           config("You are an acceptance test agent."),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				Config: config("You are an acceptance test agent. Be terse."),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("version"), knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue("anthropic_deployment.test", tfjsonpath.New("agent_version"), knownvalue.Int64Exact(2)),
				},
			},
			{ResourceName: "anthropic_environment.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"delete_on_destroy"}},
			{ResourceName: "anthropic_memory_store.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"delete_on_destroy"}},
			{ResourceName: "anthropic_vault.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"delete_on_destroy"}},
			{ResourceName: "anthropic_agent.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"archive_on_destroy", "tools", "multiagent", "skills"}},
			{ResourceName: "anthropic_deployment.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"archive_on_destroy", "initial_events", "github_repositories", "paused"}},
		},
	})
}

func TestAccLiveAgentsReadOnly(t *testing.T) {
	skipUnlessLiveAgents(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_agents"        "all" { include_archived = true }
data "anthropic_environments"  "all" { include_archived = true }
data "anthropic_vaults"        "all" {}
data "anthropic_deployments"   "all" { include_archived = true }
data "anthropic_memory_stores" "all" {}
data "anthropic_skills"        "anthropic" { source = "anthropic" }
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_skills.anthropic", tfjsonpath.New("skills"), knownvalue.ListPartial(map[int]knownvalue.Check{})),
			},
		}},
	})
}
