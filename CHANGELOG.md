## Unreleased

SECURITY:

* Refuse HTTP redirects instead of following them. Go's HTTP client strips only
  `Authorization` and `Cookie` when a redirect changes host, so a redirect from
  the configured `base_url` would have replayed `X-Api-Key` (five of the six
  credential classes) on whatever host the redirect named. A 3xx now surfaces
  as an API error
* Validate `base_url`: the scheme must be `https`, with `http` accepted only for
  loopback addresses so the bundled mock server keeps working, and a URL with
  embedded credentials is rejected. The error text never repeats the value.
  Previously any parseable URL was accepted, including `http://` to a remote
  host, which sent the Admin key in cleartext
* Stop echoing non-JSON error bodies into diagnostics. A 5xx page from a proxy
  or gateway in front of the API is now logged at debug level only; the API's
  own error envelope is still surfaced verbatim
* Mask every configured credential by value in provider logs. The previous
  field-key masking never matched a logged field and did not reach request
  contexts
* Require `https://` on every attribute that decides where a token or a signing
  key goes: `anthropic_federation_issuer.jwks.url` and `jwks.discovery_base`
  (where JWT signing keys are fetched from), `anthropic_vault_credential`
  `static_bearer.mcp_server_url`, `mcp_oauth.mcp_server_url` and
  `mcp_oauth.refresh.token_endpoint` (where the bearer, access and refresh
  tokens are sent), and `anthropic_deployment.github_repositories[].url` (where
  the clone token is sent). `issuer_url` already had the check; the siblings
  did not, so a cleartext URL was accepted and the platform would have
  delivered the secret to it
* `anthropic_skill` no longer follows symlinks inside `source_dir`. A link to a
  file outside the skill directory would have been read and uploaded under the
  link's name. The file-count and size limits are now enforced during the walk
  rather than after the whole tree is in memory
* Build releases from the reviewed module graph. The goreleaser `before` hook
  ran `go mod tidy`, which could rewrite `go.mod` and `go.sum` inside the
  release job so the signed binaries compiled a different dependency set than
  the one in the tagged commit. It now runs `go mod download` and
  `go mod verify`, which fail instead
* Attach SLSA build provenance to every release archive and to the checksum
  file (`actions/attest-build-provenance`), and an SPDX SBOM per archive.
  Verify a download with
  `gh attestation verify <file> --repo uttej-badwane/terraform-provider-anthropic-enterprise`
* Run the release job in a `release` environment so a protection rule can gate
  access to the signing key, drop the checkout token from `.git/config` before
  third-party steps run, and scope `contents: write` to the one job that needs it
* Enable `gosec` and `bodyclose` in the linter, add a CodeQL workflow and an
  OpenSSF Scorecard workflow, and pin `golangci-lint` and `govulncheck` to
  exact versions instead of `latest`
* Ignore `*.tfstate` and `*.tfvars` everywhere. The previous `./*.tfstate`
  pattern never matched anything, so a state file, and with it any key an
  example configuration had read, could be committed

ENHANCEMENTS:

* Import `archive_on_destroy` and `delete_on_destroy` as `false` on every
  resource that archives or deletes a real object (workspace, service account,
  federation issuer and rule, agent, environment, vault, vault credential,
  deployment, memory store, skill). Previously an imported production
  workspace inherited `true`, and removing it from configuration archived it
  and every API key scoped to it. The first plan after import now shows the
  flag moving to its default so the operator decides; `anthropic_api_key` and
  `anthropic_user` already imported the safe value
* Warn in the plan when `anthropic_compliance_settings` is about to move to
  `disabled`, since that stops audit-log export for the whole organization

BUG FIXES:

* Cap `Retry-After` at the 20 second retry ceiling. The default backoff honoured
  the header unbounded, so a `429` or `503` carrying a large value parked the
  apply for as long as the server asked
* Do not replay a create after a `5xx` or a mid-flight transport error. The
  server may have committed the write before failing, and a replay created a
  second object that Terraform never learned about. Creates are still retried
  on `429` and when the connection could not be opened at all; updates,
  archives and reads keep the previous retry behaviour
