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

# Asset-level helper verification is opt-in because it downloads the published
# darwin tarballs, which only exist after the darwin-assets job has uploaded
# them (the local pre-tag run in release-package.sh has nothing to download).
verify_darwin_helper_assets() {
  local repo="${GITHUB_REPOSITORY:-suho-han/one-click-ai-tools}"
  local tmp_assets checksums arch asset expected actual file_output
  tmp_assets="$(mktemp -d)"
  trap 'rm -rf "$tmp_assets"' EXIT

  if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    gh release download "$RELEASE_TAG" -R "$repo" -p checksums.txt -D "$tmp_assets" --clobber
  else
    curl -fsSL --retry 3 -o "$tmp_assets/checksums.txt" \
      "https://github.com/${repo}/releases/download/${RELEASE_TAG}/checksums.txt"
  fi
  checksums="$tmp_assets/checksums.txt"
  [[ -s "$checksums" ]] || { echo "ERROR: checksums.txt missing for $RELEASE_TAG"; return 1; }

  for arch in arm64 amd64; do
    asset="one-click-ai-tools_darwin_${arch}.tar.gz"
    if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
      gh release download "$RELEASE_TAG" -R "$repo" -p "$asset" -D "$tmp_assets" --clobber
    else
      curl -fsSL --retry 3 -o "$tmp_assets/$asset" \
        "https://github.com/${repo}/releases/download/${RELEASE_TAG}/$asset"
    fi

    expected="$(awk -v file="$asset" '$2 == file {print $1; exit}' "$checksums")"
    [[ -n "$expected" ]] || { echo "ERROR: no checksum entry for $asset"; return 1; }
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "$tmp_assets/$asset" | awk '{print $1}')"
    else
      actual="$(shasum -a 256 "$tmp_assets/$asset" | awk '{print $1}')"
    fi
    [[ "$actual" == "$expected" ]] || { echo "ERROR: checksum mismatch for $asset"; return 1; }

    tar -tzf "$tmp_assets/$asset" | grep -qx "OctMenubarApp" || {
      echo "ERROR: OctMenubarApp missing from $asset"
      return 1
    }
    tar -xzf "$tmp_assets/$asset" -C "$tmp_assets" OctMenubarApp
    [[ -x "$tmp_assets/OctMenubarApp" ]] || {
      echo "ERROR: OctMenubarApp in $asset is not executable"
      return 1
    }
    # The helper must be a universal binary: the same file rides in both the
    # amd64 and arm64 tarballs (see the darwin-assets workflow job).
    file_output="$(file "$tmp_assets/OctMenubarApp")"
    echo "$file_output" | grep -q "universal binary with 2 architectures" || {
      echo "ERROR: OctMenubarApp in $asset is not a universal binary: $file_output"
      return 1
    }
    echo "OK: $asset contains executable universal OctMenubarApp"
    rm -f "$tmp_assets/OctMenubarApp"
  done
}

if [[ -n "${RELEASE_TAG:-}" && "${OCT_VERIFY_ASSETS:-0}" == "1" ]]; then
  verify_darwin_helper_assets
fi
