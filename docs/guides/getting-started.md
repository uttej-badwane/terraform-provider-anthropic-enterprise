---
page_title: "Getting started"
subcategory: ""
description: |-
  Install the provider, pick a credential, and manage a first workspace.
---

# Getting started

This guide takes you from nothing to a managed workspace. It assumes Terraform
1.13 or newer, or OpenTofu 1.11 or newer.

## 1. Declare the provider

The provider is published as `uttej-badwane/anthropic-enterprise`, but its
resource and data source types use the `anthropic_` prefix. Terraform maps a
type such as `anthropic_workspace` to whichever provider is declared with the
local name `anthropic`, so declare it with that name:

```hcl
terraform {
  required_providers {
    anthropic = {
      source  = "uttej-badwane/anthropic-enterprise"
      version = "~> 0.1"
    }
  }
}

provider "anthropic" {}
```

Naming it anything else (for example `anthropic_enterprise`) leaves Terraform
looking for a provider called `anthropic` that is not in `required_providers`,
and planning fails before any request is made.

## 2. Choose a credential

Most of the organization is reachable with an Admin API key. Create one in the
Claude Console under **Settings → API keys**, then export it:

```sh
export ANTHROPIC_ADMIN_API_KEY="sk-ant-admin01-..."
```

An Admin key covers the organization, users, invites, workspaces, workspace
members, API key status, external keys and rate limits. Service accounts and
workload identity federation need an `org:admin` OAuth token instead, and the
Claude Enterprise, compliance, analytics and Managed Agents surfaces each take
their own credential. The [credentials guide](./credentials) explains which one
a given resource needs and how to obtain it.

Nothing needs to go in the configuration. Every credential attribute falls back
to an environment variable, which keeps secrets out of `.tf` files and out of
state.

## 3. Create a workspace

```hcl
resource "anthropic_workspace" "production" {
  name = "Production"
}

output "workspace_id" {
  value = anthropic_workspace.production.id
}
```

```sh
terraform init
terraform plan
terraform apply
```

## 4. Add a member

Workspace membership binds an existing organization user to a workspace:

```hcl
data "anthropic_user" "jane" {
  email = "jane@example.com"
}

resource "anthropic_workspace_member" "jane" {
  workspace_id   = anthropic_workspace.production.id
  user_id        = data.anthropic_user.jane.id
  workspace_role = "workspace_developer"
}
```

`anthropic_user` takes either `id` or `email`, so you rarely need to hardcode an
opaque user id. `anthropic_users` returns the whole directory when you want to
iterate over it instead.

The provider cannot create organization users; people join by accepting an
invite. Use `anthropic_invite` to send one, and manage the resulting user with
`anthropic_user` once they have accepted.

## 5. Know what destroy does

Several objects in this API archive rather than delete, and archiving is
irreversible. `terraform destroy` on an `anthropic_workspace` archives it, and
the workspace cannot be brought back — a replacement is a new workspace with a
new id. The same applies to service accounts, federation issuers and rules,
agents and deployments.

Resources that genuinely delete say so in their documentation, and the Managed
Agents resources accept `delete_on_destroy = false` to archive instead. Check
the destroy semantics table in the README before running destroy against an
organization you care about.

## Next steps

- [Choosing credentials](./credentials) — which key reaches which resource.
- [Adopting an existing organization](./import-existing-organization) — bring
  an organization you already have under Terraform without recreating anything.
- [Managed Agents](./managed-agents) — agents, environments, vaults and
  scheduled deployments, and the behaviours that differ from the rest of the
  provider.
