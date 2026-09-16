#!/usr/bin/env bash
# Fails if any line of an external denylist appears in the repository tree or
# in commit messages. The denylist lives outside the repository and is never
# committed; point PUBLISH_DENYLIST at it.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ -z "${PUBLISH_DENYLIST:-}" ]]; then
  echo "PUBLISH_DENYLIST is not set; skipping." >&2
  exit 0
fi
if [[ ! -r "$PUBLISH_DENYLIST" ]]; then
  echo "PUBLISH_DENYLIST=$PUBLISH_DENYLIST is not readable." >&2
  exit 2
fi

status=0
while IFS= read -r term; do
  [[ -z "$term" || "$term" == \#* ]] && continue
  if grep -rIil --exclude-dir=.git --exclude-dir=dist --exclude-dir=.terraform -e "$term" . >/dev/null; then
    echo "denylisted term found in tree: (redacted)" >&2
    grep -rIil --exclude-dir=.git --exclude-dir=dist --exclude-dir=.terraform -e "$term" . >&2
    status=1
  fi
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1 && git log --all --format='%an %ae %s %b' | grep -qi -e "$term"; then
    echo "denylisted term found in git history (redacted)" >&2
    status=1
  fi
done < "$PUBLISH_DENYLIST"

if [[ $status -eq 0 ]]; then
  echo "publish-check: clean"
fi
exit $status
