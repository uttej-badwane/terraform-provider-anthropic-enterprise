data "anthropic_skills" "custom" {
  source = "custom"
}

output "custom_skill_ids" {
  value = data.anthropic_skills.custom.skills[*].id
}
