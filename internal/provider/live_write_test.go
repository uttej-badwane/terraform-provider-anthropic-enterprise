package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// Write-tier live tests. These create and delete real objects (invites,
// members, service accounts, federation objects) and are meant for a
// dedicated development organization only. They run when BOTH
// ANTHROPIC_ACC_LIVE=1 and ANTHROPIC_ACC_LIVE_WRITE=1 are set.

func skipUnlessLiveWrite(t *testing.T) {
	t.Helper()
	skipUnlessLive(t)
	if os.Getenv("ANTHROPIC_ACC_LIVE_WRITE") == "" {
		t.Skip("write-tier live test: set ANTHROPIC_ACC_LIVE_WRITE=1 against a development organization")
	}
}

func liveSlug() string { return strings.ToLower(acctest.RandomWithPrefix("tf-acc")) }

func TestAccLiveWriteInviteLifecycle(t *testing.T) {
	skipUnlessLiveWrite(t)
	email := os.Getenv("ANTHROPIC_ACC_INVITE_EMAIL")
	if email == "" {
		t.Skip("set ANTHROPIC_ACC_INVITE_EMAIL to an address you control")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_invite" "test" {
  email = %q
  role  = "user"
}
data "anthropic_invites" "pending" { statuses = ["pending"] }
`, email),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_invite.test", tfjsonpath.New("status"), knownvalue.StringExact("pending")),
				},
			},
			{ResourceName: "anthropic_invite.test", ImportState: true, ImportStateVerify: true},
		},
	})
}

func TestAccLiveWriteWorkspaceMember(t *testing.T) {
	skipUnlessLiveWrite(t)
	userID := os.Getenv("ANTHROPIC_ACC_MEMBER_USER_ID")
	if userID == "" {
		t.Skip("set ANTHROPIC_ACC_MEMBER_USER_ID to a non-admin member id (user_...)")
	}
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" { name = %q }
resource "anthropic_workspace_member" "test" {
  workspace_id   = anthropic_workspace.test.id
  user_id        = %q
  workspace_role = "workspace_user"
}
`, name, userID),
			},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" { name = %q }
resource "anthropic_workspace_member" "test" {
  workspace_id   = anthropic_workspace.test.id
  user_id        = %q
  workspace_role = "workspace_developer"
}
`, name, userID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_workspace_member.test", tfjsonpath.New("workspace_role"), knownvalue.StringExact("workspace_developer")),
				},
			},
			{ResourceName: "anthropic_workspace_member.test", ImportState: true, ImportStateVerify: true},
		},
	})
}

func TestAccLiveWriteFederationLifecycle(t *testing.T) {
	skipUnlessLiveWrite(t)
	if os.Getenv("ANTHROPIC_AUTH_TOKEN") == "" {
		t.Skip("needs ANTHROPIC_AUTH_TOKEN (org:admin OAuth token) for service accounts and federation")
	}
	sa, issuer, rule, ws := liveSlug(), liveSlug(), liveSlug(), acctest.RandomWithPrefix("tf-acc")
	config := providerConfig + fmt.Sprintf(`
resource "anthropic_workspace" "test" { name = %q }

resource "anthropic_service_account" "test" {
  name        = %q
  description = "acceptance test"
}

resource "anthropic_workspace_service_account" "test" {
  workspace_id       = anthropic_workspace.test.id
  service_account_id = anthropic_service_account.test.id
  workspace_role     = "workspace_developer"
}

resource "anthropic_federation_issuer" "test" {
  name       = %q
  issuer_url = "https://token.actions.githubusercontent.com"
}

resource "anthropic_federation_rule" "test" {
  name               = %q
  issuer_id          = anthropic_federation_issuer.test.id
  service_account_id = anthropic_service_account.test.id
  oauth_scope        = "workspace:inference"
  workspace_id       = anthropic_workspace.test.id
  match = {
    subject_prefix = "repo:example-org/example-repo:ref:refs/heads/main"
  }
}
`, ws, sa, issuer, rule)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: config},
			{ResourceName: "anthropic_service_account.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"archive_on_destroy"}},
			{ResourceName: "anthropic_federation_issuer.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"archive_on_destroy"}},
			{ResourceName: "anthropic_federation_rule.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"archive_on_destroy"}},
		},
	})
}
