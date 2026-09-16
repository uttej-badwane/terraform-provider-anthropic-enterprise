data "anthropic_compliance_groups" "all" {}

output "scim_groups" {
  value = [for g in data.anthropic_compliance_groups.all.groups : g.name if g.source_type == "scim"]
}
