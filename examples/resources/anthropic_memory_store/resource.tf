resource "anthropic_memory_store" "shared_notes" {
  name        = "shared-notes"
  description = "Cross-session notes for the reviewer agent"
  metadata    = { team = "platform" }
}
