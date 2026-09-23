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
5. The release is published, and the Terraform and OpenTofu registries pick it
   up (see below; they do not do it the same way).

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

## How the registries pick up a release

**The Terraform Registry** is told about each release by a webhook, so a new
version normally appears within a minute or two of being published.

**The OpenTofu registry** has no webhook. It scans the repository's tags every
15 minutes and downloads the assets for any version it has not seen.

That scan can land in the gap between the tag being pushed and the release being
published. semantic-release pushes the tag first, then the release sits as a
draft for about five minutes while the checks above run, and a draft's assets
cannot be downloaded. If OpenTofu scans in that window, the download fails and it
records the version as errored. With a 15-minute scan and a five-minute window,
roughly one release in three will hit this.

This is a delay, not a loss. OpenTofu retries an errored version with a doubling
backoff, starting at 30 minutes after the failure, and by then the release is
published. The backoff is only the minimum, though: the retry waits for the next
scan after it expires, GitHub runs scheduled workflows late, and a listed
version then waits for a deploy and a cache. v0.9.0 went like this:

| UTC | |
|---|---|
| 15:34 | scan fails, version recorded as errored |
| 16:04 | backoff expires |
| 16:27 | a scan retries it and it is listed in the registry metadata |
| 16:48 | the registry API serves it, 74 minutes after the error |

So:

- **A version missing from OpenTofu for up to two hours needs nothing done.**
  v0.9.0 was the first to hit this: OpenTofu scanned 28 seconds after its
  draft was created and five minutes before it was published, and it was
  served 74 minutes later without anyone intervening.
- **The error is visible** in `versions_errors` in
  [the provider's metadata file](https://github.com/opentofu/registry/blob/main/providers/u/uttej-badwane/anthropic-enterprise.json),
  typically as `checksums not found in release`.
- **Check what each registry actually serves** rather than the release page:

  ```sh
  curl -s https://registry.terraform.io/v1/providers/uttej-badwane/anthropic-enterprise/versions | jq -r '.versions[].version'
  curl -s https://registry.opentofu.org/v1/providers/uttej-badwane/anthropic-enterprise/versions | jq -r '.versions[].version'
  ```

- **Worth investigating** only if a version is still absent after several hours,
  or the recorded error is anything other than missing checksums.

Removing the window would mean verifying the artifacts before the tag becomes
public, which is a restructuring of the part of the pipeline that signs and
publishes releases. For a delay that clears itself on one registry, that
trade has not been worth making.

## Dependency updates

Dependabot's patch and minor updates merge themselves once every required check
passes (`.github/workflows/dependabot-auto-merge.yml`). Majors stop for a
person, because a major changes behaviour rather than fixing it.

Nothing about that bypasses review by machines: `gh pr merge --auto` only queues
the merge, and branch protection holds it until the build, the acceptance suite
across every supported runtime, the vulnerability scan and the release
configuration checks have all passed. A failing update is never merged.

Whether such a merge cuts a release depends on its prefix, and the prefixes are
assigned by what the dependency actually is:

| Ecosystem | Prefix | Releases |
|---|---|---|
| Go module at the root | `deps` | patch — it is in the shipped binary |
| Go module under `tools/` | `ci(tools)` | no |
| GitHub Actions | `ci(actions)` | no |
| npm release tooling | `ci(release-tooling)` | no |

Only the first reaches users. The rest build or publish the provider without
being part of it, and a new version of the provider whose binary is unchanged is
noise on a registry that cannot withdraw one.

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
- The provider listed in the OpenTofu registry
  ([opentofu/registry#5532](https://github.com/opentofu/registry/pull/5532)),
  with the same release key registered there
  ([#5533](https://github.com/opentofu/registry/pull/5533)) so OpenTofu verifies
  signatures rather than skipping them.

## CHANGELOG.md

Entries through `v0.5.0` were written by hand and are kept as history. Newer
releases are described by their generated release notes on the
[releases page](https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise/releases),
which are assembled from the commit subjects.
