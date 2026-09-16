# API keys cannot be created through the Admin API. Create the key in the
# Claude Console, import it, then manage its name and status here.
resource "anthropic_api_key" "ci" {
  name                  = "CI pipeline"
  status                = "active"
  deactivate_on_destroy = true
}
