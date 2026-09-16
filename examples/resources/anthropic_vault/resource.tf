resource "anthropic_vault" "ci" {
  display_name = "CI credentials"
  metadata = {
    team = "platform"
  }
}
