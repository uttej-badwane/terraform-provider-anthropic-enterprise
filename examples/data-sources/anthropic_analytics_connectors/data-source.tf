# Connector usage for one day, restricted to one RBAC group.
data "anthropic_analytics_connectors" "engineering" {
  date    = "2026-01-15"
  filters = ["rbac_group_id:rbac_group_01ExampleGroupId00000000"]
}

output "write_calls_by_connector" {
  value = { for r in data.anthropic_analytics_connectors.engineering.rows : r.connector_name => r.write_call_count }
}
