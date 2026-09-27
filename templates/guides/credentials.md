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

**Enterprise, compliance and analytics keys** — created by the primary owner of
a Claude Enterprise organization in **claude.ai → Organization settings → API**,
each carrying the scopes selected when it is created. The next section maps
scopes to what they unlock. None of these can be minted through the Admin API,
and neither can Console Admin keys, which is why `anthropic_api_key` is
import-only: the provider can manage a key's name and status but cannot create
one.

## Claude Enterprise keys and scopes

A key created in claude.ai can only do what its scopes allow, so a key that
works for one resource can fail on the next with a 403. Pick scopes from what
the configuration manages:

| Scope | Unlocks in this provider |
|---|---|
| `read:members` | Reading users and invites; `anthropic_rbac_roles`, and `anthropic_rbac_role` including its permissions. There is no separate role scope. |
| `write:members` | `anthropic_user` role changes and removal; `anthropic_invite` create and withdraw |
| `read:rbac_groups` | Reading `anthropic_rbac_group` and `anthropic_rbac_group_member` |
| `write:rbac_groups` | Managing `anthropic_rbac_group` and `anthropic_rbac_group_member`. Also needed by `anthropic_invite` when `rbac_group_ids` is set, because joining a group can grant its roles' permissions. |
| `read:spend_limits` | Reading spend limits and spend limit increase requests |
| `write:spend_limits` | Managing `anthropic_spend_limit` |
| `read:analytics` | Every `anthropic_analytics_*` data source |
| `read:compliance_org_data` | `anthropic_compliance_organizations`, `anthropic_compliance_role(s)`, `anthropic_compliance_groups`, `anthropic_compliance_effective_settings` |
| `read:compliance_user_data` | `anthropic_compliance_organization_users` and `anthropic_compliance_group_members`, which list people rather than directory structure |
| `read:org_audit` | A read-only scope covering every member, invite, group and role read, plus the Compliance API reads. It grants no writes. |

**Group and audit scopes need a key created for all organizations.** Groups
belong to the enterprise as a whole rather than to one organization, so
`read:rbac_groups`, `write:rbac_groups` and `read:org_audit` are unavailable on
a key limited to a single organization. In the key dialog they appear greyed out
until **All organizations** is selected.

**Which attribute a key goes in.** Anthropic calls the claude.ai key that
carries the members, groups and spend limit scopes an *Admin API key*, and it
begins `sk-ant-admin01-` like a Console Admin key does. In this provider it
still belongs in **`enterprise_api_key`**, not `admin_api_key`, which is for a
Claude Console organization. Put it in `admin_api_key` and members and invites
will work, because those endpoints are shared, but groups and spend limits will
ask for `enterprise_api_key`.

A key carrying the compliance scopes goes in `compliance_api_key`, and one
carrying `read:analytics` in `analytics_api_key`. Both fall back to
`enterprise_api_key`, so a single key holding every scope you need can be set
once.

**Retired scope.** Until June 30, 2026, effective organization settings needed
`read:compliance_org_settings`. That scope has been retired: a key carrying only
it now gets a 403 from `anthropic_compliance_effective_settings`. Create a key
with `read:compliance_org_data` instead.

**What is deliberately not covered.** `read:compliance_activities` returns the
activity feed, and beyond the two directory data sources above,
`read:compliance_user_data` also reaches chat, file and session content. Both
would copy personal data, including email addresses, IP addresses and
conversation text, into Terraform state, which is stored and shared far more
widely than an audit system. `delete:compliance_user_data`
is an irreversible one-off action rather than something to declare.
`read:plugins` and `write:plugins` have no documented API yet.

Anthropic documents these in
[User management](https://platform.claude.com/docs/en/manage-claude/user-management),
[Spend Limits API](https://platform.claude.com/docs/en/manage-claude/spend-limits-api)
and [Compliance organization data](https://platform.claude.com/docs/en/manage-claude/compliance-org-data).

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

## Where requests go

Every request goes to `base_url`, which defaults to `https://api.anthropic.com`
and must use `https` (plain `http` is accepted only for loopback addresses, so
the bundled mock server works). Redirects are refused rather than followed, so a
credential is never replayed against a host the configuration did not name.
Treat `ANTHROPIC_BASE_URL` in the environment of a Terraform run with the same
care as the credentials: whoever controls it decides where the keys are sent.
