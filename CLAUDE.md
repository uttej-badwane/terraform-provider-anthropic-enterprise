# Working in this repository

A Terraform provider for managing Anthropic organizations, built on the
Terraform Plugin Framework (protocol 6).

**Read [CONTRIBUTING.md](./CONTRIBUTING.md) first.** Its *Conventions* and
*Things that catch people out* sections hold the rules that matter, and several
of them are not inferable from the code. This file only adds orientation.

## Layout

| Path | What it holds |
|---|---|
| `internal/client/` | Hand-written HTTP client. Credential routing, retries, pagination |
| `internal/provider/` | One file per resource and data source, plus `helpers_*.go` |
| `internal/mock/` | In-memory implementation of every API, backing the whole test suite |
| `templates/` | Source for generated docs. Guides live in `templates/guides/` |
| `docs/` | **Generated output. Do not edit by hand** |
| `examples/` | Per-resource examples, also rendered into the docs |

## Commands

```sh
make build              # compile
make test               # unit tests (client + mock)
make testacc            # acceptance suite against the in-process mock
make lint               # golangci-lint
make generate           # regenerate docs/ from schemas and examples
make docs-validate      # check the generated docs
```

`make testacc` needs no credentials — the mock backs everything. Run it, plus
`make lint` and `make generate`, before proposing any change.

## Credentials

The provider spans several APIs that do not share an auth scheme, so there is
one attribute per credential class and requests are routed to the one their
endpoint accepts. The
[credentials guide](./templates/guides/credentials.md) explains which reaches
what. A key of the wrong class is rejected at configure time.

Live tests are opt-in and point at a development organization only. Several
objects archive irreversibly on destroy, so never aim them at an organization
you are not prepared to lose objects in.

## Before proposing a change

1. `make test testacc lint` all pass.
2. `make generate` has been run and any `docs/` changes are committed.
3. The pull request title is a Conventional Commit (`fix(scope): ...`).
   It becomes the commit subject on merge and decides the next version.
   Do not edit `CHANGELOG.md`; release notes are generated.
4. No credentials, organization ids, user ids, email addresses or workspace
   names appear in the diff. Examples use placeholders only.
5. The change goes up as a pull request. `main` is protected and requires
   passing checks.

## Two things worth repeating

**`docs/` is generated and regeneration deletes anything it does not own.** A
file written directly under `docs/guides/` will vanish. Guides go in
`templates/guides/`.

**Issues labelled `good first issue` are reserved for contributors.** Do not
implement one without claiming it in the issue first.
