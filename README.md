# Terraform Provider: Anthropic Enterprise

[![Terraform Registry](https://img.shields.io/github/v/release/uttej-badwane/terraform-provider-anthropic-enterprise?label=registry&color=7b42bc)](https://registry.terraform.io/providers/uttej-badwane/anthropic-enterprise/latest)
[![Tests](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/actions/workflows/test.yml/badge.svg)](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/actions/workflows/test.yml)
[![License: MPL-2.0](https://img.shields.io/badge/license-MPL--2.0-blue)](./LICENSE)
[![CodeQL](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/actions/workflows/codeql.yml/badge.svg)](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/uttej-badwane/terraform-provider-anthropic-enterprise/badge)](https://scorecard.dev/viewer/?uri=github.com/uttej-badwane/terraform-provider-anthropic-enterprise)

**[Documentation on the Terraform Registry](https://registry.terraform.io/providers/uttej-badwane/anthropic-enterprise/latest/docs)** ·
[Getting started](https://registry.terraform.io/providers/uttej-badwane/anthropic-enterprise/latest/docs/guides/getting-started) ·
[Choosing credentials](https://registry.terraform.io/providers/uttej-badwane/anthropic-enterprise/latest/docs/guides/credentials) ·
[Adopting an existing organization](https://registry.terraform.io/providers/uttej-badwane/anthropic-enterprise/latest/docs/guides/import-existing-organization)

Manage an Anthropic organization with Terraform through the [Admin API](https://platform.claude.com/docs/en/manage-claude/admin-api): workspaces, workspace members, invites, users, API key status, service accounts, workload identity federation, customer-managed encryption keys, and, for Claude Enterprise organizations, RBAC groups and per-user spend limits.

Built on the Terraform Plugin Framework (protocol 6). Works with Terraform 1.13+ and OpenTofu 1.11+. Both floors are exercised in CI. OpenTofu earlier than 1.11 rejects the write-only attributes that `anthropic_vault_credential` and `anthropic_deployment` rely on.

OpenTofu users: the provider is not yet listed in the OpenTofu registry (a listing request is in progress). Until it is, name the Terraform registry explicitly in `source`, which OpenTofu supports as-is:

```hcl
terraform {
  required_providers {
    anthropic = {
      source = "registry.terraform.io/uttej-badwane/anthropic-enterprise"
    }
  }
}
```

The provider is published as `uttej-badwane/anthropic-enterprise`. Its resource and data source types use the `anthropic_` prefix, so declare it with the local name `anthropic` exactly as in the example below; Terraform maps `anthropic_*` types to that local name.

## Usage

```hcl
terraform {
  required_providers {
    anthropic = {
      source = "uttej-badwane/anthropic-enterprise"
    }
  }
}

provider "anthropic" {
  # admin_api_key      = var.admin_api_key       # or ANTHROPIC_ADMIN_API_KEY
  # oauth_token        = var.oauth_token         # or ANTHROPIC_AUTH_TOKEN
  # enterprise_api_key = var.enterprise_api_key  # or ANTHROPIC_ENTERPRISE_API_KEY
}

resource "anthropic_workspace" "production" {
  name = "Production"
}

resource "anthropic_workspace_member" "alice" {
  workspace_id   = anthropic_workspace.production.id
  user_id        = "user_01ExampleUserId0000000000"
  workspace_role = "workspace_developer"
}
```

## Credentials

| Attribute | Env var | Reaches |
|---|---|---|
| `admin_api_key` | `ANTHROPIC_ADMIN_API_KEY` | Console org: organization, users, invites, workspaces, workspace members, API keys, external keys, rate limits |
| `oauth_token` | `ANTHROPIC_AUTH_TOKEN` | Everything above plus service accounts, federation issuers/rules, workspace service-account memberships |
| `enterprise_api_key` | `ANTHROPIC_ENTERPRISE_API_KEY` | Claude Enterprise org: users, invites, RBAC groups, RBAC roles, spend limits and increase requests |
| `compliance_api_key` | `ANTHROPIC_COMPLIANCE_API_KEY` | Compliance directory data sources (falls back to `enterprise_api_key` when it carries compliance scopes) |
| `analytics_api_key` | `ANTHROPIC_ANALYTICS_API_KEY` | Claude Enterprise analytics data sources (falls back to `enterprise_api_key` when it carries `read:analytics`) |
| `api_key` + `workspace_id` | `ANTHROPIC_API_KEY`, `ANTHROPIC_WORKSPACE_ID` | Managed Agents control plane and Skills API (beta). Regular workspace key; Admin keys are rejected |

Set only what you use. No resource stores secret material in Terraform state.

## Coverage

| Resource | Credential | Destroy semantics |
|---|---|---|
| `anthropic_workspace` | admin | archive (irreversible) |
| `anthropic_workspace_member` | admin | delete |
| `anthropic_api_key` | admin | import-only; manages `name` and `status` |
| `anthropic_invite` | admin or enterprise | delete while pending |
| `anthropic_user` | admin or enterprise | import-only; manages `role`; removal opt-in |
| `anthropic_service_account` | oauth | archive |
| `anthropic_workspace_service_account` | oauth | delete |
| `anthropic_federation_issuer` | oauth | archive |
| `anthropic_federation_rule` | oauth | archive |
| `anthropic_federation_rule_workspace` | oauth | delete |
| `anthropic_external_key` | admin | delete |
| `anthropic_rbac_group` | enterprise | delete |
| `anthropic_rbac_group_member` | enterprise | delete |
| `anthropic_spend_limit` | enterprise | delete |
| `anthropic_compliance_settings` | admin | singleton; destroy leaves the setting in place |
| `anthropic_agent` | api_key | archive (terminal); versioned, `version` exported for deployments |
| `anthropic_environment` | api_key | delete (or archive with `delete_on_destroy = false`) |
| `anthropic_vault` | api_key | delete cascades credentials (or archive) |
| `anthropic_vault_credential` | api_key | delete (or archive); secrets are write-only and never stored in state |
| `anthropic_deployment` | api_key | archive (no delete endpoint); `paused` drives pause/unpause |
| `anthropic_memory_store` | api_key | delete (or archive) |
| `anthropic_skill` | api_key | delete; local content hash drives new versions |

Data sources (68):

* Console: `anthropic_organization`, `anthropic_user`, `anthropic_users`, `anthropic_invite`, `anthropic_invites`, `anthropic_workspace`, `anthropic_workspaces`, `anthropic_workspace_member`, `anthropic_workspace_members`, `anthropic_api_key`, `anthropic_api_keys`, `anthropic_rate_limits`, `anthropic_workspace_rate_limits`, `anthropic_external_key`, `anthropic_external_keys`, `anthropic_compliance_settings`
* OAuth: `anthropic_service_account`, `anthropic_service_accounts`, `anthropic_workspace_service_accounts`, `anthropic_federation_issuer`, `anthropic_federation_issuers`, `anthropic_federation_rule`, `anthropic_federation_rules`, `anthropic_federation_rule_workspaces`
* Claude Enterprise: `anthropic_rbac_group`, `anthropic_rbac_groups`, `anthropic_rbac_group_members`, `anthropic_rbac_role`, `anthropic_rbac_roles`, `anthropic_spend_limit`, `anthropic_spend_limits`, `anthropic_spend_limit_increase_request`, `anthropic_spend_limit_increase_requests`
* Reports (point-in-time, re-read every plan): `anthropic_usage_report`, `anthropic_cost_report`, `anthropic_claude_code_usage_report`
* Enterprise analytics (`analytics_api_key`): `anthropic_analytics_summaries`, `anthropic_analytics_users`, `anthropic_analytics_skills`, `anthropic_analytics_connectors`, `anthropic_analytics_plugins`, `anthropic_analytics_artifacts`, `anthropic_analytics_usage_report`, `anthropic_analytics_cost_report`, `anthropic_analytics_user_usage_report`, `anthropic_analytics_user_cost_report`
* Managed Agents (`api_key`): `anthropic_agent`, `anthropic_agents`, `anthropic_agent_versions`, `anthropic_environment`, `anthropic_environments`, `anthropic_vault`, `anthropic_vaults`, `anthropic_vault_credentials`, `anthropic_deployment`, `anthropic_deployments`, `anthropic_memory_store`, `anthropic_memory_stores`, `anthropic_skill`, `anthropic_skills`, `anthropic_skill_versions`
* Compliance directory: `anthropic_compliance_organizations`, `anthropic_compliance_organization_users`, `anthropic_compliance_roles`, `anthropic_compliance_role`, `anthropic_compliance_groups`, `anthropic_compliance_group_members`, `anthropic_compliance_effective_settings`

Not covered, because the API has no write endpoint: rate-limit overrides, organization or seat-tier spend caps, custom roles and role-to-group attachment, SCIM groups, admin or owner org roles, Claude Code settings, SSO, IP allowlists, retention, invite domains, and creation of Admin, Compliance or Analytics keys. Compliance Activity Feed, chat and file content endpoints are event streams and are also out of scope. Deprecated `tunnels` endpoints are skipped.

## Managed Agents

The Managed Agents control plane (agents, environments, vaults, credentials, scheduled deployments, memory stores) and the Skills API are in beta and use a regular workspace API key (`api_key`), plus `workspace_id` when the key can reach several workspaces. Things to know:

* Agents are versioned. Every effective update creates a new version; `anthropic_agent.version` feeds `anthropic_deployment.agent_version` so deployments pin explicitly instead of floating to latest.
* The API fills defaults into `tools` and `multiagent` on every response. The provider compares your JSON as a subset of the server's, so filled defaults never show as drift while real changes do.
* Vault credential secrets are write-only attributes (Terraform 1.11 or newer). They are sent to the API, never written to state, and cannot be drift-detected. Bump `secret_version` to rotate.
* Skills are uploaded from a local directory. A SHA-256 over the files decides when a new version is created; the API exposes no content hash, so remote edits are not detected.
* Agents and deployments cannot be deleted through the API; destroy archives them. Environments, vaults, credentials, memory stores and skills delete for real unless `delete_on_destroy = false`.

## Contributing

[![good first issues](https://img.shields.io/github/issues/uttej-badwane/terraform-provider-anthropic-enterprise/good%20first%20issue?label=good%20first%20issues&color=7057ff)](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
[![help wanted](https://img.shields.io/github/issues/uttej-badwane/terraform-provider-anthropic-enterprise/help%20wanted?label=help%20wanted&color=008672)](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22)

Contributions are welcome, and **you do not need an Anthropic account to make
one**. The acceptance suite runs against an in-process mock of every API the
provider touches, so `make testacc` works on a fresh clone with no credentials,
and pull requests from forks get the full suite in CI automatically.

**[Good first issues](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)** ·
**[Help wanted](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22)** ·
[All open issues](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues)

Currently open, smallest first:

| Issue | What it needs |
|---|---|
| [#21](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/21) | Unit tests for the eventual-consistency helpers. Pure Go, no Terraform, no credentials |
| [#20](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/20) | An acceptance test for `anthropic_external_key` updates, against the mock |
| [#18](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/18) | Expose the request timeout and retry count as provider attributes |
| [#19](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/19) | `timeouts` blocks on the resources that wait for eventual consistency |
| [#22](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/22) | An ephemeral resource for federation token exchange. The largest, and the most interesting |

Each issue names the files to change and the conventions to follow. Comment to
claim one and it will be assigned to you — the maintainer is not working on any
of them.

[CONTRIBUTING.md](./CONTRIBUTING.md) covers the workflow, how to run the suite,
and how to drive the provider against the standalone mock server.

## Security

Read [SECURITY.md](./SECURITY.md) for the disclosure process and what is in scope. In short: no secret reaches state or logs, credentials only travel to the configured `https` base URL, redirects are refused, and every URL attribute that decides where a token goes must be `https`.

Releases are GPG-signed (verified by the registry on ingest) and carry SLSA build provenance. To check a download:

```sh
gh attestation verify terraform-provider-anthropic-enterprise_<version>_<os>_<arch>.zip \
  --repo uttej-badwane/terraform-provider-anthropic-enterprise
```

## Governance

One maintainer today ([@uttej-badwane](https://github.com/uttej-badwane)). Every change, including the maintainer's own, lands through a pull request with the full check suite green; `main` is protected and releases run in a gated environment. The plan for growing that is in [ROADMAP.md](./ROADMAP.md): regular contributors are invited to become maintainers with review and release rights, so the project does not depend on one person.

## Development

A standalone mock of the Admin API ships with the repo for offline testing with the real CLI:

```sh
go run ./internal/mock/cmd/mockserver          # prints the env vars to export
make install && eval "$(make -s dev-override)"  # dev_overrides for terraform / tofu
```

See [CONTRIBUTING.md](CONTRIBUTING.md). Releases follow [RELEASING.md](RELEASING.md).

## License

[Mozilla Public License 2.0](LICENSE)
