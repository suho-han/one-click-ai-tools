#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

VERSION_ARG="${1:-}"
if [[ -z "$VERSION_ARG" ]]; then
  echo "Usage: bash scripts/release-package.sh vX.Y.Z"
  exit 1
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
echo "--- Step 5: Waiting for GitHub Release workflow ---"
if command -v gh >/dev/null 2>&1; then
  RUN_ID=""
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
