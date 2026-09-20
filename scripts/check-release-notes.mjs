// Renders release notes from .releaserc.json against a synthetic commit set.
//
// semantic-release only runs `generateNotes` when there is actually something
// to release. A push carrying no releasable commit is the normal case here, so
// the notes generator can go untouched for weeks and a broken configuration
// surfaces on the one push that does release — after the version has been
// decided, and in the job that publishes.
//
// That is not hypothetical. conventional-changelog-conventionalcommits@10
// requires conventional-changelog-writer@9 or newer, and
// @semantic-release/release-notes-generator@14 pins ^8.0.0. The two cannot
// coexist. Every plugin still *loaded*, a dry run still reported "no release",
// and the incompatibility appeared only when the preset was asked to render.
//
// Loading a plugin is not the same as exercising it, so this renders.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { generateNotes } from "@semantic-release/release-notes-generator";

const config = JSON.parse(readFileSync(".releaserc.json", "utf8"));
const plugin = config.plugins.find(
  (p) => Array.isArray(p) && p[0] === "@semantic-release/release-notes-generator",
);
assert.ok(plugin, "release-notes-generator is not configured in .releaserc.json");

const commit = (hash, message) => ({
  hash,
  message,
  subject: message.split("\n")[0],
  commit: { short: hash.slice(0, 7) },
  tree: { short: "tree" },
  committerDate: "2026-01-01",
});

// One commit per type the preset config names, so a section that stops
// rendering is caught rather than silently dropped.
const commits = [
  commit("1".repeat(40), "feat(provider): add a resource"),
  commit("2".repeat(40), "fix(client): retry on 429"),
  commit("3".repeat(40), "perf(client): reuse the transport"),
  commit("4".repeat(40), "revert: undo the thing"),
  commit("5".repeat(40), "build: adjust the makefile"),
  commit("6".repeat(40), "deps(actions): bump an action"),
  commit("7".repeat(40), "docs: tidy a guide"),
  commit("8".repeat(40), "test: cover the sweepers"),
  commit("9".repeat(40), "refactor: extract a helper"),
  commit("a".repeat(40), "ci: pin the tooling"),
  commit("b".repeat(40), "feat!: drop an attribute\n\nBREAKING CHANGE: it is gone"),
];

const notes = await generateNotes(plugin[1], {
  cwd: process.cwd(),
  options: { repositoryUrl: "https://github.com/example/example" },
  lastRelease: { gitTag: "v0.0.1", version: "0.0.1" },
  nextRelease: { gitTag: "v0.0.2", version: "0.0.2", type: "patch", channel: null },
  commits,
  logger: { log() {}, error() {} },
});

// Every visible section the preset config declares must appear. `chore` and
// `style` are configured hidden, so their absence is the correct behaviour.
const expected = [
  "Features",
  "Bug Fixes",
  "Performance",
  "Reverts",
  "Build",
  "Dependencies",
  "Documentation",
  "Tests",
  "Refactoring",
  "Continuous Integration",
  "BREAKING CHANGES",
];

const missing = expected.filter((section) => !notes.includes(section));
assert.deepEqual(missing, [], `release notes are missing section(s): ${missing.join(", ")}`);

console.log("release notes render; all expected sections present");