* Bound pagination at 1000 pages and stop when the server returns the same
  cursor twice, instead of looping and accumulating memory forever
* Close the response body when a request fails after a response arrived, and
  report a body over 16 MiB as such rather than as a JSON decode error

DOCUMENTATION:

* `SECURITY.md`: replace the stale `0.1.x` support table with a "latest 0.x
  minor" policy, give the disclosure process a concrete timeline (3 business
  days to acknowledge, 7 to assess, 90-day disclosure window), describe where
  credentials are allowed to travel, and explain how to verify a release with
  `gh attestation verify`
* README: add a Security section, a Governance section pointing at
  `ROADMAP.md`, CodeQL and OpenSSF Scorecard badges, and the explicit
  `registry.terraform.io/` source OpenTofu users need until the provider is
  listed in the OpenTofu registry
* Add `ROADMAP.md` (what is next, what is later, what is not planned) and
  `AGENTS.md` for coding agents that do not read `CLAUDE.md`
* Correct the OpenTofu floor in the getting-started guide from 1.9 to 1.11,
  matching the README and the CI matrix

CHORE:

* Require Go 1.27.1, the current stable release, in both modules. The previous
  floor of 1.25.8 was on a line Go no longer patches and pinned the acceptance
  matrix, via `go-version-file`, to a toolchain with known standard library
  vulnerabilities. The directive now tracks the latest stable release; with no
  downstream users yet there is nobody to hold it back
* Record the conventions and the traps that have actually cost time in
  `CONTRIBUTING.md` — generated docs versus templates, error-expecting tests,
  diagnostic line wrapping in `ExpectError` patterns, required checks versus
  path-filtered workflows, and that `good first issue` items are reserved for
  contributors
* Add a short `CLAUDE.md` orienting coding agents in the repository layout and
  pointing them at `CONTRIBUTING.md` for the rules
* Add a Contributing section to the README pointing at the open `good first
  issue` and `help wanted` lists, with badges that track their counts, and say
  plainly that no Anthropic account is needed to contribute
* Stop path-filtering the test workflow. `paths-ignore: README.md` meant a
  README-only pull request triggered no workflows, so the required status checks
  never reported and branch protection blocked the pull request indefinitely
