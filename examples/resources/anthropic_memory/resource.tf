resource "anthropic_memory_store" "conventions" {
  name        = "conventions"
  description = "Team conventions the reviewer agent reads"
}

resource "anthropic_memory" "style_guide" {
  memory_store_id = anthropic_memory_store.conventions.id
  path            = "/style/go.md"
  content         = file("${path.module}/conventions/go.md")
}

# A memory agents keep editing: create it once and leave its content to them.
resource "anthropic_memory" "open_questions" {
  memory_store_id = anthropic_memory_store.conventions.id
  path            = "/notes/open-questions.md"
  content         = ""

  lifecycle {
    ignore_changes = [content]
  }
}
