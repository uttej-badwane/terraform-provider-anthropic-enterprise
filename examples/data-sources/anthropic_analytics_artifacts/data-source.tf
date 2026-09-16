# Artifacts created on one day, by MIME type and share state.
data "anthropic_analytics_artifacts" "day" {
  date    = "2026-01-15"
  filters = ["is_shared:true"]
}
