package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccAnalyticsUsageReportDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_analytics_usage_report" "week" {
  starting_at = "2026-01-01T00:00:00Z"
  ending_at   = "2026-01-04T00:00:00Z"
  group_by    = ["model", "product"]
  products    = ["chat", "claude_code"]
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_analytics_usage_report.week", tfjsonpath.New("buckets"), knownvalue.ListSizeExact(3)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_usage_report.week", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("model"), knownvalue.StringExact("claude-sonnet-5")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_usage_report.week", tfjsonpath.New("total_uncached_input_tokens"), knownvalue.Int64Exact(6000)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_usage_report.week", tfjsonpath.New("total_requests"), knownvalue.Int64Exact(60)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_usage_report.week", tfjsonpath.New("data_refreshed_at"), knownvalue.StringExact("2026-01-17T12:00:00Z")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_usage_report.week", tfjsonpath.New("organization_id"), knownvalue.NotNull()),
			},
		}},
	})
}

func TestAccAnalyticsCostReportDataSource(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_analytics_cost_report" "week" {
  starting_at = "2026-01-01T00:00:00Z"
  ending_at   = "2026-01-04T00:00:00Z"
  group_by    = ["cost_type"]
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_analytics_cost_report.week", tfjsonpath.New("buckets"), knownvalue.ListSizeExact(3)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_cost_report.week", tfjsonpath.New("total_amount"), knownvalue.StringExact("123840.000000")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_cost_report.week", tfjsonpath.New("total_list_amount"), knownvalue.StringExact("154800.000000")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_cost_report.week", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("cost_type"), knownvalue.StringExact("tokens")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_cost_report.week", tfjsonpath.New("buckets").AtSliceIndex(0).AtMapKey("results").AtSliceIndex(0).AtMapKey("requests"), knownvalue.Null()),
			},
		}},
	})
}

func TestAccAnalyticsUserReportsDataSources(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_analytics_user_usage_report" "top" {
  starting_at = "2026-01-01T00:00:00Z"
  order_by    = "total_tokens"
  limit_rows  = 2
}
data "anthropic_analytics_user_cost_report" "top" {
  starting_at  = "2026-01-01T00:00:00Z"
  bucket_width = "1d"
  user_ids     = [data.anthropic_analytics_user_usage_report.top.rows[0].user_id]
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_analytics_user_usage_report.top", tfjsonpath.New("rows"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_user_usage_report.top", tfjsonpath.New("rows").AtSliceIndex(0).AtMapKey("total_tokens"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.anthropic_analytics_user_usage_report.top", tfjsonpath.New("rows").AtSliceIndex(0).AtMapKey("deleted"), knownvalue.Bool(false)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_user_cost_report.top", tfjsonpath.New("rows"), knownvalue.ListSizeExact(1)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_user_cost_report.top", tfjsonpath.New("rows").AtSliceIndex(0).AtMapKey("amount"), knownvalue.StringExact("1000.500000")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_user_cost_report.top", tfjsonpath.New("data_refreshed_at"), knownvalue.StringExact("2026-01-17T12:00:00Z")),
			},
		}},
	})
}

func TestAccAnalyticsEntityDataSources(t *testing.T) {
	skipUnlessMock(t)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_analytics_users" "day" {
  date     = "2026-01-15"
  group_by = ["rbac_group_id"]
}
data "anthropic_analytics_skills" "range" {
  starting_date = "2026-01-01"
  ending_date   = "2026-01-08"
  group_by      = ["product"]
  filters       = ["share_status:organization"]
}
data "anthropic_analytics_connectors" "day" {
  date       = "2026-01-15"
  limit_rows = 2
}
data "anthropic_analytics_plugins" "day" {
  date = "2026-01-15"
}
data "anthropic_analytics_artifacts" "day" {
  date = "2026-01-15"
}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("data.anthropic_analytics_users.day", tfjsonpath.New("rows").AtSliceIndex(0).AtMapKey("rbac_group_id"), knownvalue.NotNull()),
				statecheck.ExpectKnownValue("data.anthropic_analytics_users.day", tfjsonpath.New("rows").AtSliceIndex(1).AtMapKey("chat_message_count"), knownvalue.Int64Exact(20)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_users.day", tfjsonpath.New("rows").AtSliceIndex(1).AtMapKey("claude_code_tool_actions").AtSliceIndex(0).AtMapKey("tool"), knownvalue.StringExact("edit_tool")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_skills.range", tfjsonpath.New("rows"), knownvalue.ListSizeExact(3)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_skills.range", tfjsonpath.New("rows").AtSliceIndex(0).AtMapKey("product"), knownvalue.StringExact("claude_code")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_skills.range", tfjsonpath.New("rows").AtSliceIndex(0).AtMapKey("estimated_overage_spend"), knownvalue.StringExact("1250")),
				statecheck.ExpectKnownValue("data.anthropic_analytics_connectors.day", tfjsonpath.New("rows"), knownvalue.ListSizeExact(2)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_connectors.day", tfjsonpath.New("rows").AtSliceIndex(0).AtMapKey("read_call_count"), knownvalue.Int64Exact(40)),
				statecheck.ExpectKnownValue("data.anthropic_analytics_plugins.day", tfjsonpath.New("rows").AtSliceIndex(1).AtMapKey("plugin_id"), knownvalue.Null()),
				statecheck.ExpectKnownValue("data.anthropic_analytics_artifacts.day", tfjsonpath.New("rows"), knownvalue.ListSizeExact(4)),
			},
		}},
	})
}

func TestAccAnalyticsUsers_dateAndRangeConflict(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_analytics_users" "bad" {
  date          = "2026-01-15"
  starting_date = "2026-01-01"
}
`,
			ExpectError: regexp.MustCompile(`Exactly one of these attributes must be configured`),
		}},
	})
}

func TestAccAnalyticsSkills_badFilterDimension(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: providerConfig + `
data "anthropic_analytics_skills" "bad" {
  date    = "2026-01-15"
  filters = ["bogus:x"]
}
`,
			ExpectError: regexp.MustCompile(`must be one of the supported dimensions`),
		}},
	})
}
