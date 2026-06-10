#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

MANAGER="${1:-npm}"
shift || true
PUBLISH_ARGS=("$@")

case "$MANAGER" in
  npm|pnpm)
    ;;
  *)
    echo "Usage: bash scripts/release-package.sh [npm|pnpm] [publish args...]"
    exit 1
    ;;
esac

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
echo "--- Step 4: Dry-run publish via $MANAGER ---"
case "$MANAGER" in
  npm)
    npm publish --dry-run --access public "${PUBLISH_ARGS[@]}"
    ;;
  pnpm)
    pnpm publish --dry-run --access public --no-git-checks "${PUBLISH_ARGS[@]}"
    ;;
esac

echo
echo "--- Step 5: Pushing git commit and tag ---"
git push --follow-tags origin main

echo
echo "--- Step 6: Publishing via $MANAGER ---"
case "$MANAGER" in
  npm)
    npm publish --access public "${PUBLISH_ARGS[@]}"
    ;;
  pnpm)
    pnpm publish --access public --no-git-checks "${PUBLISH_ARGS[@]}"
    ;;
esac

echo
echo "=========================================="
echo "✅ Release completed successfully"
echo "manager: $MANAGER"
echo "1. Git commit & tag created"
echo "2. Verified release integrity"
echo "3. Ran go test ./... and go build ./..."
echo "4. Pushed to GitHub"
echo "5. Published package"
echo "=========================================="
