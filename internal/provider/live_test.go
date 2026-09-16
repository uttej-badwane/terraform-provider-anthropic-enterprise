package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// Live tests run only with ANTHROPIC_ACC_LIVE=1 and are restricted to
// read-only data sources plus the lifecycle of tf-acc- prefixed objects.
// Nothing here sends invitations, changes users or touches spend limits.

func TestAccLiveReadOnlyDataSources(t *testing.T) {
	skipUnlessLive(t)
	if os.Getenv("ANTHROPIC_ADMIN_API_KEY") == "" && os.Getenv("ANTHROPIC_AUTH_TOKEN") == "" {
		t.Skip("needs ANTHROPIC_ADMIN_API_KEY or ANTHROPIC_AUTH_TOKEN")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_organization" "org" {}
data "anthropic_workspaces"   "all" { include_archived = true }
data "anthropic_users"        "all" {}
data "anthropic_api_keys"     "all" {}
data "anthropic_rate_limits"  "all" {}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_organization.org", tfjsonpath.New("id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.anthropic_organization.org", tfjsonpath.New("name"), knownvalue.NotNull()),
			},
		}},
	})
}

func TestAccLiveWorkspaceLifecycle(t *testing.T) {
	skipUnlessLive(t)
	if os.Getenv("ANTHROPIC_ADMIN_API_KEY") == "" && os.Getenv("ANTHROPIC_AUTH_TOKEN") == "" {
		t.Skip("needs ANTHROPIC_ADMIN_API_KEY or ANTHROPIC_AUTH_TOKEN")
	}
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" {
  name = %q
  tags = { managed_by = "terraform-acceptance-test" }
}
data "anthropic_workspace" "by_id" { id = anthropic_workspace.test.id }
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("data.anthropic_workspace.by_id", tfjsonpath.New("name"), knownvalue.StringExact(name)),
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
  name = "%s-renamed"
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("name"), knownvalue.StringExact(name+"-renamed")),
					statecheck.ExpectKnownValue("anthropic_workspace.test", tfjsonpath.New("tags"), knownvalue.Null()),
				},
			},
		},
	})
}

func TestAccLiveEnterpriseReadOnly(t *testing.T) {
	skipUnlessLive(t)
	if os.Getenv("ANTHROPIC_ENTERPRISE_API_KEY") == "" {
		t.Skip("needs ANTHROPIC_ENTERPRISE_API_KEY")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_rbac_groups" "all" {}
data "anthropic_rbac_roles"  "all" {}
`,
		}},
	})
}

func TestAccLiveRBACGroupLifecycle(t *testing.T) {
	skipUnlessLive(t)
	if os.Getenv("ANTHROPIC_ENTERPRISE_API_KEY") == "" {
		t.Skip("needs ANTHROPIC_ENTERPRISE_API_KEY")
	}
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: providerConfig + fmt.Sprintf(`resource "anthropic_rbac_group" "test" { name = %q }`, name)},
			{ResourceName: "anthropic_rbac_group.test", ImportState: true, ImportStateVerify: true},
			{Config: providerConfig + fmt.Sprintf(`resource "anthropic_rbac_group" "test" { name = "%s-renamed" }`, name)},
		},
	})
}
