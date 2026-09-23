package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccModelsDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_models" "all" {}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_models.all", tfjsonpath.New("models"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_models.all", tfjsonpath.New("models").AtSliceIndex(0).AtMapKey("id"), knownvalue.StringExact("claude-opus-5")),
				statecheck.ExpectKnownValue("data.anthropic_models.all", tfjsonpath.New("models").AtSliceIndex(0).AtMapKey("max_input_tokens"), knownvalue.Int64Exact(1000000)),
				// The flat map is what configurations will actually branch on.
				statecheck.ExpectKnownValue("data.anthropic_models.all", tfjsonpath.New("models").AtSliceIndex(0).AtMapKey("capabilities").AtMapKey("code_execution"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue("data.anthropic_models.all", tfjsonpath.New("models").AtSliceIndex(1).AtMapKey("capabilities").AtMapKey("code_execution"), knownvalue.Bool(false)),
				// A capability that nests sub-capabilities must still report its
				// own top-level flag rather than being dropped for not fitting.
				statecheck.ExpectKnownValue("data.anthropic_models.all", tfjsonpath.New("models").AtSliceIndex(0).AtMapKey("capabilities").AtMapKey("context_management"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue("data.anthropic_models.all", tfjsonpath.New("ids"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.StringExact("claude-opus-5"),
					knownvalue.StringExact("claude-haiku-4-5-20251001"),
				})),
			},
		}},
	})
}

func TestAccModelDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_model" "opus" {
  id = "claude-opus-5"
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_model.opus", tfjsonpath.New("display_name"), knownvalue.StringExact("Claude Opus 5")),
				statecheck.ExpectKnownValue("data.anthropic_model.opus", tfjsonpath.New("max_tokens"), knownvalue.Int64Exact(128000)),
			},
		}},
	})
}

// The flat capabilities map cannot express a sub-capability, so the raw payload
// is carried alongside it. This asserts the nesting actually survives, which a
// check on the flat map alone would not notice.
func TestAccModelDataSource_capabilitiesJSONKeepsNesting(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_model" "opus" {
  id = "claude-opus-5"
}

output "compact_supported" {
  value = jsondecode(data.anthropic_model.opus.capabilities_json).context_management.compact_20260112.supported
}
`,
			Check: resource.TestCheckOutput("compact_supported", "false"),
		}},
	})
}

// Reading a model that does not exist must fail the plan. That is the reason
// the data source exists: a retired or mistyped id should stop before apply.
func TestAccModelDataSource_unknownModelFails(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_model" "gone" {
  id = "claude-opus-4-1"
}
`,
			ExpectError: regexp.MustCompile(`model not found`),
		}},
	})
}
