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

func TestAccSpendLimitResource(t *testing.T) {
	skipUnlessMock(t)
	user := testMock.UserByEmail("dev@example.com")
	cfg := func(amount string) string {
		return providerConfig + fmt.Sprintf(`
resource "anthropic_spend_limit" "test" {
  user_id = %q
  %s
}
`, user.ID, amount)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(`amount = "50000"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_spend_limit.test", tfjsonpath.New("id"), knownvalue.StringRegexp(regexp.MustCompile(`^spl_`))),
					statecheck.ExpectKnownValue("anthropic_spend_limit.test", tfjsonpath.New("amount"), knownvalue.StringExact("50000")),
					statecheck.ExpectKnownValue("anthropic_spend_limit.test", tfjsonpath.New("period"), knownvalue.StringExact("monthly")),
					statecheck.ExpectKnownValue("anthropic_spend_limit.test", tfjsonpath.New("currency"), knownvalue.StringExact("USD")),
				},
			},
			{
				ResourceName:      "anthropic_spend_limit.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: cfg(`amount = "25000"`),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_spend_limit.test", tfjsonpath.New("amount"), knownvalue.StringExact("25000")),
				},
			},
			{
				Config: cfg(``),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("anthropic_spend_limit.test", tfjsonpath.New("amount"), knownvalue.Null()),
				},
			},
		},
	})
}

func TestAccSpendLimitResource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + `
resource "anthropic_spend_limit" "bad" {
  user_id = "user_01ExampleUserId0000000000"
  amount  = "12.50"
}`,
				ExpectError: regexp.MustCompile(`must be a non-negative integer string in cents`),
			},
			{
				Config: providerConfig + `
resource "anthropic_spend_limit" "bad" {
  user_id = "user_01ExampleUserId0000000000"
  period  = "yearly"
}`,
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}

func TestAccSpendLimitsDataSource(t *testing.T) {
	skipUnlessMock(t)
	user := testMock.UserByEmail("dev@example.com")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig + fmt.Sprintf(`
resource "anthropic_spend_limit" "test" {
  user_id = %q
  amount  = "70000"
}

data "anthropic_spend_limits" "dev" {
  user_ids = [%q]
  periods  = ["monthly"]
  depends_on = [anthropic_spend_limit.test]
}
`, user.ID, user.ID),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.anthropic_spend_limits.dev", tfjsonpath.New("spend_limits"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.ObjectPartial(map[string]knownvalue.Check{
							"user_id":     knownvalue.StringExact(user.ID),
							"email":       knownvalue.StringExact("dev@example.com"),
							"amount":      knownvalue.StringExact("70000"),
							"period":      knownvalue.StringExact("monthly"),
							"source_type": knownvalue.StringExact("user"),
							"source_id":   knownvalue.StringExact(user.ID),
						}),
					})),
				},
			},
		},
	})
}
