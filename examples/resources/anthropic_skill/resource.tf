# The directory must contain SKILL.md; its basename becomes the skill folder.
resource "anthropic_skill" "release_notes" {
  source_dir = "${path.module}/skills/example-skill"
}

resource "anthropic_agent" "writer" {
  name  = "release-writer"
  model = "claude-sonnet-5"

  skills = [
    { type = "custom", skill_id = anthropic_skill.release_notes.id, version = anthropic_skill.release_notes.latest_version_id },
  ]
}
