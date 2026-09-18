# Releasing

Releases are cut from the commit history. There is no manual version bump, tag
or publish step.

## How a release happens

1. A pull request is merged to `main`. Because merges are squashes, the pull
   request **title** becomes the commit subject, and CI rejects a title that is
   not a [Conventional Commit](https://www.conventionalcommits.org).
2. `semantic-release` reads every commit since the last tag and decides whether
   a release is warranted and what the version is.
3. If one is, it creates the tag and runs `goreleaser`, which builds the
   fourteen platform archives, the checksum file and an SBOM per archive, and
   signs the checksums with the release GPG key.
4. The artifacts are verified before anything becomes installable (below).
5. The release is published and the Terraform Registry ingests it.

A push carrying no releasable commit is the normal case, not a failure.

## What decides the version

| Commit type | Release |
|---|---|
| `feat` | minor |
| `fix`, `perf`, `revert`, `build`, `deps` | patch |
| `docs`, `test`, `refactor`, `style`, `chore`, `ci` | none |
| `!` after the type, or a `BREAKING CHANGE:` footer | minor, while below 1.0 |

A type that does not release still appears in the release notes; it just does
not cut a version on its own.

A breaking change moves the **minor**, not the major, which is what `0.x`
already means under semver: below `1.0` the minor is the compatibility signal.
It also stops a stray `!` from declaring `1.0.0` by accident, which would be an
irreversible claim of API stability on a registry where a version cannot be
withdrawn.

Reaching `1.0.0` is therefore a deliberate act: change the `breaking` rule in
`.releaserc.json` to `major`, or push the `v1.0.0` tag by hand once and let the
automation continue from there.

## What is verified before publishing

`goreleaser` leaves the release as a draft. It is promoted only after:

- **The checksum file has no SBOM entries.** The registry parses that file to
  find a release's artifacts. v0.4.0 was built and signed correctly and the
  registry silently refused it, because fourteen `.sbom.json` lines had appeared
  there. SBOMs are still attached to the release; they are kept out of the file
  the registry reads.
- **The checksum signature verifies** against the release key.
- **The binary reports a current Go toolchain**, so a release is never built by
  a toolchain carrying known standard library vulnerabilities.
- **`govulncheck` finds nothing** in the built binary.

Any of these failing leaves the release a draft, so nothing unverified can be
installed.

## Forcing a release

`workflow_dispatch` on the Release workflow runs the same path. Use it if the
automation is unavailable; the artifacts are identical either way.

## One-time setup, already done

- An RSA GPG key (the registry rejects ECC), with the public half registered
  under the `uttej-badwane` namespace in HCP Terraform.
- `GPG_PRIVATE_KEY` and `PASSPHRASE` as repository secrets, held in the
  `release` environment so a protection rule can gate releases if one is ever
  added.
- The provider published once through the registry, which installs the webhook
  that ingests every later release.

## CHANGELOG.md

Entries through `v0.5.0` were written by hand and are kept as history. Newer
releases are described by their generated release notes on the
[releases page](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/releases),
which are assembled from the commit subjects.
