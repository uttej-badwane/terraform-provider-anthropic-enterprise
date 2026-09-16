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

func TestAccInviteDataSources(t *testing.T) {
	skipUnlessMock(t)
	email := acctest.RandomWithPrefix("tf-acc") + "@example.com"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`
resource "anthropic_invite" "test" {
  email = %q
  role  = "developer"
}

data "anthropic_invite" "one" { id = anthropic_invite.test.id }

data "anthropic_invites" "pending" {
  email    = anthropic_invite.test.email
  statuses = ["pending"]
}
`, email),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_invite.one", tfjsonpath.New("email"), knownvalue.StringExact(email)),
				statecheck.ExpectKnownValue("data.anthropic_invite.one", tfjsonpath.New("status"), knownvalue.StringExact("pending")),
				statecheck.ExpectKnownValue("data.anthropic_invite.one", tfjsonpath.New("role"), knownvalue.StringExact("developer")),
				statecheck.ExpectKnownValue("data.anthropic_invites.pending", tfjsonpath.New("invites"), knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.anthropic_invites.pending", tfjsonpath.New("invites").AtSliceIndex(0).AtMapKey("email"), knownvalue.StringExact(email)),
			},
		}},
	})
}

func TestAccWorkspaceMemberDataSources(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	user := testMock.UserByEmail("dev@example.com")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "ws" { name = %q }

resource "anthropic_workspace_member" "m" {
  workspace_id   = anthropic_workspace.ws.id
  user_id        = %q
  workspace_role = "workspace_developer"
}

data "anthropic_workspace_members" "all" {
  workspace_id = anthropic_workspace.ws.id
  depends_on   = [anthropic_workspace_member.m]
}

data "anthropic_workspace_member" "one" {
  workspace_id = anthropic_workspace.ws.id
  user_id      = anthropic_workspace_member.m.user_id
}
`, name, user.ID),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_workspace_members.all", tfjsonpath.New("members"), knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.anthropic_workspace_members.all", tfjsonpath.New("members").AtSliceIndex(0).AtMapKey("user_id"), knownvalue.StringExact(user.ID)),
				statecheck.ExpectKnownValue("data.anthropic_workspace_member.one", tfjsonpath.New("workspace_role"), knownvalue.StringExact("workspace_developer")),
			},
		}},
	})
}

func TestAccAPIKeyDataSource(t *testing.T) {
	skipUnlessMock(t)
	key := testMock.APIKeys()[0]
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`data "anthropic_api_key" "one" { id = %q }`, key.ID),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_api_key.one", tfjsonpath.New("name"), knownvalue.StringExact(key.Name)),
				statecheck.ExpectKnownValue("data.anthropic_api_key.one", tfjsonpath.New("status"), knownvalue.StringExact("active")),
				statecheck.ExpectKnownValue("data.anthropic_api_key.one", tfjsonpath.New("scope_type"), knownvalue.StringExact("workspace")),
				statecheck.ExpectKnownValue("data.anthropic_api_key.one", tfjsonpath.New("principal_type"), knownvalue.StringExact("user_actor")),
				statecheck.ExpectKnownValue("data.anthropic_api_key.one", tfjsonpath.New("partial_key_hint"), knownvalue.NotNull()),
			},
		}},
	})
}

func TestAccExternalKeyDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
resource "anthropic_external_key" "k" {
  display_name = "tf-acc-read-test"
  provider_config = {
    type    = "aws"
    kms_arn = "arn:aws:kms:us-east-1:123456789012:key/11111111-2222-3333-4444-555555555555"
  }
}

data "anthropic_external_key" "one" { id = anthropic_external_key.k.id }
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_external_key.one", tfjsonpath.New("display_name"), knownvalue.StringExact("tf-acc-read-test")),
				statecheck.ExpectKnownValue("data.anthropic_external_key.one", tfjsonpath.New("provider_type"), knownvalue.StringExact("aws")),
				statecheck.ExpectKnownValue("data.anthropic_external_key.one", tfjsonpath.New("region"), knownvalue.StringExact("us-east-1")),
				statecheck.ExpectKnownValue("data.anthropic_external_key.one", tfjsonpath.New("attachment_type"), knownvalue.StringExact("unattached")),
			},
		}},
	})
}

func TestAccServiceAccountReadDataSources(t *testing.T) {
	name := slugName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "ws" { name = %q }

resource "anthropic_service_account" "sa" {
  name        = %q
  description = "read test"
}

resource "anthropic_workspace_service_account" "m" {
  workspace_id       = anthropic_workspace.ws.id
  service_account_id = anthropic_service_account.sa.id
  workspace_role     = "workspace_developer"
}

data "anthropic_service_account" "by_id"   { id   = anthropic_service_account.sa.id }
data "anthropic_service_account" "by_name" { name = anthropic_service_account.sa.name }

data "anthropic_workspace_service_accounts" "all" {
  workspace_id = anthropic_workspace.ws.id
  depends_on   = [anthropic_workspace_service_account.m]
}
`, name, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_service_account.by_id", tfjsonpath.New("name"), knownvalue.StringExact(name)),
					statecheck.ExpectKnownValue("data.anthropic_service_account.by_id", tfjsonpath.New("description"), knownvalue.StringExact("read test")),
					statecheck.ExpectKnownValue("data.anthropic_service_account.by_name", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^svac_`))),
					statecheck.ExpectKnownValue("data.anthropic_service_account.by_name", tfjsonpath.New("organization_role"), knownvalue.StringExact("developer")),
					statecheck.ExpectKnownValue("data.anthropic_workspace_service_accounts.all", tfjsonpath.New("service_accounts"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("data.anthropic_workspace_service_accounts.all", tfjsonpath.New("service_accounts").AtSliceIndex(0).AtMapKey("workspace_role"), knownvalue.StringExact("workspace_developer")),
					statecheck.ExpectKnownValue("data.anthropic_workspace_service_accounts.all", tfjsonpath.New("service_accounts").AtSliceIndex(0).AtMapKey("implicit"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func TestAccServiceAccountDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig + `data "anthropic_service_account" "bad" {}`,
				ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured`),
			},
			{
				Config:      providerConfig + `data "anthropic_service_account" "missing" { name = "does-not-exist" }`,
				ExpectError: regexp.MustCompile(`Service account not found`),
			},
		},
	})
}

