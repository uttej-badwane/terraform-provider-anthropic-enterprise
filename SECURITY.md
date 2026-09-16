# Security policy

## Supported versions

The most recent minor release receives security fixes. The provider is below
1.0, so patches land on the current line rather than being backported. Check
the [releases page](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/releases)
for the current line.

| Version | Supported |
|---|---|
| Latest 0.x minor | Yes |
| Earlier 0.x | No, upgrade to the latest minor |

## Reporting a vulnerability

**Please do not open a public issue for a security problem.**

Report it through GitHub's private vulnerability reporting, on the
[Security tab](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/security/advisories/new)
of this repository. That channel is private to the maintainers and lets us
prepare a fix and an advisory before anything is disclosed.

Please include:

- The provider version and the affected resource or data source.
- What an attacker can do, and what access they need to do it.
- Steps to reproduce, with any credentials or organization identifiers removed.

What to expect:

- An acknowledgement within 3 business days.
- A severity assessment and a plan within 7 days of the acknowledgement.
- A fix in a patch release, with a GitHub Security Advisory crediting you
  unless you prefer otherwise. Please hold public disclosure until the release
  is out or 90 days have passed, whichever comes first.

**Do not include real credentials, organization ids, user ids or email
addresses in a report.** If a credential has been exposed, revoke it first —
note that archiving an Anthropic API key is irreversible, so revoking means
creating a replacement.

## Scope

In scope:

- The provider writing secret material to Terraform state or to logs.
- A credential being sent to an endpoint of a different credential class.
- Sending requests to a host other than the configured API base URL.
- Verification being skipped where the provider claims to verify.

Out of scope:

- Vulnerabilities in the Anthropic APIs themselves. Report those to Anthropic.
- Vulnerabilities in Terraform, OpenTofu or Go. Report those upstream.
- A configuration granting more access than intended — for example a
  `subject_prefix` on a federation rule that is broader than the author meant.
  Tell us if the documentation encouraged it and we will fix the documentation.

## Handling of secrets

The provider does not write secret material to state. API keys cannot be
created through the Anthropic API, so no key material passes through it, and
vault credential secrets use write-only attributes that are sent to the API and
never persisted. If you find a path where a secret reaches state, a log or an
error message, that is a vulnerability and we want to hear about it.

Where credentials travel is constrained on purpose:

- Every request goes to `base_url`, which must be `https` (plain `http` only
  for loopback addresses, so the bundled mock works) and may not carry
  embedded credentials.
- HTTP redirects are refused, not followed, so a key is never replayed against
  a host the configuration did not name.
- Attributes that name a host a token or signing key is sent to or fetched
  from (`jwks.url`, `mcp_server_url`, `token_endpoint`, repository URLs)
  require `https`.
- Error bodies that are not the API's own error envelope are not copied into
  Terraform diagnostics.
- Configured credential values are masked by value in provider logs.

## Verifying a release

Release checksums are signed with the maintainer's GPG key, which the Terraform
Registry verifies on ingest. Every archive also carries SLSA build provenance
from GitHub Actions:

```sh
gh attestation verify terraform-provider-anthropic-enterprise_<version>_linux_amd64.zip \
  --repo uttej-badwane/terraform-provider-anthropic-enterprise
```

A successful verification proves the file was built by this repository's
release workflow from the tagged commit.
