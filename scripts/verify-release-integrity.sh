#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ROOT_VERSION="$(python3 - <<'PY'
import re
from pathlib import Path
match = re.search(r'Version:\s*"([^"]+)"', Path('cmd/root.go').read_text())
if match:
    print(match.group(1))
PY
)"

if [[ -z "$ROOT_VERSION" ]]; then
  echo "ERROR: cannot parse Version from cmd/root.go"
  exit 1
fi

echo "OK: cmd/root.go version = $ROOT_VERSION"

if [[ -n "${RELEASE_TAG:-}" ]]; then
  TAG_VERSION="${RELEASE_TAG#v}"
  if [[ "$TAG_VERSION" != "$ROOT_VERSION" ]]; then
    echo "ERROR: release tag does not match root command version"
    echo "  RELEASE_TAG:  $RELEASE_TAG"
    echo "  cmd/root.go:  $ROOT_VERSION"
    exit 1
  fi
  echo "OK: tag parity RELEASE_TAG($RELEASE_TAG) == v$ROOT_VERSION"
fi

bash -n scripts/install.sh
GOTOOLCHAIN=auto go build ./...

echo "OK: install script syntax and Go build passed"
