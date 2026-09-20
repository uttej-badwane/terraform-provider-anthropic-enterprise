# Contributing

## Proposing a change

Changes land through pull requests, including documentation-only ones.

```sh
git checkout -b short-descriptive-branch-name
# make the change
make test testacc lint generate
git commit
gh pr create --fill
```

Before opening the pull request:

* `make generate` has been run and any resulting `docs/` changes are committed.
  Documentation is generated from the schemas and examples; editing `docs/` by
  hand is undone by the next generation. Static pages such as guides live in
  `templates/guides/`, because tfplugindocs treats `docs/` as output and
  deletes anything it does not own.
* The pull request title is a Conventional Commit, because it becomes the
  commit subject and decides the next version.
* No credentials, organization ids, user ids, email addresses or workspace
  names appear anywhere in the diff. Examples use placeholders
  (`wrkspc_...`, `user_...`, `example.com`).

The pull request template lists the same checks. CI runs the build, the unit tests, the
linter, a vulnerability scan, CodeQL, documentation generation and the acceptance
suite against the mock on three Terraform and two OpenTofu versions; all of it
must pass before merge.

## Where to start

* Issues labelled **good first issue** are scoped to one resource or one
  document and need no access to a real organization.
* Documentation is the easiest place to help. The guides in `templates/guides/`
  and the `MarkdownDescription` on any attribute are fair game, and the mock
  makes it possible to verify an example without an Anthropic account.
* Adding a resource or data source? Open an issue first with the API endpoint
  that backs it. Several organization features are read-only in the API and
  cannot be managed here; the README lists what is deliberately not covered.

## Development

The `go` directive in `go.mod` tracks the latest stable Go release and is bumped
when Go ships a new one; there are no users on an older toolchain to hold it
back. `make tools` installs the linters at the versions CI runs.

The local checkout can live in any directory. Only the GitHub repository name matters: the Terraform Registry requires it to be `terraform-provider-anthropic-enterprise`, and the Go module path and goreleaser `project_name` already reflect that. Resource types keep the `anthropic_` prefix, so configurations declare the provider with the local name `anthropic`.

```sh
make tools        # install golangci-lint and govulncheck at the versions CI uses
make build        # compile
make test         # unit tests (client + mock)
make testacc      # acceptance tests against the in-process mock Admin API
make generate     # regenerate docs/ from schemas + examples/
make lint         # golangci-lint
make vulncheck    # govulncheck, the same scan the pull request runs
```

Run the provider from source against a Terraform configuration:

```sh
make install
eval "$(make -s dev-override)"      # exports TF_CLI_CONFIG_FILE=dev.tfrc
cd examples/resources/anthropic_workspace
terraform plan                      # no `terraform init` with dev_overrides
```

Run the standalone mock Admin API for local development:

```sh
go run ./internal/mock/cmd/mockserver -addr 127.0.0.1:8787
```

The mock server prints the local endpoint and the environment variables needed by the provider. Set the printed `ANTHROPIC_BASE_URL` and test credentials in your shell before running provider commands against the mock.

## Live acceptance tests

`make testacc-live` runs the `TestAccLive*` subset against a real organization. It only reads data sources and creates, renames and archives objects whose names start with `tf-acc-`. Export `ANTHROPIC_ADMIN_API_KEY` (and optionally `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_ENTERPRISE_API_KEY`) in the shell before running it. Never point it at an organization you are not allowed to modify.

### Managed Agents live tests

When `ANTHROPIC_API_KEY` (a regular workspace key) is also exported, `make testacc-live` runs the `TestAccLiveAgents*` tests: they create and destroy a `tf-acc-` environment, memory store, vault, credential, skill, agent and a paused, unscheduled deployment with a budget cap. No session is ever started, so no inference credits are consumed.

### Write-tier live tests (development organizations only)

`make testacc-live-write` additionally creates and destroys invites, workspace members, service accounts and federation objects. Point it only at a development organization. It reads `ANTHROPIC_ACC_INVITE_EMAIL` (an address you control) and `ANTHROPIC_ACC_MEMBER_USER_ID` (a non-admin member id) for the tests that need them, and needs `ANTHROPIC_AUTH_TOKEN` for the federation lifecycle.

## Cleaning up after a live run

The live tiers create objects named `tf-acc-<random>`. A run that fails partway
leaves them behind, and several of them archive rather than delete, so a
development organization accumulates permanent records.

```sh
go test ./internal/provider/ -sweep=all -timeout 30m
```

Sweeping needs the same credentials the live tiers do and acts on a real
organization. Only names beginning with `tf-acc-` are touched; that prefix is
the entire safety boundary, so never widen it to tidy up something else.

## Conventions

* Terraform Plugin Framework only. `terraform-plugin-sdk/v2` imports are rejected by the linter.
* Every attribute has a `MarkdownDescription`. Docs are generated; do not edit `docs/` by hand.
* Every resource supports `terraform import`; composite ids use `parent_id/child_id`.
* Archive-only API objects (workspaces, service accounts, federation issuers and rules) say so in their description and remove themselves from state when archived out of band.
* Examples use placeholder identifiers (`wrkspc_...`, `user_...`, `example.com`) only.
* Title the pull request as a [Conventional Commit](https://www.conventionalcommits.org). Merges are squashes, so the title becomes the commit subject, and that subject is what decides whether a release is cut and what the version is. `feat` gives a minor, `fix` and `perf` a patch, and `docs`, `test`, `refactor`, `chore` and `ci` release nothing on their own. CI rejects a title that does not parse. See [RELEASING.md](./RELEASING.md).
* Do not edit `CHANGELOG.md`. Entries through `v0.5.0` are kept as history; newer releases are described by their generated release notes.

## Things that catch people out

Each of these has cost someone real time. They are not obvious from reading the code.

* **`docs/` is generated output and tfplugindocs deletes anything it does not own.** Static pages such as guides belong in `templates/guides/`; a file created directly under `docs/guides/` disappears on the next `make generate`. Run `make generate` and commit the rendered result alongside the template.
* **Subcategories are applied after generation.** `scripts/set-subcategories.sh` maps every page to a registry sidebar section, and an unmapped page is a hard error rather than a silent fallback. A new resource or data source needs an entry there.
* **A test step that expects an error must be its own test function.** The post-test destroy re-runs the last configuration, so grouping an error case with a success case replays the wrong one.
* **`ExpectError` patterns must tolerate line wrapping.** Terraform wraps diagnostic text at a width that depends on the surrounding message, so a literal space between two words may become a newline. Match `\s+` instead. This broke CI on Linux while passing on macOS, because the temp directory path lengths differ.
* **An Optional and Computed `SingleNestedAttribute` needs every nested attribute Optional and Computed**, or Terraform core reports a perpetual diff.
* **Never use `pull_request_target` in a workflow.** Fork pull requests currently run with a read-only token and no access to secrets, which is what makes it safe to run CI on them automatically. `pull_request_target` would hand fork code the repository's secrets.
* **Do not path-filter a workflow whose checks are required by branch protection.** A pull request that matches the filter triggers no workflow, the required check never reports, and the pull request is blocked indefinitely.
* **Squash merges mean `git branch -d` will not recognise a merged branch.** Confirm the content is on `main` before reaching for `-D`.
* **Issues labelled `good first issue` are reserved for contributors.** Do not implement one without claiming it in the issue first. This has already cost an outside contributor their work.