* The mock server documentation in `CONTRIBUTING.md` is now the version
  contributed in [#14](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/pull/14),
  which was opened before the equivalent in-house change was merged. Thanks to
  @Rayan-and-beyond
* Run the client and mock unit tests in CI. The acceptance matrix only covered
  `./internal/provider`, so `make test` and CI disagreed about what was tested
* Cancel a superseded run of the test workflow and run on pushes to `main`
  only, instead of once for the branch push and once for the pull request
* Add `make tools` (installs the pinned linters) and `make vulncheck`
* Bring the `tools/` module's `golang.org/x/*` dependencies level with the
  provider's

## v0.3.0 (2026-09-16)

ENHANCEMENTS:

* Detect a credential of the wrong class at configure time and name the
  attribute and what it expects, instead of letting the API answer with a bare
  `401 API key is invalid` that says nothing about which attribute is at fault.
  Only the two unambiguous swaps are rejected — an Admin key in `api_key` and a
  regular API key in `admin_api_key` — because refusing a key that would have
  worked is worse than letting the API decide ([#7](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/7))

BUG FIXES:

* Correct the documented OpenTofu floor from 1.9 to 1.11. Releases before 1.11
  reject the write-only attributes that `anthropic_vault_credential` and
  `anthropic_deployment` rely on, so the previous claim never held for a
  configuration that used them ([#6](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/6))

CHORE:

* Document the standalone mock server in `CONTRIBUTING.md`. It runs the
  acceptance suite's mock as a real HTTP server, so the provider can be driven
  end to end with no Anthropic credentials ([#8](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/8))
* Run the acceptance suite against OpenTofu in CI, on the 1.11 floor and on a
  current release. The suite already supported it through
  `TF_ACC_PROVIDER_HOST` and `TF_ACC_TERRAFORM_PATH`; nothing exercised it, which
  is how the inaccurate floor above went unnoticed ([#6](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/6))

DOCUMENTATION:

* **New Guide:** Managed Agents — builds an environment, vault, credential,
  skill, agent and scheduled deployment, and explains the behaviours that
  differ from the rest of the provider: agent versioning and how deployments
  pin to it, why `tools` is compared as a JSON subset, write-only vault secrets
  and `secret_version` rotation, local content hashing for skills, and which
  objects archive rather than delete ([#9](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/9))


## v0.2.0 (2026-09-16)

DOCUMENTATION:

* Group the generated resource and data source pages into registry
  subcategories (Console Organization, Workspaces, Service Accounts and
  Federation, Claude Enterprise, Compliance, Usage and Analytics, Managed
  Agents, Skills), so the registry sidebar is navigable rather than one flat
  list of ninety pages
* **New Guide:** Getting started
* **New Guide:** Choosing credentials — which credential class reaches which
  resource, how to obtain each, and how to read the resulting errors
* **New Guide:** Adopting an existing organization — `import` blocks,
  `-generate-config-out`, the import id format for every resource, and the
  order to adopt them in
* **New Example:** `examples/complete` — a workspace, a member, and a service
  account assumed by GitHub Actions through workload identity federation
* Add registry, CI and licence badges and guide links to the README

SECURITY:

* Update `google.golang.org/grpc` to 1.83.2, `golang.org/x/net` to 0.56.0 and
  `golang.org/x/text` to 0.39.0, clearing five vulnerabilities that govulncheck
  reported as reachable from provider code (GO-2026-6443, GO-2026-6348,
  GO-2026-6061, GO-2026-5970, GO-2026-5026). All are transitive dependencies of
  the plugin framework; the provider's own behaviour is unchanged
* Run `govulncheck` in CI, so a dependency whose vulnerable code path is
  reachable fails the build rather than waiting to be noticed
* Build releases with the current patched Go toolchain instead of the module's
  minimum. v0.1.0 was built on go1.25.8 and shipped the standard library
  vulnerabilities fixed in go1.25.9 and later, in `crypto/tls`, `crypto/x509`,
  `html/template`, `encoding/asn1`, `mime` and `archive/tar`; a binary scan of
  the v0.1.0 release reported 37 findings. A binary built from this change
  reports none
* Add Dependabot for both Go modules and for workflow actions, grouping minor
  and patch bumps into one pull request per ecosystem

CHORE:

* Add issue and pull request templates, `SECURITY.md` and `CODE_OF_CONDUCT.md`
* Document the pull request workflow and where to start in `CONTRIBUTING.md`

## v0.1.0 (2026-09-15)

FEATURES:

* **New Provider:** `uttej-badwane/anthropic-enterprise` (resource prefix `anthropic_`, declared with local name `anthropic`) with credential classes (`admin_api_key`, `oauth_token`, `enterprise_api_key`) and an in-process mock of the Admin API for acceptance tests
* **New Resource:** `anthropic_workspace`
* **New Resource:** `anthropic_workspace_member`
* **New Resource:** `anthropic_api_key` (import-only; manages name and status)
* **New Resource:** `anthropic_invite`
* **New Resource:** `anthropic_user` (import-only; manages role)
* **New Resource:** `anthropic_service_account`
* **New Resource:** `anthropic_workspace_service_account`
* **New Resource:** `anthropic_federation_issuer`
* **New Resource:** `anthropic_federation_rule`
* **New Resource:** `anthropic_federation_rule_workspace`
* **New Resource:** `anthropic_external_key`
* **New Resource:** `anthropic_rbac_group`
* **New Resource:** `anthropic_rbac_group_member`
* **New Resource:** `anthropic_spend_limit`
* **New Resource:** `anthropic_compliance_settings` (singleton toggle for the Compliance API)
* **New Data Source:** `anthropic_organization`, `anthropic_user`, `anthropic_users`, `anthropic_invite`, `anthropic_invites`, `anthropic_workspace`, `anthropic_workspaces`, `anthropic_workspace_member`, `anthropic_workspace_members`, `anthropic_api_key`, `anthropic_api_keys`, `anthropic_rate_limits`, `anthropic_workspace_rate_limits`, `anthropic_external_key`, `anthropic_external_keys`, `anthropic_compliance_settings`
* **New Data Source:** `anthropic_service_account`, `anthropic_service_accounts`, `anthropic_workspace_service_accounts`, `anthropic_federation_issuer`, `anthropic_federation_issuers`, `anthropic_federation_rule`, `anthropic_federation_rules`, `anthropic_federation_rule_workspaces`
* **New Data Source:** `anthropic_rbac_group`, `anthropic_rbac_groups`, `anthropic_rbac_group_members`, `anthropic_rbac_role`, `anthropic_rbac_roles`, `anthropic_spend_limit`, `anthropic_spend_limits`, `anthropic_spend_limit_increase_request`, `anthropic_spend_limit_increase_requests`
* **New Data Source:** `anthropic_usage_report`, `anthropic_cost_report`, `anthropic_claude_code_usage_report`, `anthropic_analytics_summaries`
* **New Data Source:** `anthropic_compliance_organizations`, `anthropic_compliance_organization_users`, `anthropic_compliance_roles`, `anthropic_compliance_role`, `anthropic_compliance_groups`, `anthropic_compliance_group_members`, `anthropic_compliance_effective_settings`
* Provider attributes `compliance_api_key` and `analytics_api_key` (both fall back to `enterprise_api_key`)
* **New Data Source:** `anthropic_analytics_users`, `anthropic_analytics_skills`, `anthropic_analytics_connectors`, `anthropic_analytics_plugins`, `anthropic_analytics_artifacts`, `anthropic_analytics_usage_report`, `anthropic_analytics_cost_report`, `anthropic_analytics_user_usage_report`, `anthropic_analytics_user_cost_report`
* Provider attributes `api_key` and `workspace_id` for the Managed Agents control plane and Skills API (beta)
* **New Resource:** `anthropic_agent`, `anthropic_environment`, `anthropic_vault`, `anthropic_vault_credential`, `anthropic_deployment`, `anthropic_memory_store`, `anthropic_skill`
* **New Data Source:** `anthropic_agent`, `anthropic_agents`, `anthropic_agent_versions`, `anthropic_environment`, `anthropic_environments`, `anthropic_vault`, `anthropic_vaults`, `anthropic_vault_credentials`, `anthropic_deployment`, `anthropic_deployments`, `anthropic_memory_store`, `anthropic_memory_stores`, `anthropic_skill`, `anthropic_skills`, `anthropic_skill_versions`

NOTES:

* Workspaces, service accounts, federation issuers and federation rules cannot be deleted through the Admin API; destroy archives them unless `archive_on_destroy = false`.
* Workspace membership reads use the list endpoint because the per-member GET is served from a cache that updates do not invalidate for tens of seconds; membership writes retry while a fresh membership is not yet visible and wait for the new role to read back.
* `anthropic_federation_rule` re-reads the rule after create and update because the API's write responses omit the read-time fields `issuer_name` and `service_account_name`.
* Verified against a live Console organization: organization, users, workspaces, API keys and rate-limit data sources, the `anthropic_workspace`, `anthropic_workspace_member`, `anthropic_service_account`, `anthropic_workspace_service_account`, `anthropic_federation_issuer` and `anthropic_federation_rule` lifecycles, and the full Managed Agents graph (environment, memory store, vault, vault credential, skill, agent, deployment) including an empty re-plan and imports.
* Enterprise (`rbac_*`, `spend_limit`, compliance, analytics) resources and `anthropic_invite` are verified against the mock only until those credentials and a disposable mailbox are available.
* Vault credential secrets use write-only attributes and require Terraform 1.11 or newer.
* `anthropic_analytics_chat_projects` is not included: the endpoint's reference page was unavailable when the schema was captured.
