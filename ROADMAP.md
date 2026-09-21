# Roadmap

What is planned, roughly in order. Issues carry the detail; this page is the
map. Anything here is open to contributors unless it says otherwise, and the
`good first issue` items are reserved for people who have not contributed yet.

## Next

- **Provider-level tuning.** Expose the request timeout and retry count
  ([#18](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/18))
  and `timeouts` blocks on the resources that wait for eventual consistency
  ([#19](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/19)).
- **Test coverage.** Unit tests for the eventual-consistency helpers
  ([#21](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/21))
  and an acceptance test for `anthropic_external_key` updates
  ([#20](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/20)).

## Later

- **Ephemeral federation token.** An `ephemeral` resource that exchanges a
  workload identity token for a short-lived Anthropic credential, so a
  pipeline can call the API without a stored key
  ([#22](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/issues/22)).
- **Write-tier live suite in CI.** The `make testacc-live-write` tier runs
  against a development organization by hand today. Running it on a schedule
  needs a dedicated organization and a credential in a gated environment.
- **A second maintainer.** Review and release rights for a regular contributor,
  so the project does not depend on one person.

## Not planned

Anything the Anthropic APIs expose no write endpoint for: rate-limit
overrides, organization spend caps, custom roles, SCIM groups, SSO, IP
allowlists, retention, invite domains, and the creation of Admin, Compliance
or Analytics keys. The README lists these and why. Open an issue with the
endpoint if one appears.

## How to influence this

Comment on an issue, or open one with the use case first and the HCL you
would like to write second. Documentation fixes need no discussion; open the
pull request.
