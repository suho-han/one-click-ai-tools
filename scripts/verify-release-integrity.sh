#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

PACKAGE_VERSION="$(node -p "require('./package.json').version")"
ROOT_VERSION="$(grep -E '^[[:space:]]*Version:[[:space:]]*"[^"]+"' cmd/root.go | sed -E 's/.*"([^"]+)".*/\1/' | head -1)"

if [[ -z "$ROOT_VERSION" ]]; then
  echo "ERROR: cannot parse Version from cmd/root.go"
  exit 1
fi

if [[ "$PACKAGE_VERSION" != "$ROOT_VERSION" ]]; then
  echo "ERROR: version mismatch"
  echo "  package.json: $PACKAGE_VERSION"
  echo "  cmd/root.go:  $ROOT_VERSION"
  exit 1
fi

echo "OK: version parity package.json == cmd/root.go == $PACKAGE_VERSION"

if [[ -n "${RELEASE_TAG:-}" ]]; then
  TAG_VERSION="${RELEASE_TAG#v}"
  if [[ "$TAG_VERSION" != "$PACKAGE_VERSION" ]]; then
    echo "ERROR: release tag does not match package version"
    echo "  RELEASE_TAG:  $RELEASE_TAG"
    echo "  package.json: $PACKAGE_VERSION"
    exit 1
  fi
  echo "OK: tag parity RELEASE_TAG($RELEASE_TAG) == v$PACKAGE_VERSION"
fi

npm pack --dry-run >/dev/null

echo "OK: npm pack --dry-run passed"
