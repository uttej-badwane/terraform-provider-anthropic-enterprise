---
page_title: "Adopting an existing organization"
subcategory: ""
description: |-
  Bring an organization you already have under Terraform management using import blocks, without recreating or disturbing anything.
---

# Adopting an existing organization

Almost nobody starts with an empty organization. This guide brings one that
already exists under management without recreating anything, using `import`
blocks and generated configuration.

Every resource in this provider supports import.

## Why import blocks

The `terraform import` command imports one object into state and leaves you to
write the matching configuration by hand. `import` blocks (Terraform 1.5 and
newer) are declarative, run inside a normal plan, and can generate the
configuration for you. For an organization with dozens of workspaces and
members, that difference is the whole job.

Nothing is written until you apply, so the discovery and generation steps below
are safe to run against production.

## 1. Discover what exists

Use the data sources to enumerate the organization before importing anything.
Put this in a scratch directory:

```hcl
data "anthropic_organization" "current" {}
data "anthropic_workspaces" "all" {}
data "anthropic_users" "all" {}

output "organization" {
  value = data.anthropic_organization.current.name
}

output "workspaces" {
  value = { for w in data.anthropic_workspaces.all.workspaces : w.name => w.id }
}

output "users" {
  value = { for u in data.anthropic_users.all.users : u.email => u.id }
}
```

```sh
terraform apply
```

Confirm the organization name is the one you meant to touch before going
further. A credential pointing at the wrong organization is the single most
expensive mistake available here.

## 2. Write import blocks

Each block names the address you want and the object's id:

```hcl
import {
  to = anthropic_workspace.production
  id = "wrkspc_01ExampleWorkspaceId000000"
}

import {
  to = anthropic_workspace_member.jane_production
  id = "wrkspc_01ExampleWorkspaceId000000/user_01ExampleUserId0000000000"
}
```

## 3. Generate the configuration

```sh
terraform plan -generate-config-out=generated.tf
```

Terraform writes HCL for every imported address into `generated.tf`. Review it
before applying — generated configuration is faithful rather than idiomatic. It
hardcodes ids that you probably want as references, and it includes optional
attributes you may prefer to leave unset.

Then apply:

```sh
terraform apply
```

A clean adoption ends with `terraform plan` reporting no changes. If the plan
wants to modify something, the configuration does not yet match reality; fix
the configuration rather than applying, until the plan is empty.

## Import id formats

Most resources import by their own id. Membership-style resources are keyed by
their two parents, joined with `/`.

| Resource | Import id |
|---|---|
| `anthropic_workspace` | `wrkspc_…` |
| `anthropic_workspace_member` | `wrkspc_…/user_…` |
| `anthropic_api_key` | `apikey_…` |
| `anthropic_invite` | `invite_…` |
| `anthropic_user` | `user_…` |
| `anthropic_external_key` | `ekey_…` |
| `anthropic_service_account` | `svac_…` |
| `anthropic_workspace_service_account` | `wrkspc_…/svac_…` |
| `anthropic_federation_issuer` | `fdis_…` |
| `anthropic_federation_rule` | `fdrl_…` |
| `anthropic_federation_rule_workspace` | `fdrl_…/wrkspc_…` |
| `anthropic_rbac_group` | `rbac_group_…` |
| `anthropic_rbac_group_member` | `rbac_group_…/user_…` |
| `anthropic_spend_limit` | `spl_…` |
| `anthropic_compliance_settings` | `compliance_settings` |
| `anthropic_agent` | `agent_…` |
| `anthropic_environment` | `env_…` |
| `anthropic_vault` | `vlt_…` |
| `anthropic_vault_credential` | `vlt_…/vcrd_…` |
| `anthropic_deployment` | `depl_…` |
| `anthropic_memory_store` | `memstore_…` |
| `anthropic_skill` | `skill_…` |

`anthropic_compliance_settings` is a singleton, so its id is the literal string
`compliance_settings`.

## Suggested order

Import parents before children, so generated configuration can be rewritten to
reference them:

1. Workspaces.
2. Workspace members and workspace service accounts.
3. Service accounts, then federation issuers, then federation rules, then rule
   workspace attachments.
4. Claude Enterprise: RBAC groups, then group members, then spend limits.
5. Managed Agents: environments and vaults, then vault credentials, then agents
   and skills, then deployments.

Adopt one group, get to an empty plan, commit, then move to the next. A single
import run covering an entire organization is difficult to review and harder to
unpick when one address is wrong.

## Things to know before you start

**Import-only resources.** `anthropic_api_key` and `anthropic_user` cannot be
created by the provider, because the API has no endpoint to create them. They
exist to manage a key's name and status, and a user's role, on objects that are
already there. Import is the only way they enter state.

**Write-only attributes do not import.** `anthropic_vault_credential` secrets
are write-only and never stored in state, so an imported credential has no
secret in configuration. Supply the secret and a `secret_version` to take over
rotation, or leave it and let the existing secret stand.

**Skills compare on local content.** `anthropic_skill` decides when to create a
new version by hashing the local directory. After importing, point `source_dir`
at content matching what was uploaded, or the next apply creates a new version.

**Destroy is not the inverse of import.** Removing a resource from
configuration and applying will archive or delete a real object. Workspaces,
service accounts, federation issuers and rules, agents and deployments archive
irreversibly. If you only want Terraform to stop managing something, use a
`removed` block with `lifecycle { destroy = false }`, not a plain deletion.

To make that mistake harder right after adoption, every `archive_on_destroy` or
`delete_on_destroy` attribute is imported as `false`, whatever its default.
The first plan after an import therefore shows one change on each imported
resource: the flag moving from `false` to its default of `true`. Read that line
as a question. Apply it to accept the normal destroy behaviour, or set the
attribute to `false` in configuration to keep the object safe from a later
`terraform destroy`. Nothing is sent to the API either way.

**Reports are point-in-time.** The usage, cost and analytics data sources
re-read on every plan and their values change between runs. That is expected,
not drift.
