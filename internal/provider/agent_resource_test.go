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

func agentConfig(name, system string) string {
	return providerConfig + fmt.Sprintf(`
resource "anthropic_agent" "test" {
  name  = %q
  model = "claude-opus-5"
  %s
  tools = jsonencode([
    { type = "agent_toolset_20260401", configs = [{ name = "bash", permission_policy = { type = "always_allow" } }] },
    { type = "mcp_toolset", mcp_server_name = "docs" },
  ])
  mcp_servers = [{ name = "docs", url = "https://mcp.example.com/sse" }]
  skills      = [{ type = "anthropic", skill_id = "xlsx" }]
  metadata    = { team = "platform" }
}
`, name, system)
}

func TestAccAgentResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: agentConfig(name, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^agent_`))),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("version"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("model_effort"), knownvalue.StringExact("high")),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("model_speed"), knownvalue.StringExact("standard")),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("skills").AtSliceIndex(0).AtMapKey("resolved_version"), knownvalue.StringExact("20251013")),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("skills").AtSliceIndex(0).AtMapKey("version"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("archived_at"), knownvalue.Null()),
				},
			},
			{
				// Same config: server-side normalization must not produce a diff.
				Config:   agentConfig(name, ""),
				PlanOnly: true,
			},
			{
				Config: agentConfig(name, `system = "Be terse."`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("version"), knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("system"), knownvalue.StringExact("Be terse.")),
				},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_agent" "test" {
  name         = %q
  model        = "claude-sonnet-5"
  model_effort = "low"
  metadata     = { owner = "infra" }
}
data "anthropic_agent" "read"          { id = anthropic_agent.test.id }
data "anthropic_agents" "all"          {}
data "anthropic_agent_versions" "hist" { agent_id = anthropic_agent.test.id }
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("version"), knownvalue.Int64Exact(3)),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("model_effort"), knownvalue.StringExact("low")),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("tools"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("mcp_servers"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_agent.test", tfjsonpath.New("metadata"), knownvalue.MapExact(map[string]knownvalue.Check{"owner": knownvalue.StringExact("infra")})),
					statecheck.ExpectKnownValue("data.anthropic_agent.read", tfjsonpath.New("model"), knownvalue.StringExact("claude-sonnet-5")),
					statecheck.ExpectKnownValue("data.anthropic_agent_versions.hist", tfjsonpath.New("versions").AtSliceIndex(0).AtMapKey("version"), knownvalue.Int64Exact(3)),
				},
			},
			{
				ResourceName:            "anthropic_agent.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"archive_on_destroy", "tools", "multiagent", "skills"},
			},
		},
	})
}

func TestAccAgentResource_orphanMCPServer(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_agent" "bad" {
  name        = "bad"
  model       = "claude-opus-5"
  mcp_servers = [{ name = "orphan", url = "https://mcp.example.com" }]
}`,
			ExpectError: regexp.MustCompile(`Unreferenced MCP server`),
		}},
	})
}

func TestAccAgentResource_badEffort(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_agent" "bad" {
  name         = "bad"
  model        = "claude-opus-5"
  model_effort = "extreme"
}`,
			ExpectError: regexp.MustCompile(`value must be one of`),
		}},
	})
}
