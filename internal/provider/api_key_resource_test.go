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

func TestAccAPIKeyResource_import(t *testing.T) {
	skipUnlessMock(t)
	keyID := testMock.APIKeys()[0].ID
	cfg := func(name, status string) string {
		return providerConfig + fmt.Sprintf(`
resource "anthropic_api_key" "test" {
  name   = %q
  status = %q
}
`, name, status)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             cfg("Developer Key", "active"),
				ResourceName:       "anthropic_api_key.test",
				ImportState:        true,
				ImportStateId:      keyID,
				ImportStatePersist: true,
			},
			{
				Config: cfg("Developer Key", "active"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_api_key.test", tfjsonpath.New("id"), knownvalue.StringExact(keyID)),
					statecheck.ExpectKnownValue("anthropic_api_key.test", tfjsonpath.New("partial_key_hint"), knownvalue.StringExact("sk-ant-api03-R2D...igAA")),
					statecheck.ExpectKnownValue("anthropic_api_key.test", tfjsonpath.New("scope_type"), knownvalue.StringExact("workspace")),
					statecheck.ExpectKnownValue("anthropic_api_key.test", tfjsonpath.New("principal_type"), knownvalue.StringExact("user_actor")),
					statecheck.ExpectKnownValue("anthropic_api_key.test", tfjsonpath.New("created_by_type"), knownvalue.StringExact("user")),
					statecheck.ExpectKnownValue("anthropic_api_key.test", tfjsonpath.New("deactivate_on_destroy"), knownvalue.Bool(false)),
				},
			},
			{
				Config: cfg("Renamed Key", "inactive"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_api_key.test", tfjsonpath.New("name"), knownvalue.StringExact("Renamed Key")),
					statecheck.ExpectKnownValue("anthropic_api_key.test", tfjsonpath.New("status"), knownvalue.StringExact("inactive")),
				},
			},
			{
				Config: cfg("Developer Key", "active"),
			},
		},
	})
}

func TestAccAPIKeyResource_createErrors(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerConfig + `resource "anthropic_api_key" "new" { name = "x" }`,
				ExpectError: regexp.MustCompile(`API keys cannot be created`),
			},
		},
	})
}

func TestAccAPIKeysDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
data "anthropic_api_keys" "all" {}
data "anthropic_api_keys" "active" { status = "active" }
data "anthropic_api_keys" "none" { status = "expired" }
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_api_keys.all", tfjsonpath.New("api_keys").AtSliceIndex(0).AtMapKey("name"), knownvalue.StringExact("Developer Key")),
					statecheck.ExpectKnownValue("data.anthropic_api_keys.active", tfjsonpath.New("api_keys"), knownvalue.ListSizeExact(1)),
					statecheck.ExpectKnownValue("data.anthropic_api_keys.none", tfjsonpath.New("api_keys"), knownvalue.ListSizeExact(0)),
				},
			},
		},
	})
}
