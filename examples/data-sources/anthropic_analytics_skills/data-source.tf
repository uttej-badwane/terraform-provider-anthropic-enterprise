# Skill usage rolled up over a week, per product surface.
data "anthropic_analytics_skills" "week" {
  starting_date = "2026-01-01"
  ending_date   = "2026-01-08"
  group_by      = ["product"]
  filters       = ["share_status:organization"]
  order_by      = "distinct_user_count"
  order         = "desc"
  limit_rows    = 25
}
