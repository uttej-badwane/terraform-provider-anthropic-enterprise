package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccSpendLimitIncreaseRequestsDataSources(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_spend_limit_increase_requests" "all" {}
data "anthropic_spend_limit_increase_requests" "pending" { statuses = ["pending"] }
data "anthropic_spend_limit_increase_request" "first" {
  id = data.anthropic_spend_limit_increase_requests.pending.requests[0].id
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_spend_limit_increase_requests.all", tfjsonpath.New("requests"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_spend_limit_increase_requests.pending", tfjsonpath.New("requests"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.ObjectPartial(map[string]knownvalue.Check{
						"status":               knownvalue.StringExact("pending"),
						"current_amount":       knownvalue.StringExact("10000"),
						"period_to_date_spend": knownvalue.StringExact("9950.5"),
						"resolved_by_type":     knownvalue.Null(),
					}),
				})),
				statecheck.ExpectKnownValue("data.anthropic_spend_limit_increase_request.first", tfjsonpath.New("status"), knownvalue.StringExact("pending")),
				statecheck.ExpectKnownValue("data.anthropic_spend_limit_increase_request.first", tfjsonpath.New("actor_email"), knownvalue.StringExact("user@example.com")),
			},
		}},
	})
}

func TestAccUsageReportDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_usage_report" "daily" {
  starting_at = "2026-01-01T00:00:00Z"
  ending_at   = "2026-01-05T00:00:00Z"
  group_by    = ["model", "workspace_id"]
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_usage_report.daily", tfjsonpath.New("buckets"), knownvalue.ListSizeExact(4)),
				statecheck.ExpectKnownValue("data.anthropic_usage_report.daily", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("model"), knownvalue.StringExact("claude-sonnet-5")),
				statecheck.ExpectKnownValue("data.anthropic_usage_report.daily", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("api_key_id"), knownvalue.Null()),
				// 1500 * (1+2+3+4)
				statecheck.ExpectKnownValue("data.anthropic_usage_report.daily", tfjsonpath.New("total_uncached_input_tokens"), knownvalue.Int64Exact(15000)),
				statecheck.ExpectKnownValue("data.anthropic_usage_report.daily", tfjsonpath.New("total_output_tokens"), knownvalue.Int64Exact(5000)),
			},
		}},
	})
}

// The speed dimension only works when the client sends the fast-mode beta
// header, and the mock rejects it otherwise exactly as the API does. So this
// passing is evidence the header actually went out, not just that the schema
// accepts the attribute.
func TestAccUsageReportDataSource_speed(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_usage_report" "fast" {
  starting_at = "2026-01-01T00:00:00Z"
  ending_at   = "2026-01-03T00:00:00Z"
  group_by    = ["model", "speed"]
  speeds      = ["fast"]
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_usage_report.fast", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("speed"), knownvalue.StringExact("fast")),
				// Grouping by speed must not disturb the other dimensions.
				statecheck.ExpectKnownValue("data.anthropic_usage_report.fast", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("model"), knownvalue.StringExact("claude-sonnet-5")),
			},
		}},
	})
}

// A report that never mentions speed must still come back without it, which is
// the case that breaks if the header is ever made unconditional and the API
// starts returning the dimension to everyone.
func TestAccUsageReportDataSource_speedAbsentWhenNotGrouped(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_usage_report" "plain" {
  starting_at = "2026-01-01T00:00:00Z"
  ending_at   = "2026-01-03T00:00:00Z"
  group_by    = ["model"]
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_usage_report.plain", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("speed"), knownvalue.Null()),
			},
		}},
	})
}

func TestAccUsageReportDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_usage_report" "bad" {
  starting_at  = "2026-01-01T00:00:00Z"
  bucket_width = "1w"
}
`,
			ExpectError: regexp.MustCompile(`value must be one of`),
		}},
	})
}

func TestAccCostReportDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_cost_report" "grouped" {
  starting_at = "2026-01-01T00:00:00Z"
  group_by    = ["description"]
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_cost_report.grouped", tfjsonpath.New("buckets"), knownvalue.ListSizeExact(3)),
				statecheck.ExpectKnownValue("data.anthropic_cost_report.grouped", tfjsonpath.New("total_amount"), knownvalue.StringExact("370.5")),
				statecheck.ExpectKnownValue("data.anthropic_cost_report.grouped", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("token_type"), knownvalue.StringExact("uncached_input_tokens")),
			},
		}},
	})
}

func TestAccClaudeCodeUsageReportDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_claude_code_usage_report" "day" { date = "2026-01-01" }
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_claude_code_usage_report.day", tfjsonpath.New("records"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_claude_code_usage_report.day", tfjsonpath.New("records").AtSliceIndex(0).AtMapKey("actor_email"), knownvalue.StringExact("dev@example.com")),
				statecheck.ExpectKnownValue("data.anthropic_claude_code_usage_report.day", tfjsonpath.New("records").AtSliceIndex(0).AtMapKey("commits"), knownvalue.Int64Exact(8)),
				statecheck.ExpectKnownValue("data.anthropic_claude_code_usage_report.day", tfjsonpath.New("records").AtSliceIndex(0).AtMapKey("tool_actions"), knownvalue.ListExact([]knownvalue.Check{
					knownvalue.ObjectExact(map[string]knownvalue.Check{"tool": knownvalue.StringExact("edit_tool"), "accepted": knownvalue.Int64Exact(25), "rejected": knownvalue.Int64Exact(3)}),
					knownvalue.ObjectExact(map[string]knownvalue.Check{"tool": knownvalue.StringExact("write_tool"), "accepted": knownvalue.Int64Exact(8), "rejected": knownvalue.Int64Exact(0)}),
				})),
				statecheck.ExpectKnownValue("data.anthropic_claude_code_usage_report.day", tfjsonpath.New("records").AtSliceIndex(1).AtMapKey("actor_api_key_name"), knownvalue.StringExact("Developer Key")),
				statecheck.ExpectKnownValue("data.anthropic_claude_code_usage_report.day", tfjsonpath.New("records").AtSliceIndex(1).AtMapKey("is_remote"), knownvalue.Bool(true)),
			},
		}},
	})
}

func TestAccClaudeCodeUsageReportDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config:      providerConfig + `data "anthropic_claude_code_usage_report" "bad" { date = "yesterday" }`,
			ExpectError: regexp.MustCompile(`must be YYYY-MM-DD`),
		}},
	})
}

func TestAccAnalyticsSummariesDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_analytics_summaries" "week" {
  starting_date = "2026-01-01"
  ending_date   = "2026-01-04"
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_analytics_summaries.week", tfjsonpath.New("summaries"), knownvalue.ListSizeExact(3)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_summaries.week", tfjsonpath.New("summaries").AtSliceIndex(0).AtMapKey("pending_invite_count"), knownvalue.Int64Exact(1)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_summaries.week", tfjsonpath.New("summaries").AtSliceIndex(0).AtMapKey("daily_adoption_rate"), knownvalue.Float64Exact(0.6)),
			},
		}},
	})
}
