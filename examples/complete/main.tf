// A complete, realistic configuration: a workspace, a human member, and a
// service account that GitHub Actions can assume through workload identity
// federation, so CI never holds a long-lived Anthropic API key.
//
// Credentials: this configuration needs an org:admin OAuth token, because
// service accounts and federation are not reachable with an Admin API key.
//
//   ant --profile admin auth login --scope org:admin
//   export ANTHROPIC_AUTH_TOKEN="$(ant auth print-credentials --profile admin --access-token)"

terraform {
  required_version = ">= 1.13"

  required_providers {
    anthropic = {
      source  = "uttej-badwane/anthropic-enterprise"
      version = "~> 0.1"
    }
  }
}

provider "anthropic" {
  // Reads ANTHROPIC_AUTH_TOKEN from the environment.
}

variable "github_repository" {
  description = "The owner/name of the GitHub repository allowed to assume the service account."
  type        = string
  default     = "example-org/example-repo"
}

variable "member_email" {
  description = "An existing organization user to add to the workspace."
  type        = string
  default     = "jane@example.com"
}

// The workspace that isolates this team's keys, members and rate limits.
resource "anthropic_workspace" "production" {
  name = "Production"

  tags = {
    managed_by = "terraform"
  }
}

// A human member. The user must already exist in the organization; invite them
// with anthropic_invite first if they do not.
data "anthropic_user" "member" {
  email = var.member_email
}

resource "anthropic_workspace_member" "member" {
  workspace_id   = anthropic_workspace.production.id
  user_id        = data.anthropic_user.member.id
  workspace_role = "workspace_developer"
}

// The identity CI acts as. Service accounts hold no password and no key; they
// are assumed through federation.
resource "anthropic_service_account" "ci" {
  name        = "ci-deploy"
  description = "Assumed by GitHub Actions through workload identity federation"
}

resource "anthropic_workspace_service_account" "ci" {
  workspace_id       = anthropic_workspace.production.id
  service_account_id = anthropic_service_account.ci.id
  workspace_role     = "workspace_developer"
}

// Trust GitHub's OIDC issuer.
resource "anthropic_federation_issuer" "github_actions" {
  name       = "github-actions"
  issuer_url = "https://token.actions.githubusercontent.com"
}

// Exchange a GitHub Actions OIDC token for a short-lived Anthropic credential,
// but only for workflows running on the default branch of one repository.
// Widening subject_prefix widens who can assume this service account, so keep
// it as specific as the workflow allows.
resource "anthropic_federation_rule" "ci_main" {
  name               = "ci-deploy-main"
  issuer_id          = anthropic_federation_issuer.github_actions.id
  service_account_id = anthropic_service_account.ci.id
  oauth_scope        = "workspace:inference"
  workspace_id       = anthropic_workspace.production.id

  match = {
    subject_prefix = "repo:${var.github_repository}:ref:refs/heads/main"
  }
}

output "workspace_id" {
  description = "Set this as ANTHROPIC_WORKSPACE_ID where the workspace must be named explicitly."
  value       = anthropic_workspace.production.id
}

output "service_account_id" {
  value = anthropic_service_account.ci.id
}
