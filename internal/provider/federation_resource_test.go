package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccFederationIssuerResource(t *testing.T) {
	name := slugName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_federation_issuer" "test" {
  name       = %q
  issuer_url = "https://token.actions.githubusercontent.com"
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^fdis_`))),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("check_jti"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("max_jwt_lifetime_seconds"), knownvalue.Int64Exact(3600)),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("jwks").AtMapKey("type"), knownvalue.StringExact("discovery")),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("archived_at"), knownvalue.Null()),
				},
			},
			{
				ResourceName:            "anthropic_federation_issuer.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"archive_on_destroy"},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_federation_issuer" "test" {
  name                     = "%s-2"
  issuer_url               = "https://token.actions.githubusercontent.com"
  check_jti                = false
  max_jwt_lifetime_seconds = 600
  jwks = {
    type = "explicit_url"
    url  = "https://token.actions.githubusercontent.com/.well-known/jwks"
  }
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("name"), knownvalue.StringExact(name+"-2")),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("check_jti"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("max_jwt_lifetime_seconds"), knownvalue.Int64Exact(600)),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("jwks").AtMapKey("type"), knownvalue.StringExact("explicit_url")),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("jwks").AtMapKey("url"), knownvalue.StringExact("https://token.actions.githubusercontent.com/.well-known/jwks")),
				},
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_federation_issuer" "test" {
  name       = "%s-2"
  issuer_url = "https://token.actions.githubusercontent.com"
  jwks = {
    type = "inline"
    keys = [jsonencode({ kty = "RSA", kid = "k1", n = "AQAB", e = "AQAB" })]
  }
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("jwks").AtMapKey("type"), knownvalue.StringExact("inline")),
					statecheck.ExpectKnownValue("anthropic_federation_issuer.test", tfjsonpath.New("jwks").AtMapKey("keys"), knownvalue.ListSizeExact(1)),
				},
			},
		},
	})
}

func TestAccFederationIssuerResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "anthropic_federation_issuer" "bad" {
  name       = "bad"
  issuer_url = "http://insecure.example.com"
}`,
				ExpectError: regexp.MustCompile(`must start with https://`),
			},
			{
				Config: providerConfig + `
resource "anthropic_federation_issuer" "bad" {
  name       = "bad"
  issuer_url = "https://issuer.example.com"
  jwks       = { type = "explicit_url" }
}`,
				ExpectError: regexp.MustCompile(`jwks.url is required`),
			},
		},
	})
}

func federationFixture(name string) string {
	return providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "ws" { name = %q }
resource "anthropic_service_account" "sa" { name = %q }
resource "anthropic_federation_issuer" "iss" {
  name       = %q
  issuer_url = "https://token.actions.githubusercontent.com"
}
`, name, name, name)
}

func TestAccFederationRuleResource(t *testing.T) {
	name := slugName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: federationFixture(name) + fmt.Sprintf(`
resource "anthropic_federation_rule" "test" {
  name               = %q
  issuer_id          = anthropic_federation_issuer.iss.id
  service_account_id = anthropic_service_account.sa.id
  oauth_scope        = "workspace:inference"
  workspace_id       = anthropic_workspace.ws.id
  description        = "deploys from main"
  match = {
    subject_prefix = "repo:example-org/example-repo:ref:refs/heads/main"
    claims         = { repository_owner = "example-org" }
  }
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^fdrl_`))),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("token_lifetime_seconds"), knownvalue.Int64Exact(3600)),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("applies_to_all_workspaces"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("workspace_ids"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("issuer_name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("service_account_name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("match").AtMapKey("claims").AtMapKey("repository_owner"), knownvalue.StringExact("example-org")),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("match").AtMapKey("audience"), knownvalue.Null()),
				},
			},
			{
				ResourceName:            "anthropic_federation_rule.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"archive_on_destroy"},
			},
			{
				Config: federationFixture(name) + fmt.Sprintf(`
resource "anthropic_federation_rule" "test" {
  name                   = "%s-2"
  issuer_id              = anthropic_federation_issuer.iss.id
  service_account_id     = anthropic_service_account.sa.id
  oauth_scope            = "workspace:developer"
  workspace_id           = anthropic_workspace.ws.id
  token_lifetime_seconds = 900
  match = {
    condition = "claims.ref == 'refs/heads/main'"
  }
}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("name"), knownvalue.StringExact(name+"-2")),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("oauth_scope"), knownvalue.StringExact("workspace:developer")),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("token_lifetime_seconds"), knownvalue.Int64Exact(900)),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("match").AtMapKey("subject_prefix"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_federation_rule.test", tfjsonpath.New("match").AtMapKey("condition"), knownvalue.StringExact("claims.ref == 'refs/heads/main'")),
				},
			},
		},
	})
}

func TestAccFederationRuleResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "anthropic_federation_rule" "bad" {
  name               = "bad"
  issuer_id          = "fdis_01ExampleIssuerId00000000"
  service_account_id = "svac_01ExampleServiceAcct000000"
  oauth_scope        = "workspace:inference"
  workspace_id       = "wrkspc_01ExampleWorkspaceId000000"
  match              = { audience = "only-audience" }
}`,
				ExpectError: regexp.MustCompile(`At least one of match.subject_prefix, match.claims or match.condition`),
			},
			{
				Config: providerConfig + `
resource "anthropic_federation_rule" "bad" {
  name               = "bad"
  issuer_id          = "fdis_01ExampleIssuerId00000000"
  service_account_id = "svac_01ExampleServiceAcct000000"
  oauth_scope        = "workspace:inference"
  match              = { subject_prefix = "repo:example-org/*" }
}`,
				ExpectError: regexp.MustCompile(`workspace_id is required unless applies_to_all_workspaces`),
			},
			{
				Config: providerConfig + `
resource "anthropic_federation_rule" "bad" {
  name                      = "bad"
  issuer_id                 = "fdis_01ExampleIssuerId00000000"
  service_account_id        = "svac_01ExampleServiceAcct000000"
  oauth_scope               = "workspace:inference"
  applies_to_all_workspaces = true
  workspace_id              = "wrkspc_01ExampleWorkspaceId000000"
  match                     = { subject_prefix = "repo:example-org/*" }
}`,
				ExpectError: regexp.MustCompile(`workspace_id cannot be set when applies_to_all_workspaces`),
			},
		},
	})
}

func TestAccFederationRuleWorkspaceResource(t *testing.T) {
	name := slugName()
	cfg := federationFixture(name) + fmt.Sprintf(`
resource "anthropic_workspace" "ws2" { name = "%s-2" }
resource "anthropic_federation_rule" "rule" {
  name               = %q
  issuer_id          = anthropic_federation_issuer.iss.id
  service_account_id = anthropic_service_account.sa.id
  oauth_scope        = "workspace:inference"
  workspace_id       = anthropic_workspace.ws.id
  match              = { subject_prefix = "repo:example-org/example-repo:*" }
}
resource "anthropic_federation_rule_workspace" "test" {
  federation_rule_id = anthropic_federation_rule.rule.id
  workspace_id       = anthropic_workspace.ws2.id
}
data "anthropic_federation_rules" "all" { depends_on = [anthropic_federation_rule_workspace.test] }
data "anthropic_federation_issuers" "all" { depends_on = [anthropic_federation_issuer.iss] }
`, name, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_federation_rule_workspace.test", tfjsonpath.New("workspace_name"), knownvalue.StringExact(name+"-2")),
					statecheck.ExpectKnownValue("anthropic_federation_rule_workspace.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^fdrl_.+/wrkspc_`))),
					statecheck.ExpectKnownValue("data.anthropic_federation_rules.all", tfjsonpath.New("rules"), knownvalue.ListPartial(map[int]knownvalue.Check{})),
					statecheck.ExpectKnownValue("data.anthropic_federation_issuers.all", tfjsonpath.New("issuers"), knownvalue.ListPartial(map[int]knownvalue.Check{})),
				},
			},
			{
				ResourceName:      "anthropic_federation_rule_workspace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Refresh: the rule's primary workspace_id must survive the second binding.
				Config:   cfg,
				PlanOnly: true,
			},
		},
	})
}
