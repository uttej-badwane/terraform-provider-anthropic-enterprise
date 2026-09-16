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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccVaultResource(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_vault" "test" {
  display_name = %q
  metadata     = { team = "platform", env = "test" }
}
data "anthropic_vault"  "one" { id = anthropic_vault.test.id }
data "anthropic_vaults" "all" {}
data "anthropic_vault_credentials" "none" { vault_id = anthropic_vault.test.id }
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_vault.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^vlt_`))),
					statecheck.ExpectKnownValue("anthropic_vault.test", tfjsonpath.New("delete_on_destroy"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("data.anthropic_vault.one", tfjsonpath.New("display_name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("data.anthropic_vault_credentials.none", tfjsonpath.New("credentials"), knownvalue.ListSizeExact(0)),
				},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_vault" "test" {
  display_name = "%s-2"
  metadata     = { team = "platform" }
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_vault.test", tfjsonpath.New("display_name"), knownvalue.StringExact(name+"-2")),
					statecheck.ExpectKnownValue("anthropic_vault.test", tfjsonpath.New("metadata"), knownvalue.MapExact(map[string]knownvalue.Check{"team": knownvalue.StringExact("platform")})),
				},
			},
			{ResourceName: "anthropic_vault.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"delete_on_destroy"}},
		},
	})
}

func credentialImportID(res string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[res]
		if !ok {
			return "", fmt.Errorf("%s not in state", res)
		}
		return rs.Primary.Attributes["vault_id"] + "/" + rs.Primary.ID, nil
	}
}

func TestAccVaultCredentialResource(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	base := providerConfig + fmt.Sprintf(`
resource "anthropic_vault" "test" { display_name = %q }
`, name)
	envVar := func(version, hosts string) string {
		return base + fmt.Sprintf(`
resource "anthropic_vault_credential" "gh" {
  vault_id       = anthropic_vault.test.id
  display_name   = "github token"
  secret_version = %q
  environment_variable = {
    secret_name     = "GH_TOKEN"
    secret_value    = "example-secret-value"
    networking_type = "limited"
    allowed_hosts   = [%s]
  }
}
data "anthropic_vault_credentials" "all" {
  vault_id   = anthropic_vault.test.id
  depends_on = [anthropic_vault_credential.gh]
}
`, version, hosts)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: envVar("v1", `"api.github.com"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_vault_credential.gh", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^vcrd_`))),
					statecheck.ExpectKnownValue("anthropic_vault_credential.gh", tfjsonpath.New("environment_variable").AtMapKey("secret_value"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_vault_credential.gh", tfjsonpath.New("environment_variable").AtMapKey("inject_header"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("anthropic_vault_credential.gh", tfjsonpath.New("environment_variable").AtMapKey("inject_body"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("data.anthropic_vault_credentials.all", tfjsonpath.New("credentials").AtSliceIndex(0).AtMapKey("secret_name"), knownvalue.StringExact("GH_TOKEN")),
					statecheck.ExpectKnownValue("data.anthropic_vault_credentials.all", tfjsonpath.New("credentials").AtSliceIndex(0).AtMapKey("auth_type"), knownvalue.StringExact("environment_variable")),
				},
			},
			{
				// Re-applying the same config must plan no changes (secrets are write-only).
				Config:             envVar("v1", `"api.github.com"`),
				ConfigPlanChecks:   resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ExpectNonEmptyPlan: false,
			},
			{
				// Rotation: bumping secret_version updates in place.
				Config:           envVar("v2", `"api.github.com", "uploads.github.com"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("anthropic_vault_credential.gh", plancheck.ResourceActionUpdate)}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_vault_credential.gh", tfjsonpath.New("environment_variable").AtMapKey("allowed_hosts"), knownvalue.ListSizeExact(2)),
				},
			},
			{
				ResourceName:            "anthropic_vault_credential.gh",
				ImportState:             true,
				ImportStateIdFunc:       credentialImportID("anthropic_vault_credential.gh"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_on_destroy", "secret_version"},
			},
			{
				// Changing the immutable key replaces the credential.
				Config: base + `
resource "anthropic_vault_credential" "gh" {
  vault_id = anthropic_vault.test.id
  environment_variable = {
    secret_name     = "OTHER_TOKEN"
    secret_value    = "example-secret-value"
    networking_type = "unrestricted"
  }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("anthropic_vault_credential.gh", plancheck.ResourceActionReplace)}},
			},
		},
	})
}

func TestAccVaultCredentialResource_staticBearerAndOAuth(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_vault" "test" { display_name = %q }

resource "anthropic_vault_credential" "bearer" {
  vault_id = anthropic_vault.test.id
  static_bearer = {
    mcp_server_url = "https://mcp.example.com/sse"
    token          = "example-bearer-token"
  }
}

resource "anthropic_vault_credential" "oauth" {
  vault_id = anthropic_vault.test.id
  mcp_oauth = {
    mcp_server_url = "https://oauth-mcp.example.com/sse"
    access_token   = "example-access-token"
    refresh = {
      token_endpoint           = "https://idp.example.com/oauth/token"
      client_id                = "example-client"
      refresh_token            = "example-refresh-token"
      token_endpoint_auth_type = "client_secret_post"
      client_secret            = "example-client-secret"
    }
  }
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_vault_credential.bearer", tfjsonpath.New("static_bearer").AtMapKey("token"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_vault_credential.oauth", tfjsonpath.New("mcp_oauth").AtMapKey("refresh").AtMapKey("token_endpoint_auth_type"), knownvalue.StringExact("client_secret_post")),
					statecheck.ExpectKnownValue("anthropic_vault_credential.oauth", tfjsonpath.New("mcp_oauth").AtMapKey("refresh").AtMapKey("client_secret"), knownvalue.Null()),
				},
			},
		},
	})
}

func TestAccVaultCredentialResource_duplicateKey(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`
resource "anthropic_vault" "test" { display_name = %q }
resource "anthropic_vault_credential" "a" {
  vault_id = anthropic_vault.test.id
  environment_variable = { secret_name = "DUP", secret_value = "x", networking_type = "unrestricted" }
}
resource "anthropic_vault_credential" "b" {
  vault_id   = anthropic_vault.test.id
  depends_on = [anthropic_vault_credential.a]
  environment_variable = { secret_name = "DUP", secret_value = "y", networking_type = "unrestricted" }
}
`, name),
			ExpectError: regexp.MustCompile(`already exists`),
		}},
	})
}

func TestAccVaultCredentialResource_exactlyOne(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_vault_credential" "bad" {
  vault_id = "vlt_01ExampleVaultId000000000"
}
`,
			ExpectError: regexp.MustCompile(`(?s)one of.*static_bearer`),
		}},
	})
}
