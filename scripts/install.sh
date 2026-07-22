#!/bin/sh
set -eu

REPO=${OCT_REPO:-suho-han/one-click-ai-tools}
VERSION=${OCT_VERSION:-latest}
INSTALL_DIR=${OCT_INSTALL_DIR:-$HOME/.local/bin}
BIN_NAME=${OCT_BIN_NAME:-oct}
SKIP_CHECKSUM=${OCT_INSTALL_SKIP_CHECKSUM:-0}
REQUIRE_CHECKSUM=${OCT_INSTALL_REQUIRE_CHECKSUM:-0}
DRY_RUN=${OCT_INSTALL_DRY_RUN:-0}

fail() {
    echo "one-click-ai-tools installer: $*" >&2
    exit 1
}

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

detect_os() {
    case "$(uname -s)" in
        Darwin) echo darwin ;;
        Linux) echo linux ;;
        *) fail "unsupported OS: $(uname -s)" ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64) echo amd64 ;;
        arm64|aarch64) echo arm64 ;;
        *) fail "unsupported architecture: $(uname -m)" ;;
    esac
}

download() {
    url=$1
    dest=$2
    if command -v curl >/dev/null 2>&1; then
        curl -fL --retry 3 --connect-timeout 20 -o "$dest" "$url"
        return
    fi
    if command -v wget >/dev/null 2>&1; then
        wget -O "$dest" "$url"
        return
    fi
    fail "curl or wget is required"
}

sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print $1}'
        return
    fi
    if command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{print $1}'
        return
    fi
    return 1
}

normalize_version() {
    case "$VERSION" in
        latest) echo latest ;;
        v*) echo "$VERSION" ;;
        *) echo "v$VERSION" ;;
    esac
}

os_name=$(detect_os)
arch_name=$(detect_arch)
release_version=$(normalize_version)
asset="one-click-ai-tools_${os_name}_${arch_name}.tar.gz"

if [ "$release_version" = "latest" ]; then
    release_base="https://github.com/${REPO}/releases/latest/download"
else
    release_base="https://github.com/${REPO}/releases/download/${release_version}"
fi

archive_url="${release_base}/${asset}"
checksum_url="${release_base}/checksums.txt"

echo "one-click-ai-tools installer"
echo "  repo:        ${REPO}"
echo "  version:     ${release_version}"
echo "  platform:    ${os_name}/${arch_name}"
echo "  asset:       ${asset}"
echo "  install dir: ${INSTALL_DIR}"

if [ "$DRY_RUN" = "1" ]; then
    echo "  archive URL:  ${archive_url}"
    echo "  checksum URL: ${checksum_url}"
    echo "dry run: no files were downloaded or installed."
    exit 0
fi

need_cmd tar
need_cmd awk

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM
archive_path="${tmpdir}/${asset}"
checksums_path="${tmpdir}/checksums.txt"

echo "downloading ${archive_url}"
download "$archive_url" "$archive_path"

if [ "$SKIP_CHECKSUM" != "1" ]; then
    echo "verifying checksum"
    download "$checksum_url" "$checksums_path"
    expected=$(awk -v file="$asset" '$2 == file {print $1; exit}' "$checksums_path")
    if [ -n "$expected" ]; then
        actual=$(sha256_file "$archive_path") || fail "sha256sum or shasum is required for checksum verification"
        [ "$actual" = "$expected" ] || fail "checksum mismatch for ${asset}"
    elif [ "$REQUIRE_CHECKSUM" = "1" ]; then
        fail "checksum entry not found for ${asset}"
    else
        echo "warning: checksum entry not found for ${asset}; continuing without checksum verification." >&2
        echo "         Set OCT_INSTALL_REQUIRE_CHECKSUM=1 to fail instead." >&2
    fi
else
    echo "checksum verification skipped because OCT_INSTALL_SKIP_CHECKSUM=1"
fi

echo "extracting archive"
tar -xzf "$archive_path" -C "$tmpdir"

candidate="${tmpdir}/${BIN_NAME}"
if [ ! -f "$candidate" ]; then
    candidate=""
    for path in "$tmpdir"/*/"$BIN_NAME"; do
        if [ -f "$path" ]; then
            candidate=$path
            break
        fi
    done
fi
[ -n "$candidate" ] && [ -f "$candidate" ] || fail "binary '${BIN_NAME}' not found in archive"

mkdir -p "$INSTALL_DIR"
if command -v install >/dev/null 2>&1; then
    install -m 0755 "$candidate" "${INSTALL_DIR}/${BIN_NAME}"
else
    cp "$candidate" "${INSTALL_DIR}/${BIN_NAME}"
    chmod 0755 "${INSTALL_DIR}/${BIN_NAME}"
fi

echo "installed ${BIN_NAME} to ${INSTALL_DIR}/${BIN_NAME}"
case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) echo "add ${INSTALL_DIR} to PATH to run '${BIN_NAME}' from any shell." ;;
esac
"${INSTALL_DIR}/${BIN_NAME}" --version || true
