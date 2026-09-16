resource "anthropic_environment" "ci" {
  name        = "ci-runners"
  description = "Restricted network, Python tooling preinstalled"
  type        = "cloud"

  networking = {
    type                   = "limited"
    allowed_hosts          = ["api.example.com", "*.pypi.org"]
    allow_mcp_servers      = true
    allow_package_managers = true
  }

  packages = {
    pip = ["requests", "pyyaml"]
  }
}
