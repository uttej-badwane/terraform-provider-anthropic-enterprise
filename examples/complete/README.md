# Complete example

A realistic organization setup: one workspace, a human member, and a service
account that GitHub Actions assumes through workload identity federation, so CI
holds no long-lived Anthropic API key.

## Credentials

Service accounts and federation are not reachable with an Admin API key. This
example needs an `org:admin` OAuth token:

```sh
ant --profile admin auth login --scope org:admin
export ANTHROPIC_AUTH_TOKEN="$(ant auth print-credentials --profile admin --access-token)"
```

See the [credentials guide](https://registry.terraform.io/providers/uttej-badwane/anthropic-enterprise/latest/docs/guides/credentials)
for the other credential classes.

## Run it

```sh
terraform init
terraform plan \
  -var 'github_repository=your-org/your-repo' \
  -var 'member_email=you@example.com'
```

Inspect the plan before applying. `member_email` must be a user who already
exists in the organization; invite them with `anthropic_invite` first if not.

## What it creates

| Resource | Purpose |
|---|---|
| `anthropic_workspace.production` | Isolates keys, members and rate limits |
| `anthropic_workspace_member.member` | Adds an existing user as a developer |
| `anthropic_service_account.ci` | The identity CI acts as |
| `anthropic_workspace_service_account.ci` | Grants that account access to the workspace |
| `anthropic_federation_issuer.github_actions` | Trusts GitHub's OIDC issuer |
| `anthropic_federation_rule.ci_main` | Allows only `refs/heads/main` of one repository to assume the account |

`subject_prefix` on the federation rule is the security boundary. It is scoped
to a single branch of a single repository here; widening it widens who can
assume the service account.

## Before you destroy

Workspaces, service accounts, federation issuers and federation rules **archive
rather than delete**, and archiving is irreversible. `terraform destroy` on this
example leaves archived records behind that cannot be restored. Run it against a
development organization.