func TestAccFederationReadDataSources(t *testing.T) {
	name := slugName()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: federationFixture(name) + fmt.Sprintf(`
resource "anthropic_workspace" "ws2" { name = "%s-2" }

resource "anthropic_federation_rule" "rule" {
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

resource "anthropic_federation_rule_workspace" "extra" {
  federation_rule_id = anthropic_federation_rule.rule.id
  workspace_id       = anthropic_workspace.ws2.id
}

data "anthropic_federation_issuer" "by_id"   { id   = anthropic_federation_issuer.iss.id }
data "anthropic_federation_issuer" "by_name" { name = anthropic_federation_issuer.iss.name }
data "anthropic_federation_rule"   "by_id"   { id   = anthropic_federation_rule.rule.id }
data "anthropic_federation_rule"   "by_name" { name = anthropic_federation_rule.rule.name }

data "anthropic_federation_rule_workspaces" "all" {
  federation_rule_id = anthropic_federation_rule.rule.id
  depends_on         = [anthropic_federation_rule_workspace.extra]
}
`, name, name),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_federation_issuer.by_id", tfjsonpath.New("issuer_url"), knownvalue.StringExact("https://token.actions.githubusercontent.com")),
				statecheck.ExpectKnownValue("data.anthropic_federation_issuer.by_id", tfjsonpath.New("jwks_type"), knownvalue.StringExact("discovery")),
				statecheck.ExpectKnownValue("data.anthropic_federation_issuer.by_id", tfjsonpath.New("check_jti"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue("data.anthropic_federation_issuer.by_name", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^fdis_`))),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule.by_id", tfjsonpath.New("oauth_scope"), knownvalue.StringExact("workspace:inference")),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule.by_id", tfjsonpath.New("description"), knownvalue.StringExact("deploys from main")),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule.by_id", tfjsonpath.New("subject_prefix"), knownvalue.StringExact("repo:example-org/example-repo:ref:refs/heads/main")),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule.by_id", tfjsonpath.New("claims").AtMapKey("repository_owner"), knownvalue.StringExact("example-org")),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule.by_id", tfjsonpath.New("condition"), knownvalue.Null()),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule.by_id", tfjsonpath.New("issuer_name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule.by_id", tfjsonpath.New("service_account_name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule.by_name", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^fdrl_`))),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule_workspaces.all", tfjsonpath.New("workspaces"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_federation_rule_workspaces.all", tfjsonpath.New("workspaces").AtSliceIndex(0).AtMapKey("workspace_name"), knownvalue.StringExact(name)),
			},
		}},
	})
}

func TestAccRBACGroupReadDataSources(t *testing.T) {
	skipUnlessMock(t)
	name := acctest.RandomWithPrefix("tf-acc")
	user := testMock.UserByEmail("user@example.com")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`
resource "anthropic_rbac_group" "g" { name = %q }

resource "anthropic_rbac_group_member" "m" {
  group_id = anthropic_rbac_group.g.id
  user_id  = %q
}

data "anthropic_rbac_group" "by_id"   { id   = anthropic_rbac_group.g.id }
data "anthropic_rbac_group" "by_name" { name = anthropic_rbac_group.g.name }

data "anthropic_rbac_group_members" "all" {
  group_id   = anthropic_rbac_group.g.id
  depends_on = [anthropic_rbac_group_member.m]
}
`, name, user.ID),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_rbac_group.by_id", tfjsonpath.New("name"), knownvalue.StringExact(name)),
				statecheck.ExpectKnownValue("data.anthropic_rbac_group.by_id", tfjsonpath.New("source_type"), knownvalue.StringExact("direct")),
				statecheck.ExpectKnownValue("data.anthropic_rbac_group.by_name", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^rbac_group_`))),
				statecheck.ExpectKnownValue("data.anthropic_rbac_group_members.all", tfjsonpath.New("members"), knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.anthropic_rbac_group_members.all", tfjsonpath.New("members").AtSliceIndex(0).AtMapKey("email"), knownvalue.StringExact("user@example.com")),
			},
		}},
	})
}

func TestAccSpendLimitDataSource(t *testing.T) {
	skipUnlessMock(t)
	user := testMock.UserByEmail("dev@example.com")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + fmt.Sprintf(`
resource "anthropic_spend_limit" "sl" {
  user_id = %q
  amount  = "50000"
}

data "anthropic_spend_limit" "one" { id = anthropic_spend_limit.sl.id }
`, user.ID),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_spend_limit.one", tfjsonpath.New("user_id"), knownvalue.StringExact(user.ID)),
				statecheck.ExpectKnownValue("data.anthropic_spend_limit.one", tfjsonpath.New("amount"), knownvalue.StringExact("50000")),
				statecheck.ExpectKnownValue("data.anthropic_spend_limit.one", tfjsonpath.New("period"), knownvalue.StringExact("monthly")),
				statecheck.ExpectKnownValue("data.anthropic_spend_limit.one", tfjsonpath.New("currency"), knownvalue.StringExact("USD")),
			},
		}},
	})
}
