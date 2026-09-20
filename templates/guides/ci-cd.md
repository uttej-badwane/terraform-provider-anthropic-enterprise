---
page_title: "Running in CI without a long-lived key"
subcategory: ""
description: |-
  Exchange a pipeline's own OIDC token for a short-lived Anthropic credential, so no API key is ever stored in the CI system.
---

# Running in CI without a long-lived key

A pipeline that calls the Anthropic API normally holds an API key in its secret
store. That key does not expire, it works from anywhere, and anyone who can read
the secret or edit the workflow can use it.

Workload identity federation replaces it. The pipeline proves who it is with a
token its own platform mints for that job, and exchanges it for an Anthropic
credential that lasts minutes and is scoped to one workspace. Nothing long-lived
is stored anywhere.

This guide uses GitHub Actions. The shape is the same for any OIDC issuer; only
where the assertion comes from changes.

~> Workload identity federation is in beta. `anthropic_federation_token`
requires Terraform 1.10 or later. The provider supports OpenTofu from 1.11.

## What the pipeline needs from its own platform

GitHub mints an OIDC token for a job only when the workflow asks for one:

```yaml
permissions:
  id-token: write   # required, or no token is minted
  contents: read
```

The token's `sub` claim identifies the workflow — for example
`repo:your-org/your-repo:ref:refs/heads/main`. That claim is what the federation
rule matches on, so it is worth looking at a decoded token before writing the
rule:

```sh
jq -rR 'split(".")[1] | gsub("-";"+") | gsub("_";"/") | @base64d | fromjson' <<< "$TOKEN"
```

## The trust configuration

Three objects, managed with an `org:admin` OAuth token (see the
[credentials guide](./credentials)):

```hcl
resource "anthropic_workspace" "ci" {
  name = "ci"
}

# The identity the pipeline acts as. It holds no key of its own.
resource "anthropic_service_account" "ci" {
  name        = "ci-deploy"
  description = "Assumed by GitHub Actions through workload identity federation"
}

resource "anthropic_workspace_service_account" "ci" {
  workspace_id       = anthropic_workspace.ci.id
  service_account_id = anthropic_service_account.ci.id
  workspace_role     = "workspace_developer"
}

# Trust GitHub's OIDC issuer.
resource "anthropic_federation_issuer" "github" {
  name       = "github-actions"
  issuer_url = "https://token.actions.githubusercontent.com"
}
```

### The rule is the security boundary

```hcl
resource "anthropic_federation_rule" "ci_main" {
  name               = "ci-deploy-main"
  issuer_id          = anthropic_federation_issuer.github.id
  service_account_id = anthropic_service_account.ci.id
  oauth_scope        = "workspace:inference"
  workspace_id       = anthropic_workspace.ci.id

  token_lifetime_seconds = 900

  match = {
    subject_prefix = "repo:your-org/your-repo:ref:refs/heads/main"
  }
}
```

Read `match` carefully, because it decides who can assume the service account.

- `subject_prefix` is an **exact** match unless it ends in `*`, and it is
  case-sensitive. `repo:your-org/your-repo:*` trusts every branch, every tag and
  every pull request in that repository — including a branch a first-time
  contributor pushed.
- `audience` and `claims` are ANDed with it. A rule needs at least one of
  `subject_prefix`, `claims` or `condition`; one containing only `audience` is
  rejected, because it would accept every token the issuer ever signs.
- `condition` takes a CEL expression for anything the static matchers cannot
  express, such as matching several branches.

`oauth_scope` is a ceiling on what the minted token can do, and
`workspace:inference` is the narrower of the two. Use it unless the pipeline
genuinely manages files or skills.

`token_lifetime_seconds` defaults to an hour. A pipeline usually needs minutes.

## Exchanging the token

```hcl
variable "github_oidc_token" {
  description = "The OIDC token GitHub minted for this job."
  type        = string
  sensitive   = true
}

data "anthropic_organization" "current" {}

ephemeral "anthropic_federation_token" "ci" {
  federation_rule_id = anthropic_federation_rule.ci_main.id
  organization_id    = data.anthropic_organization.current.id
  service_account_id = anthropic_service_account.ci.id
  assertion          = var.github_oidc_token
}

provider "anthropic" {
  alias       = "federated"
  oauth_token = ephemeral.anthropic_federation_token.ci.access_token
}
```

Ephemeral resources are never written to state or to the plan, which is the
reason to model a minted credential as one: it exists for the operation that
used it and nowhere else.

The exchange itself needs **no provider credential**. The signed assertion is
what authenticates it, so a configuration whose only use of the provider is
minting a token can declare `provider "anthropic" {}` with nothing set.

### In the workflow

```yaml
- name: Mint an OIDC token
  id: oidc
  uses: actions/github-script@v7
  with:
    script: |
      core.setSecret(await core.getIDToken("anthropic"))
      core.setOutput("token", await core.getIDToken("anthropic"))

- name: Terraform apply
  env:
    TF_VAR_github_oidc_token: ${{ steps.oidc.outputs.token }}
  run: terraform apply -auto-approve
```

The audience passed to `getIDToken` lands in the token's `aud` claim, so it must
match the rule's `audience` when the rule sets one.

## What can go wrong

**Every denial returns the same opaque `401` with the message
`Authentication failed`**, whichever check failed. This is deliberate: a
distinguishable error would let a caller probe the rule configuration. The
actual reason is recorded against the attempt in the Claude Console under
**Workload identity**, on the authentication history tab. Start there rather
than guessing.

Common reasons:

| Reason | What happened |
|---|---|
| `match_subject_prefix` | The `sub` claim did not match. It is case-sensitive, and a prefix match needs a trailing `*` |
| `workspace_id_required` | The rule is enabled for more than one workspace and the exchange named none. Set `workspace_id` |
| `jti_reused` | The same assertion was exchanged twice. Mint a fresh one per exchange |

**A 403 after a successful exchange** is a scope problem, not an authentication
one: the token is valid but the rule's `oauth_scope` does not cover that
endpoint.

**The `iss` claim must equal `issuer_url` byte for byte**, including any
trailing slash.

## Token lifetime

The minted token lasts `token_lifetime_seconds` and the resource does **not**
renew it. Renewing would mean exchanging again, and an assertion carrying a
`jti` claim is single-use, so replaying the same JWT is rejected. A run long
enough to outlive its token needs a longer `token_lifetime_seconds` on the rule,
not a renewal.

`expires_at` is exported if a configuration needs to reason about the deadline.

## What this replaces

| | Long-lived API key | Federated token |
|---|---|---|
| Stored in CI | yes, forever | nothing |
| Lifetime | until revoked | minutes |
| Usable from elsewhere | yes | only from a workflow matching the rule |
| Scope | whatever the key has | the rule's `oauth_scope`, in one workspace |
| Rotation | manual | every run |
