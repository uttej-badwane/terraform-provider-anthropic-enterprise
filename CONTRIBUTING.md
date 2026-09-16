# Contributing

## Development

The local checkout can live in any directory. Only the GitHub repository name matters: the Terraform Registry requires it to be `terraform-provider-anthropic-enterprise`, and the Go module path and goreleaser `project_name` already reflect that. Resource types keep the `anthropic_` prefix, so configurations declare the provider with the local name `anthropic`.

```sh
make build        # compile
make test         # unit tests (client + mock)
make testacc      # acceptance tests against the in-process mock Admin API
make generate     # regenerate docs/ from schemas + examples/
make lint         # golangci-lint
```

Run the provider from source against a Terraform configuration:

```sh
make install
eval "$(make -s dev-override)"      # exports TF_CLI_CONFIG_FILE=dev.tfrc
cd examples/resources/anthropic_workspace
terraform plan                      # no `terraform init` with dev_overrides
```

## Live acceptance tests

`make testacc-live` runs the `TestAccLive*` subset against a real organization. It only reads data sources and creates, renames and archives objects whose names start with `tf-acc-`. Export `ANTHROPIC_ADMIN_API_KEY` (and optionally `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_ENTERPRISE_API_KEY`) in the shell before running it. Never point it at an organization you are not allowed to modify.

### Managed Agents live tests

When `ANTHROPIC_API_KEY` (a regular workspace key) is also exported, `make testacc-live` runs the `TestAccLiveAgents*` tests: they create and destroy a `tf-acc-` environment, memory store, vault, credential, skill, agent and a paused, unscheduled deployment with a budget cap. No session is ever started, so no inference credits are consumed.

### Write-tier live tests (development organizations only)

`make testacc-live-write` additionally creates and destroys invites, workspace members, service accounts and federation objects. Point it only at a development organization. It reads `ANTHROPIC_ACC_INVITE_EMAIL` (an address you control) and `ANTHROPIC_ACC_MEMBER_USER_ID` (a non-admin member id) for the tests that need them, and needs `ANTHROPIC_AUTH_TOKEN` for the federation lifecycle.

## Conventions

* Terraform Plugin Framework only. `terraform-plugin-sdk/v2` imports are rejected by the linter.
* Every attribute has a `MarkdownDescription`. Docs are generated; do not edit `docs/` by hand.
* Every resource supports `terraform import`; composite ids use `parent_id/child_id`.
* Archive-only API objects (workspaces, service accounts, federation issuers and rules) say so in their description and remove themselves from state when archived out of band.
* Examples use placeholder identifiers (`wrkspc_...`, `user_...`, `example.com`) only.
* Add a `CHANGELOG.md` entry under `## Unreleased` for user-visible changes.
