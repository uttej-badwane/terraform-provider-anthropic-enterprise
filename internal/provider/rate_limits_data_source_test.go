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

func TestAccRateLimitsDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
data "anthropic_rate_limits" "all" {}
data "anthropic_rate_limits" "sonnet" { model = "claude-sonnet-5" }
data "anthropic_rate_limits" "batch"  { group_type = "batch" }
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_rate_limits.all", tfjsonpath.New("rate_limits"), knownvalue.ListSizeExact(2)),
					statecheck.ExpectKnownValue("data.anthropic_rate_limits.sonnet", tfjsonpath.New("rate_limits").AtSliceIndex(0).AtMapKey("group_type"), knownvalue.StringExact("model_group")),
					statecheck.ExpectKnownValue("data.anthropic_rate_limits.sonnet", tfjsonpath.New("rate_limits").AtSliceIndex(0).AtMapKey("limits").AtSliceIndex(0).AtMapKey("value"), knownvalue.Int64Exact(4000)),
					statecheck.ExpectKnownValue("data.anthropic_rate_limits.batch", tfjsonpath.New("rate_limits").AtSliceIndex(0).AtMapKey("models"), knownvalue.Null()),
				},
			},
			{
				Config:      providerConfig + `data "anthropic_rate_limits" "bad" { model = "no-such-model" }`,
				ExpectError: regexp.MustCompile(`404`),
			},
		},
	})
}

func TestAccWorkspaceRateLimitsDataSource(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	cfg := providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" { name = %q }

data "anthropic_workspace_rate_limits" "test" {
  workspace_id = anthropic_workspace.test.id
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_workspace_rate_limits.test", tfjsonpath.New("rate_limits"), knownvalue.ListSizeExact(0)),
				},
			},
			{
				PreConfig: func() {
					for _, ws := range testMock.Workspaces() {
						if ws.Name == name {
							testMock.SeedWorkspaceRateLimit(ws.ID)
						}
					}
				},
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_workspace_rate_limits.test", tfjsonpath.New("rate_limits"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("data.anthropic_workspace_rate_limits.test", tfjsonpath.New("rate_limits").AtSliceIndex(0).AtMapKey("limits").AtSliceIndex(0).AtMapKey("org_limit"), knownvalue.Int64Exact(4000)),
					statecheck.ExpectKnownValue("data.anthropic_workspace_rate_limits.test", tfjsonpath.New("rate_limits").AtSliceIndex(0).AtMapKey("rate_limit_id"), knownvalue.StringExact("rl_model_group_sonnet")),
				},
			},
		},
	})
}
