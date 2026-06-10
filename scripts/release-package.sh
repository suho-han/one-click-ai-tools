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

echo
echo "--- Step 2: Verifying release integrity ---"
RELEASE_TAG="$(git describe --tags --abbrev=0)" bash scripts/verify-release-integrity.sh

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
