# Anthropic-published skills are addressed by name.
data "anthropic_skill" "xlsx" {
  id = "xlsx"
}

output "xlsx_latest_version" {
  value = data.anthropic_skill.xlsx.latest_version_id
}
