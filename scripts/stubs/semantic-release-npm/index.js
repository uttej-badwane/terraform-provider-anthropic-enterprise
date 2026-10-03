// Stand-in for @semantic-release/npm. See README.md in this directory.
//
// semantic-release depends on @semantic-release/npm only because it is one of
// its default plugins. This repository replaces the defaults in .releaserc.json
// and never publishes to npm, so the real plugin is never loaded. It would
// still be installed, along with the whole npm CLI it bundles, and every
// advisory against any of those bundled packages would land in this
// repository's security alerts for code that never runs.
//
// If someone does configure the npm plugin, fail at once with a reason rather
// than appearing to work and publishing nothing.
const explain = () => {
  throw new Error(
    "@semantic-release/npm is replaced by a stub in this repository (see package.json \"overrides\" " +
      "and scripts/stubs/semantic-release-npm/README.md). Remove the override before configuring the npm plugin.",
  );
};

export const verifyConditions = explain;
export const prepare = explain;
export const publish = explain;
export const addChannel = explain;
