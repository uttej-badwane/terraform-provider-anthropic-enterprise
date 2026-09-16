package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

const linkedOrgUUID = "66666666-7777-8888-9999-000000000000"

func TestAccComplianceDirectoryDataSources(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_compliance_organizations" "all" {}

data "anthropic_compliance_organization_users" "first" {
  organization_uuid = data.anthropic_compliance_organizations.all.organizations[0].uuid
}

data "anthropic_compliance_roles" "first" {
  organization_uuid = data.anthropic_compliance_organizations.all.organizations[0].uuid
}

data "anthropic_compliance_role" "member" {
  organization_uuid = data.anthropic_compliance_organizations.all.organizations[0].uuid
  id                = data.anthropic_compliance_roles.first.roles[0].id
}

data "anthropic_compliance_groups" "all" {}

data "anthropic_compliance_group_members" "scim" {
  group_id = data.anthropic_compliance_groups.all.groups[0].id
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_compliance_organizations.all", tfjsonpath.New("organizations"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_compliance_organizations.all", tfjsonpath.New("organizations").AtSliceIndex(1).AtMapKey("uuid"), knownvalue.StringExact(linkedOrgUUID)),
				statecheck.ExpectKnownValue("data.anthropic_compliance_organization_users.first", tfjsonpath.New("users").AtSliceIndex(0).AtMapKey("organization_role"), knownvalue.StringExact("primary_owner")),
				statecheck.ExpectKnownValue("data.anthropic_compliance_roles.first", tfjsonpath.New("roles"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_compliance_role.member", tfjsonpath.New("name"), knownvalue.StringExact("Member")),
				statecheck.ExpectKnownValue("data.anthropic_compliance_role.member", tfjsonpath.New("description"), knownvalue.StringExact("Custom role")),
				statecheck.ExpectKnownValue("data.anthropic_compliance_role.member", tfjsonpath.New("permissions"), knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.anthropic_compliance_groups.all", tfjsonpath.New("groups").AtSliceIndex(0).AtMapKey("source_type"), knownvalue.StringExact("scim")),
				statecheck.ExpectKnownValue("data.anthropic_compliance_groups.all", tfjsonpath.New("groups").AtSliceIndex(0).AtMapKey("roles"), knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.anthropic_compliance_group_members.scim", tfjsonpath.New("members"), knownvalue.ListSizeExact(0)),
			},
		}},
	})
}

func TestAccComplianceEffectiveSettingsDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_compliance_effective_settings" "linked" {
  organization_uuid = "` + linkedOrgUUID + `"
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_compliance_effective_settings.linked", tfjsonpath.New("organization_id"), knownvalue.StringExact(linkedOrgUUID)),
				statecheck.ExpectKnownValue("data.anthropic_compliance_effective_settings.linked", tfjsonpath.New("settings_map").AtMapKey("content_redaction_enabled"), knownvalue.StringExact("true")),
				statecheck.ExpectKnownValue("data.anthropic_compliance_effective_settings.linked", tfjsonpath.New("settings_map").AtMapKey("ip_allowlist_ip_ranges"), knownvalue.StringExact(`["10.0.0.0/8","203.0.113.0/24"]`)),
				statecheck.ExpectKnownValue("data.anthropic_compliance_effective_settings.linked", tfjsonpath.New("settings_map").AtMapKey("account_session_duration_seconds"), knownvalue.StringExact("28800")),
				statecheck.ExpectKnownValue("data.anthropic_compliance_effective_settings.linked", tfjsonpath.New("settings").AtSliceIndex(0).AtMapKey("type"), knownvalue.StringExact("data_retention")),
				statecheck.ExpectKnownValue("data.anthropic_compliance_effective_settings.linked", tfjsonpath.New("api_keys"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.ObjectPartial(map[string]knownvalue.Check{"name": knownvalue.StringExact("Compliance Export Key"), "is_active": knownvalue.Bool(true), "expires_at": knownvalue.Null()}),
				})),
			},
		}},
	})
}

func TestAccComplianceEffectiveSettingsDataSource_parentNotFound(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_compliance_effective_settings" "parent" {
  organization_uuid = "11111111-2222-3333-4444-555555555555"
}
`,
			ExpectError: regexp.MustCompile(`(?s)Error reading effective organization settings.*404`),
		}},
	})
}
