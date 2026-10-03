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

func memoryConfig(store, path, content string) string {
	return providerConfig + fmt.Sprintf(`
resource "anthropic_memory_store" "test" { name = %q }

resource "anthropic_memory" "test" {
  memory_store_id = anthropic_memory_store.test.id
  path            = %q
  content         = %q
}

data "anthropic_memories" "all" {
  memory_store_id = anthropic_memory_store.test.id
  depends_on      = [anthropic_memory.test]
}
`, store, path, content)
}

func TestAccMemoryResource(t *testing.T) {
	store := acctest.RandomWithPrefix("tf-acc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      memoryConfig(store, "no-leading-slash.md", "x"),
				ExpectError: regexp.MustCompile(`must start with /`),
			},
			{
				Config: memoryConfig(store, "/style/go.md", "Use gofmt.\n"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_memory.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^mem_`))),
					statecheck.ExpectKnownValue("anthropic_memory.test", tfjsonpath.New("content_size_bytes"), knownvalue.Int64Exact(11)),
					statecheck.ExpectKnownValue("anthropic_memory.test", tfjsonpath.New("content_sha256"), knownvalue.StringRegexp(regexp.MustCompile(`^[0-9a-f]{64}$`))),
					statecheck.ExpectKnownValue("data.anthropic_memories.all", tfjsonpath.New("memories").AtSliceIndex(0).AtMapKey("path"), knownvalue.StringExact("/style/go.md")),
				},
			},
			// Content round-trips byte for byte, trailing newline included.
			{Config: memoryConfig(store, "/style/go.md", "Use gofmt.\n"), PlanOnly: true},
			{
				Config: memoryConfig(store, "/conventions/go.md", "Use gofmt and go vet.\n"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("anthropic_memory.test", plancheck.ResourceActionUpdate)},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_memory.test", tfjsonpath.New("path"), knownvalue.StringExact("/conventions/go.md")),
					statecheck.ExpectKnownValue("anthropic_memory.test", tfjsonpath.New("content_size_bytes"), knownvalue.Int64Exact(22)),
				},
			},
			{
				ResourceName:      "anthropic_memory.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources["anthropic_memory.test"]
					return rs.Primary.Attributes["memory_store_id"] + "/" + rs.Primary.ID, nil
				},
			},
			// Empty content is a valid memory.
			{
				Config: memoryConfig(store, "/conventions/go.md", ""),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_memory.test", tfjsonpath.New("content_size_bytes"), knownvalue.Int64Exact(0)),
				},
			},
		},
	})
}

// An agent rewriting a managed memory shows up as drift, and apply puts the
// configured content back.
func TestAccMemoryResource_agentWriteIsDrift(t *testing.T) {
	skipUnlessMock(t)
	store := acctest.RandomWithPrefix("tf-acc")
	var storeID, memoryID string
	capture := func(s *terraform.State) error {
		rs := s.RootModule().Resources["anthropic_memory.test"]
		storeID, memoryID = rs.Primary.Attributes["memory_store_id"], rs.Primary.ID
		return nil
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: memoryConfig(store, "/notes.md", "configured"), Check: capture},
			{
				PreConfig: func() {
					if !testMock.RewriteMemory(storeID, memoryID, "written by an agent") {
						t.Fatal("memory not found in mock")
					}
				},
				Config: memoryConfig(store, "/notes.md", "configured"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("anthropic_memory.test", plancheck.ResourceActionUpdate)},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_memory.test", tfjsonpath.New("content"), knownvalue.StringExact("configured")),
				},
			},
		},
	})
}
