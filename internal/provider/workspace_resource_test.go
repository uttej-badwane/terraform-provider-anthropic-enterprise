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

func TestAccWorkspaceResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" {
  name = %q
  tags = { env = "test" }
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^wrkspc_`))),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("tags"), knownvalue.MapExact(map[string]knownvalue.Check{"env": knownvalue.StringExact("test")})),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("archive_on_destroy"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("archived_at"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("allowed_inference_geos"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("workspace_geo"), knownvalue.StringExact("us")),
				},
			},
			{
				ResourceName:            "anthropic_workspace.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"archive_on_destroy"},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" {
  name                   = "%s-2"
  tags                   = { team = "platform" }
  allowed_inference_geos = ["us"]
  default_inference_geo  = "us"
  display_color          = "#112233"
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("name"), knownvalue.StringExact(name+"-2")),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("tags"), knownvalue.MapExact(map[string]knownvalue.Check{"team": knownvalue.StringExact("platform")})),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("allowed_inference_geos"), knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("us")})),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("default_inference_geo"), knownvalue.StringExact("us")),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("display_color"), knownvalue.StringExact("#112233")),
				},
			},
			{
				// Removing tags entirely must clear them, not leave stale state.
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" {
  name = "%s-2"
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("tags"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("allowed_inference_geos"), knownvalue.Null()),
				},
			},
		},
	})
}

func TestAccWorkspaceResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig + `resource "anthropic_workspace" "bad" { name = "" }`,
				ExpectError: regexp.MustCompile(`string length must be between 1 and 40`),
			},
			{
				Config: providerConfig + `
resource "anthropic_workspace" "bad" {
  name = "x"
  tags = { anthropic_owner = "y" }
}`,
				ExpectError: regexp.MustCompile(`must not start with "anthropic"`),
			},
		},
	})
}

func TestAccWorkspaceResource_archivedOutOfBand(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`resource "anthropic_workspace" "test" { name = %q }`, name),
			},
			{
				PreConfig: func() {
					for _, ws := range testMock.Workspaces() {
						if ws.Name == name {
							testMock.ArchiveWorkspaceOutOfBand(ws.ID)
						}
					}
				},
				Config:             providerConfig + fmt.Sprintf(`resource "anthropic_workspace" "test" { name = %q }`, name),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true, // archived out of band -> removed from state -> plan recreates
			},
		},
	})
}

func TestAccWorkspaceDataSources(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" { name = %q }

data "anthropic_organization" "org" {}

data "anthropic_workspace" "by_id"   { id   = anthropic_workspace.test.id }
data "anthropic_workspace" "by_name" { name = anthropic_workspace.test.name }
data "anthropic_workspaces" "all"    {}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_organization.org", tfjsonpath.New("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue("data.anthropic_workspace.by_id", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("data.anthropic_workspace.by_name", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("data.anthropic_workspaces.all", tfjsonpath.New("workspaces"), knownvalue.ListPartial(map[int]knownvalue.Check{})),
				},
			},
		},
	})
}

func TestAccWorkspaceDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig + `data "anthropic_workspace" "bad" {}`,
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured`),
			},
		},
	})
}
