data "anthropic_analytics_plugins" "day" {
  date     = "2026-01-15"
  group_by = ["product"]
}
