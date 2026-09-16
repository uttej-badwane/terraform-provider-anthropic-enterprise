---
page_title: "Managed Agents"
subcategory: ""
description: |-
  Build an agent, the environment it runs in, the secrets it can reach and a scheduled deployment, and understand the behaviours that differ from the rest of the provider.
---

# Managed Agents

The Managed Agents control plane is the most intricate surface this provider
covers, and the one where behaviour is least guessable from the schema alone.
This guide builds a working agent from the bottom up and explains the parts
that surprise people.

Managed Agents and the Skills API are **beta**.

## Credentials

Managed Agents uses a regular workspace API key (`sk-ant-api03-…`), not an Admin
key — Admin keys are rejected outright:

```sh
export ANTHROPIC_API_KEY="sk-ant-api03-..."
export ANTHROPIC_WORKSPACE_ID="wrkspc_..."   # only if the key reaches several workspaces
```

A configuration that manages both a Console organization and Managed Agents
sets `admin_api_key` and `api_key` together. See the
[credentials guide](./credentials).

## The pieces

| Resource | What it is |
|---|---|
| `anthropic_environment` | The sandbox an agent runs in: network policy, preinstalled packages |
| `anthropic_vault` / `anthropic_vault_credential` | Secrets the sandbox can use without seeing them in state |
| `anthropic_skill` | A folder of instructions uploaded from disk and versioned |
| `anthropic_memory_store` | Notes an agent keeps across sessions |
| `anthropic_agent` | The agent itself: model, tools, skills, MCP servers |
| `anthropic_deployment` | A scheduled or triggered run of one agent version |

## 1. An environment

The environment decides what the sandbox can reach. Keep `allowed_hosts` as
narrow as the work allows; it is the boundary between the agent and everything
else.

```hcl
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
```

## 2. A vault and a credential

Vault credentials are how an agent uses a secret without the secret entering
Terraform state.

```hcl
variable "github_token" {
  type      = string
  sensitive = true
}

resource "anthropic_vault" "ci" {
  display_name = "CI credentials"
}

resource "anthropic_vault_credential" "github" {
  vault_id       = anthropic_vault.ci.id
  display_name   = "GitHub token"
  secret_version = "2026-09"

  environment_variable = {
    secret_name     = "GH_TOKEN"
    secret_value    = var.github_token
    networking_type = "limited"
    allowed_hosts   = ["api.github.com"]
  }
}
```

`secret_value` is a **write-only attribute** (Terraform 1.11 or newer). It is
sent to the API and never persisted, which has two consequences worth
internalising:

- The provider cannot detect drift on it. If someone changes the secret out of
  band, Terraform will not notice.
- To rotate, change `secret_version`. That is what tells the provider to send
  the value again; editing `secret_value` alone changes nothing Terraform can
  see.

## 3. A skill

Skills upload from a local directory, which must contain `SKILL.md` at its root.

```hcl
resource "anthropic_skill" "release_notes" {
  source_dir = "${path.module}/skills/example-skill"
}
```

The provider hashes the directory contents with SHA-256 and creates a new
version when the hash changes. The API exposes no content hash of its own, so
**edits made outside Terraform are not detected** — the local directory is the
source of truth.

## 4. The agent

```hcl
resource "anthropic_agent" "reviewer" {
  name         = "code-reviewer"
  model        = "claude-opus-5"
  model_effort = "high"
  description  = "Reviews pull requests and leaves inline comments."
  system       = "You review code changes carefully and explain your reasoning briefly."

  tools = jsonencode([
    {
      type = "agent_toolset_20260401"
      configs = [
        { name = "bash", permission_policy = { type = "always_ask" } },
        { name = "web_fetch", allowed_domains = ["docs.example.com"] },
      ]
    },
    { type = "mcp_toolset", mcp_server_name = "issue-tracker" },
  ])

  mcp_servers = [
    { name = "issue-tracker", url = "https://mcp.example.com/sse" },
  ]

  skills = [
    { type = "custom", skill_id = anthropic_skill.release_notes.id, version = anthropic_skill.release_notes.latest_version_id },
    { type = "anthropic", skill_id = "xlsx" },
  ]
}
```

### Why `tools` is a JSON string

The API fills defaults into `tools` and `multiagent` on every response — a
`configs` entry you wrote with two fields comes back with five. Comparing that
literally would show a permanent diff.

The provider instead checks that **your JSON is a subset of the server's**. A
default the API filled in is not a change; a value you actually altered is.
This is why the attribute is `jsonencode(...)` rather than a typed block: it
keeps your intent distinct from the server's elaboration of it.

### Agents are versioned

Every effective update creates a **new version**. `anthropic_agent.version`
exports the current one, which matters for deployments below.

## 5. A deployment

```hcl
resource "anthropic_deployment" "nightly_report" {
  name           = "nightly-incident-report"
  agent_id       = anthropic_agent.reviewer.id
  agent_version  = anthropic_agent.reviewer.version
  environment_id = anthropic_environment.ci.id

  initial_events = jsonencode([
    {
      type    = "user.message"
      content = [{ type = "text", text = "Summarize yesterday's incidents and open a tracking issue." }]
    }
  ])

  schedule = {
    cron_expression = "0 6 * * 1-5"
    timezone        = "UTC"
  }

  vault_ids                  = [anthropic_vault.ci.id]
  budget_max_list_cost_cents = "50000"
}
```

Feeding `anthropic_agent.reviewer.version` into `agent_version` pins the
deployment to the agent as it is now. Edit the agent and Terraform creates a
new version and moves the deployment to it in the same apply, as one reviewable
change — rather than the deployment silently floating to whatever "latest"
means at run time.

Set `budget_max_list_cost_cents` on anything scheduled. A deployment that runs
on a cron with no cap is an open-ended spend commitment.

## Destroy semantics

This is the part worth reading before your first `terraform destroy`.

| Resource | On destroy | `delete_on_destroy` |
|---|---|---|
| `anthropic_agent` | **Archived** — the API has no delete | not available |
| `anthropic_deployment` | **Archived** — no delete | not available |
| `anthropic_environment` | Deleted | `false` archives instead |
| `anthropic_vault` | Deleted, **cascading to every credential in it** | `false` archives instead, purging secrets |
| `anthropic_vault_credential` | Deleted | `false` archives and purges the secret |
| `anthropic_memory_store` | Deleted | `false` archives instead |
| `anthropic_skill` | Deleted, **all versions** | `false` archives instead |

Two things to note. Agents and deployments **cannot** be deleted at all — destroy
archives them, permanently, and a replacement is a new object with a new id.

And destroying a vault takes every credential in it with it by default. If you only
meant to stop managing the vault, that is a `removed` block, not a destroy.

Set `paused = true` on a deployment to stop it running without archiving it.

## A note on beta headers

The Managed Agents and agent-memory betas use different headers, and the API
rejects a request that mixes them with a 400. The provider sends the right one
per endpoint; you should not need to think about it, but it explains why a
hand-rolled `curl` against these endpoints may fail where the provider
succeeds.
