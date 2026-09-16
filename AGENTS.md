# Working in this repository with a coding agent

This file exists so that coding agents which look for `AGENTS.md` find the
same guidance as those which read `CLAUDE.md`. The content is identical in
intent; the rules live in one place.

1. Read [CONTRIBUTING.md](./CONTRIBUTING.md) first. Its *Conventions* and
   *Things that catch people out* sections are the rules that matter, and
   several cannot be inferred from the code.
2. [CLAUDE.md](./CLAUDE.md) adds the repository layout, the `make` targets,
   and the checklist to run before proposing a change.
3. `docs/` is generated. Edit `templates/` and the schemas, then run
   `make generate`.
4. Issues labelled `good first issue` are reserved for human contributors who
   have not contributed yet. Do not implement one without a claim in the issue.
5. Never put a credential, an organization id, a user id, an email address or
   a workspace name into the tree, a test, a commit message or a pull request.
   Placeholders only (`wrkspc_...`, `user_...`, `example.com`).
