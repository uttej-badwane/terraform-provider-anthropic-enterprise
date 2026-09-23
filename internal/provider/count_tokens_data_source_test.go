package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func countTokensConfig(extra string) string {
	return providerConfig + `
data "anthropic_count_tokens" "base" {
  model    = "claude-opus-5"
  messages = [{ role = "user", content = "What is the capital of France?" }]
}

` + extra
}

func TestAccCountTokensDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: countTokensConfig(`
data "anthropic_count_tokens" "with_system" {
  model    = "claude-opus-5"
  system   = "You are a concise assistant that answers in one sentence."
  messages = [{ role = "user", content = "What is the capital of France?" }]
}

data "anthropic_count_tokens" "with_tools" {
  model    = "claude-opus-5"
  messages = [{ role = "user", content = "What is the capital of France?" }]
  tools_json = jsonencode([{
    name         = "get_weather"
    description  = "Get the weather for a city"
    input_schema = { type = "object", properties = { city = { type = "string" } } }
  }])
}

data "anthropic_count_tokens" "multi_turn" {
  model = "claude-opus-5"
  messages = [
    { role = "user", content = "What is the capital of France?" },
    { role = "assistant", content = "Paris." },
    { role = "user", content = "And of Spain?" },
  ]
}

# The counts are compared rather than pinned: what matters is that each input
# is actually sent, which is what a system prompt or tools raising the count
# demonstrates.
output "system_raises_count" {
  value = data.anthropic_count_tokens.with_system.input_tokens > data.anthropic_count_tokens.base.input_tokens
}
output "tools_raise_count" {
  value = data.anthropic_count_tokens.with_tools.input_tokens > data.anthropic_count_tokens.base.input_tokens
}
output "turns_raise_count" {
  value = data.anthropic_count_tokens.multi_turn.input_tokens > data.anthropic_count_tokens.base.input_tokens
}
`),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("data.anthropic_count_tokens.base", "input_tokens"),
				resource.TestCheckOutput("system_raises_count", "true"),
				resource.TestCheckOutput("tools_raise_count", "true"),
				resource.TestCheckOutput("turns_raise_count", "true"),
			),
		}},
	})
}

// The endpoint rejects an empty messages list. Catching it at plan time is the
// point of the validator: the user sees the attribute at fault, not a 400.
func TestAccCountTokensDataSource_emptyMessagesRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_count_tokens" "empty" {
  model    = "claude-opus-5"
  messages = []
}
`,
			ExpectError: regexp.MustCompile(`(?s)Attribute messages list must contain at least 1\s+elements`),
		}},
	})
}

// A system prompt belongs in the top-level attribute; the endpoint rejects a
// system role inside messages.
func TestAccCountTokensDataSource_systemRoleRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_count_tokens" "bad_role" {
  model    = "claude-opus-5"
  messages = [{ role = "system", content = "You are helpful." }]
}
`,
			ExpectError: regexp.MustCompile(`(?s)value must be one of`),
		}},
	})
}

func TestAccCountTokensDataSource_invalidToolsJSON(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_count_tokens" "bad_tools" {
  model      = "claude-opus-5"
  messages   = [{ role = "user", content = "hi" }]
  tools_json = "{\"not\": \"an array\"}"
}
`,
			ExpectError: regexp.MustCompile(`Invalid tools_json`),
		}},
	})
}

// A retired model must fail the plan, which is the same guarantee the models
// data source gives.
func TestAccCountTokensDataSource_unknownModelFails(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_count_tokens" "gone" {
  model    = "claude-opus-4-1"
  messages = [{ role = "user", content = "hi" }]
}
`,
			ExpectError: regexp.MustCompile(`claude-opus-4-1`),
		}},
	})
}
