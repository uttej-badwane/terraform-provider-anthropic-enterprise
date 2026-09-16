data "anthropic_rate_limits" "all" {}

data "anthropic_rate_limits" "sonnet" {
  model = "claude-sonnet-5"
}
