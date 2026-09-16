# Releasing

One-time setup:

1. Generate an RSA GPG key (ECC keys are not accepted by the Terraform Registry) and export the ASCII-armored public key.
2. Upload the public key at registry.terraform.io > User Settings > Signing Keys for the `uttej-badwane` namespace (provider `anthropic-enterprise`, repository `terraform-provider-anthropic-enterprise`).
3. Add `GPG_PRIVATE_KEY` (ASCII-armored private key) and `PASSPHRASE` as repository secrets.
4. Publish the provider once through registry.terraform.io > Publish > Provider; the registry installs a release webhook.

Each release:

1. Move `## Unreleased` in `CHANGELOG.md` to `## vX.Y.Z (YYYY-MM-DD)`.
2. Run `make generate` and commit any doc changes.
3. `git tag vX.Y.Z && git push origin vX.Y.Z`.
4. The `Release` workflow runs goreleaser, signs `SHA256SUMS`, and creates a draft GitHub release. Review the assets, then publish the release. The registry ingests it within a few minutes; use "Resync" in the provider settings if it does not.

Local dry run:

```sh
make snapshot      # goreleaser --snapshot, unsigned, artifacts in dist/
```
