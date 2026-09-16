package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func slugName() string { return strings.ToLower(acctest.RandomWithPrefix("tf-acc")) }

func TestAccServiceAccountResource(t *testing.T) {
	name := slugName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_service_account" "test" {
  name        = %q
  description = "CI deploy bot"
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_service_account.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^svac_`))),
					statecheck.ExpectKnownValue("anthropic_service_account.test", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("anthropic_service_account.test", tfjsonpath.New("description"), knownvalue.StringExact("CI deploy bot")),
					statecheck.ExpectKnownValue("anthropic_service_account.test", tfjsonpath.New("organization_role"), knownvalue.StringExact("developer")),
					statecheck.ExpectKnownValue("anthropic_service_account.test", tfjsonpath.New("archive_on_destroy"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("anthropic_service_account.test", tfjsonpath.New("archived_at"), knownvalue.Null()),
				},
			},
			{
				ResourceName:            "anthropic_service_account.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"archive_on_destroy"},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_service_account" "test" {
  name              = %q
  organization_role = "admin"
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_service_account.test", tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_service_account.test", tfjsonpath.New("organization_role"), knownvalue.StringExact("admin")),
				},
			},
		},
	})
}

func TestAccServiceAccountResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig + `resource "anthropic_service_account" "bad" { name = "Not A Slug" }`,
				ExpectError: regexp.MustCompile(`must contain only lowercase letters, digits and hyphens`),
			},
		},
	})
}

func TestAccServiceAccountResource_duplicateName(t *testing.T) {
	skipUnlessMock(t)
	name := slugName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_service_account" "a" { name = %q }
resource "anthropic_service_account" "b" {
  name       = anthropic_service_account.a.name
  depends_on = [anthropic_service_account.a]
}
`, name),
				ExpectError: regexp.MustCompile(`already exists`),
			},
		},
	})
}

func TestAccServiceAccountsDataSource(t *testing.T) {
	name := slugName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_service_account" "test" { name = %q }
data "anthropic_service_accounts" "all" { depends_on = [anthropic_service_account.test] }
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_service_accounts.all", tfjsonpath.New("service_accounts"), knownvalue.ListPartial(map[int]knownvalue.Check{})),
				},
			},
		},
	})
}

func TestAccWorkspaceServiceAccountResource(t *testing.T) {
	name := slugName()
	cfg := func(role string) string {
		return providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "ws" { name = %q }
resource "anthropic_service_account" "sa" { name = %q }
resource "anthropic_workspace_service_account" "test" {
  workspace_id       = anthropic_workspace.ws.id
  service_account_id = anthropic_service_account.sa.id
  workspace_role     = %q
}
`, name, name, role)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("workspace_developer"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace_service_account.test", tfjsonpath.New("workspace_role"), knownvalue.StringExact("workspace_developer")),
					statecheck.ExpectKnownValue("anthropic_workspace_service_account.test", tfjsonpath.New("implicit"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("anthropic_workspace_service_account.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^wrkspc_.+/svac_`))),
				},
			},
			{
				ResourceName:      "anthropic_workspace_service_account.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: cfg("workspace_admin"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace_service_account.test", tfjsonpath.New("workspace_role"), knownvalue.StringExact("workspace_admin")),
				},
			},
		},
	})
}

func TestAccWorkspaceServiceAccountResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "anthropic_workspace_service_account" "bad" {
  workspace_id       = "wrkspc_01ExampleWorkspaceId000000"
  service_account_id = "svac_01ExampleServiceAcct000000"
  workspace_role     = "workspace_billing"
}`,
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}
