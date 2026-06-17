#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

MANAGER="${1:-npm}"
shift || true
PUBLISH_ARGS=("$@")

if [ "$MANAGER" != "npm" ]; then
  echo "ERROR: official release path is npm only"
  echo "Usage: bash scripts/release-package.sh [npm] [publish args...]"
  exit 1
fi

if [ -f .env ]; then
  echo "Loading environment variables from .env..."
  set -a
  # shellcheck disable=SC1091
  . ./.env
  set +a
fi

if [[ -n "$(git status --short)" ]]; then
  echo "ERROR: working tree is not clean"
  git status --short
  exit 1
fi

echo "--- Step 1: Bumping version and tagging ---"
npx standard-version

PACKAGE_VERSION="$(node -p "require('./package.json').version")"
RELEASE_TAG="v${PACKAGE_VERSION}"

if grep -qE '^[[:space:]]*Version:[[:space:]]*"[^"]+"' cmd/root.go; then
  python3 - "$PACKAGE_VERSION" <<'PY'
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
  if ! git diff --quiet -- cmd/root.go; then
    git add cmd/root.go
    git commit --amend --no-edit
    git tag -f "$RELEASE_TAG"
  fi
fi

echo
echo "--- Step 2: Verifying release integrity ---"
RELEASE_TAG="$RELEASE_TAG" bash scripts/verify-release-integrity.sh

echo
echo "--- Step 3: Running tests/build ---"
GOTOOLCHAIN=auto go test ./...
GOTOOLCHAIN=auto go build ./...

echo
echo "--- Step 4: Dry-run publish via npm ---"
npm publish --dry-run --access public "${PUBLISH_ARGS[@]}"

echo
echo "--- Step 5: Pushing git commit and tag ---"
git push --follow-tags origin main
if ! git ls-remote --tags origin | grep -q "refs/tags/${RELEASE_TAG}$"; then
  echo "remote tag missing after --follow-tags; pushing explicit tag ${RELEASE_TAG}"
  git push origin "refs/tags/${RELEASE_TAG}"
fi
git ls-remote --tags origin | grep "refs/tags/${RELEASE_TAG}$"

echo
echo "--- Step 6: Waiting for CI npm publish ---"
if command -v gh >/dev/null 2>&1; then
  RUN_ID=""
  for _ in $(seq 1 12); do
    RUN_ID="$(gh run list --workflow goreleaser --event push --json databaseId,headSha,headBranch,displayTitle,status,conclusion --limit 20 | python3 -c 'import json, sys
release_tag = sys.argv[1]
runs = json.load(sys.stdin)
for run in runs:
    if run.get("headBranch") == release_tag:
        print(run["databaseId"])
        break
' "$RELEASE_TAG")"
    if [ -n "$RUN_ID" ]; then
      break
    fi
    sleep 10
  done
  if [ -n "$RUN_ID" ]; then
    gh run watch "$RUN_ID" --exit-status
  else
    echo "WARN: could not find matching goreleaser run for ${RELEASE_TAG}"
    echo "Check manually: gh run list --workflow goreleaser --limit 10"
  fi
else
  echo "WARN: gh not available; skipping workflow watch"
fi

echo
PUBLISHED_VERSION=""
for _ in $(seq 1 18); do
  PUBLISHED_VERSION="$(npm view one-click-tools version --registry=https://registry.npmjs.org/ 2>/dev/null || true)"
  if [ "$PUBLISHED_VERSION" = "$PACKAGE_VERSION" ]; then
    break
  fi
  sleep 10
done
if [ "$PUBLISHED_VERSION" != "$PACKAGE_VERSION" ]; then
  echo "ERROR: npm registry version mismatch after CI release"
  echo "expected: $PACKAGE_VERSION"
  echo "actual:   ${PUBLISHED_VERSION:-<empty>}"
  exit 1
fi

echo "npm registry now reports version $PUBLISHED_VERSION"

echo
echo "=========================================="
echo "✅ Release completed successfully"
echo "manager: npm (CI publish)"
echo "1. Git commit & tag created"
echo "2. Verified release integrity"
echo "3. Ran go test ./... and go build ./..."
echo "4. Pushed to GitHub"
echo "5. Waited for CI npm publish"
echo "6. Verified npm registry version"
echo "=========================================="
