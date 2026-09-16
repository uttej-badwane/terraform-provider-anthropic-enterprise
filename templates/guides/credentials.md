---
page_title: "Choosing credentials"
subcategory: ""
description: |-
  Which of the provider's credentials reaches which resource, how to obtain each one, and how to read the errors when the wrong one is used.
---

# Choosing credentials

This provider spans several Anthropic APIs that do not share an authentication
scheme. Rather than pretend they are one credential, it exposes one attribute
per credential class and routes every request to the one its endpoint accepts.
Configure only the classes you actually use.

## The classes

| Attribute | Environment variable | Reaches |
|---|---|---|
| `admin_api_key` | `ANTHROPIC_ADMIN_API_KEY` | Console organization: organization, users, invites, workspaces, workspace members, API keys, external keys, rate limits |
| `oauth_token` | `ANTHROPIC_AUTH_TOKEN` | Everything an Admin key reaches, plus service accounts, federation issuers and rules, and workspace service-account membership |
| `enterprise_api_key` | `ANTHROPIC_ENTERPRISE_API_KEY` | Claude Enterprise organization: users, invites, RBAC groups and roles, spend limits and increase requests |
| `compliance_api_key` | `ANTHROPIC_COMPLIANCE_API_KEY` | Compliance directory and effective settings |
| `analytics_api_key` | `ANTHROPIC_ANALYTICS_API_KEY` | Claude Enterprise analytics data sources |
| `api_key` (with `workspace_id`) | `ANTHROPIC_API_KEY`, `ANTHROPIC_WORKSPACE_ID` | Managed Agents control plane and the Skills API (beta) |

`compliance_api_key` and `analytics_api_key` fall back to `enterprise_api_key`
when they are not set, which is correct when that key already carries the
compliance scopes or `read:analytics`. Set them separately only when you issue
narrower keys per surface.

Every attribute falls back to its environment variable, so a configuration that
sets none of them still works:

```hcl
provider "anthropic" {}
```

## Picking one

Start from what you are managing:

- **Workspaces, members, invites, users, API key status, external keys** —
  `admin_api_key`.
- **Service accounts, or federation for CI** — `oauth_token`. An Admin key is
  rejected by these endpoints; there is no way around it.
- **RBAC groups, roles, per-user spend limits** — `enterprise_api_key`. These
  are claude.ai Enterprise features and do not exist in a Console organization.
- **Agents, environments, vaults, deployments, memory stores, skills** —
  `api_key`, a regular workspace key. Admin keys are rejected here too.

A configuration that manages both a Console organization and Managed Agents
sets two credentials, and that is expected.

## Obtaining each credential

**Admin API key** — Claude Console, **Settings → API keys**. Begins
`sk-ant-admin01-`.

**OAuth token with `org:admin`** — issued through the `ant` CLI:

```sh
ant --profile admin auth login --scope org:admin
export ANTHROPIC_AUTH_TOKEN="$(ant auth print-credentials --profile admin --access-token)"
```

Access tokens are short-lived. Re-export before a run, or wire the command into
whatever populates your environment; the provider does not refresh the token
for you.

**Workspace API key** — Claude Console, scoped to a workspace. Begins
`sk-ant-api03-`. If the key can reach more than one workspace, set
`workspace_id` so Managed Agents calls target the right one.

**Enterprise, compliance and analytics keys** — issued by Anthropic for a
claude.ai Enterprise organization. They cannot be minted through the Admin API,
and neither can Admin keys, which is why `anthropic_api_key` is import-only:
the provider can manage a key's name and status but cannot create one.

## Reading the errors

**`401 API key is invalid`** — the credential is not accepted at all. Before
assuming a typo, check whether the key was archived: an archived key is
structurally valid and still fails. List the organization's keys and look at
`status`; archiving is irreversible, so an archived key must be replaced with a
newly created one rather than re-enabled.

**`401` on Managed Agents endpoints while Console resources work** — an Admin
key is being used where a workspace key is required. Set `api_key`.

**`403` or a permission error on service accounts or federation** — an Admin
key is being used where `oauth_token` is required, or the token was issued
without `org:admin`. Check the scope on the profile you logged in with.

**A resource plans but its data source returns nothing** — the credential is
valid but points at a different organization than you expect. Confirm with the
`anthropic_organization` data source, which reports the id and name the current
credential resolves to.

## Secrets and state

No resource writes secret material to Terraform state. API keys cannot be
created through the API, so there is no key material to store. Vault credential
secrets use write-only attributes (Terraform 1.11 or newer): they are sent to
the API and never persisted, which also means they cannot be drift-detected —
bump `secret_version` to rotate.

Prefer the environment variables over provider attributes for the credentials
themselves. An attribute set from a variable is fine, but a credential written
literally into a `.tf` file tends to end up committed.
