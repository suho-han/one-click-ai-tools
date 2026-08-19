#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VERSION_ARG="${1:-}"
if [[ -z "$VERSION_ARG" ]]; then
  echo "Usage: bash scripts/release-package.sh vX.Y.Z|auto"
  exit 1
fi

if [[ "$VERSION_ARG" == "auto" ]]; then
  VERSION_ARG="$(bash scripts/next-version.sh)"
  echo "Auto-detected version (see scripts/next-version.sh for the bump rule): ${VERSION_ARG}"
fi

VERSION="${VERSION_ARG#v}"
RELEASE_TAG="v${VERSION}"

if [[ ! "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "ERROR: release version must look like vX.Y.Z"
  exit 1
fi

if [[ -n "$(git status --short)" ]]; then
  echo "ERROR: working tree is not clean"
  git status --short
  exit 1
fi

echo "--- Step 1: Updating cmd/root.go version ---"
python3 - "$VERSION" <<'PY'
import re
import sys
from pathlib import Path
version = sys.argv[1]
path = Path('cmd/root.go')
text = path.read_text()
updated, count = re.subn(r'(Version:\s*")([^"]+)(")', rf'\g<1>{version}\g<3>', text, count=1)
if count != 1:
    raise SystemExit('failed to update cmd/root.go version')
path.write_text(updated)
PY

git add cmd/root.go
git commit -m "chore(release): ${VERSION}"
git tag -a "$RELEASE_TAG" -m "$RELEASE_TAG"

echo
echo "--- Step 2: Verifying release integrity ---"
RELEASE_TAG="$RELEASE_TAG" bash scripts/verify-release-integrity.sh

echo
echo "--- Step 3: Running tests ---"
GOTOOLCHAIN=auto go test ./...

echo
echo "--- Step 4: Pushing git commit and tag ---"
git push --follow-tags origin main
if ! git ls-remote --tags origin | grep -q "refs/tags/${RELEASE_TAG}$"; then
  echo "remote tag missing after --follow-tags; pushing explicit tag ${RELEASE_TAG}"
  git push origin "refs/tags/${RELEASE_TAG}"
fi
git ls-remote --tags origin | grep "refs/tags/${RELEASE_TAG}$"

echo
echo "--- Step 5: Publishing GitHub Release workflow ---"
if command -v gh >/dev/null 2>&1; then
  RUN_ID=""
  if [[ "${CI:-}" == "true" ]]; then
    # A tag pushed with the workflow's own GITHUB_TOKEN does not trigger the
    # goreleaser workflow's `push: tags` event -- GitHub suppresses further
    # workflow runs triggered by GITHUB_TOKEN to prevent infinite loops. This
    # is the same manual "reissue a release" path documented in
    # docs/release-checklist.md, just invoked programmatically; it *is*
    # exempt from that restriction.
    DISPATCH_AT="$(date -u +%Y-%m-%dT%H:%M:%S)"
    gh workflow run goreleaser --ref main -f release_mode=release -f "git_ref=${RELEASE_TAG}"
    for _ in $(seq 1 12); do
      sleep 5
      RUN_ID="$(gh run list --workflow goreleaser --event workflow_dispatch --json databaseId,createdAt --limit 5 | python3 -c 'import json, sys
dispatch_at = sys.argv[1]
runs = json.load(sys.stdin)
for run in sorted(runs, key=lambda r: r["createdAt"], reverse=True):
    if run["createdAt"] >= dispatch_at:
        print(run["databaseId"])
        break
' "$DISPATCH_AT")"
      if [[ -n "$RUN_ID" ]]; then
        break
      fi
    done
  else
    for _ in $(seq 1 12); do
      RUN_ID="$(gh run list --workflow goreleaser --event push --json databaseId,headBranch --limit 20 | python3 -c 'import json, sys
release_tag = sys.argv[1]
runs = json.load(sys.stdin)
for run in runs:
    if run.get("headBranch") == release_tag:
        print(run["databaseId"])
        break
' "$RELEASE_TAG")"
      if [[ -n "$RUN_ID" ]]; then
        break
      fi
      sleep 10
    done
  fi
  if [[ -n "$RUN_ID" ]]; then
    gh run watch "$RUN_ID" --exit-status
    gh release view "$RELEASE_TAG" --json assets
  else
    echo "WARN: could not find matching goreleaser run for ${RELEASE_TAG}"
    echo "Check manually: gh run list --workflow goreleaser --limit 10"
  fi
else
  echo "WARN: gh not available; skipping workflow watch"
fi

echo
echo "=========================================="
echo "Release completed successfully"
echo "tag: $RELEASE_TAG"
echo "distribution: GitHub Releases"
echo "installer: scripts/install.sh"
echo "=========================================="
