# Security policy

## Supported versions

The most recent minor release receives security fixes. The provider is below
1.0, so patches land on the current line rather than being backported.

| Version | Supported |
|---|---|
| 0.1.x | Yes |
| < 0.1 | No |

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

You can expect an acknowledgement within a few days. If a fix is warranted, it
ships in a patch release with an advisory crediting you, unless you prefer
otherwise.

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
