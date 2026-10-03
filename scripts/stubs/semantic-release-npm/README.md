# Stand-in for `@semantic-release/npm`

`package.json` overrides `@semantic-release/npm` with this directory.

semantic-release depends on `@semantic-release/npm` only because it is one of
its default plugins. `.releaserc.json` replaces the defaults with
`commit-analyzer`, `release-notes-generator` and `exec`, and this repository
never publishes to npm, so the real plugin is never loaded. Release logs confirm
it: the plugin is never mentioned.

It was still being installed, and it brings the entire npm CLI with it, about
140 bundled packages. npm `overrides` cannot reach bundled packages, so an
advisory against any one of them could only be fixed by waiting for a new npm
release. That happened twice: nineteen advisories in September, then
`ip-address` (GHSA-2vr4-cq9g-pvrc), for which no npm release carried a fix.

The stub removes all of it. If the npm plugin is ever configured, this package
throws immediately with a reason instead of appearing to work.

To use the real plugin, delete the `overrides` entry in `package.json`, delete
this directory, and run `npm install`.
