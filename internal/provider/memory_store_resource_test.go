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

func TestAccMemoryStoreResource(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`resource "anthropic_memory_store" "test" { name = %q }`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_memory_store.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^memstore_`))),
					statecheck.ExpectKnownValue("anthropic_memory_store.test", tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_memory_store.test", tfjsonpath.New("metadata"), knownvalue.Null()),
				},
			},
			{Config: providerConfig + fmt.Sprintf(`resource "anthropic_memory_store" "test" { name = %q }`, name), PlanOnly: true},
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_memory_store" "test" {
  name        = %q
  description = "Shared notes"
  metadata    = { team = "platform" }
}
data "anthropic_memory_store" "read" { id = anthropic_memory_store.test.id }
data "anthropic_memory_stores" "all" {}
`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_memory_store.test", tfjsonpath.New("description"), knownvalue.StringExact("Shared notes")),
					statecheck.ExpectKnownValue("data.anthropic_memory_store.read", tfjsonpath.New("metadata"), knownvalue.MapExact(map[string]knownvalue.Check{"team": knownvalue.StringExact("platform")})),
				},
			},
			{
				Config: providerConfig + fmt.Sprintf(`resource "anthropic_memory_store" "test" { name = %q }`, name),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_memory_store.test", tfjsonpath.New("description"), knownvalue.Null()),
					statecheck.ExpectKnownValue("anthropic_memory_store.test", tfjsonpath.New("metadata"), knownvalue.Null()),
				},
			},
			{ResourceName: "anthropic_memory_store.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"delete_on_destroy"}},
		},
	})
}
