#!/bin/sh
set -eu

REPO=${OCT_REPO:-suho-han/one-click-ai-tools}
VERSION=${OCT_VERSION:-latest}
INSTALL_DIR=${OCT_INSTALL_DIR:-$HOME/.local/bin}
BIN_NAME=${OCT_BIN_NAME:-oct}
SKIP_CHECKSUM=${OCT_INSTALL_SKIP_CHECKSUM:-0}
REQUIRE_CHECKSUM=${OCT_INSTALL_REQUIRE_CHECKSUM:-0}
DRY_RUN=${OCT_INSTALL_DRY_RUN:-0}
RUN_CONFIG=${OCT_INSTALL_RUN_CONFIG:-1}

ui_line() {
    printf '%s\n' "$*"
}

ui_step() {
    printf '◇  %s\n' "$*"
}

ui_note() {
    printf '●  %s\n' "$*"
}

ui_done() {
    printf '◆  %s\n' "$*"
}

print_summary_row() {
    printf '│    [i] %-48.48s │\n' "$*"
}

config_skipped() {
    case "$RUN_CONFIG" in
        0|false|FALSE|False|no|NO|No) return 0 ;;
        *) return 1 ;;
    esac
}

install_config_status() {
    if config_skipped; then
        echo "skipped"
    else
        echo "enabled"
    fi
}

print_install_summary() {
    ui_line "◇  Installation Complete ─────────────────────────────────╮"
    ui_line "│                                                         │"
    ui_line "│  Configuration Summary                                  │"
    ui_line "│                                                         │"
    print_summary_row "Version: ${release_version}"
    print_summary_row "Platform: ${os_name}/${arch_name}"
    print_summary_row "Binary: ${INSTALL_DIR}/${BIN_NAME}"
    print_summary_row "Install config: $(install_config_status)"
    ui_line "│                                                         │"
    ui_line "├─────────────────────────────────────────────────────────╯"
    ui_line "│"
}

run_installed_config() {
    if config_skipped; then
        ui_note "Interactive configuration skipped by OCT_INSTALL_RUN_CONFIG=${RUN_CONFIG}."
        return 0
    fi

    if [ ! -t 1 ] || ! : </dev/tty >/dev/tty 2>/dev/null; then
        ui_note "Interactive configuration skipped because no terminal is available."
        ui_note "Run '${INSTALL_DIR}/${BIN_NAME} config' after opening a shell."
        return 0
    fi

    ui_step "Configuring one-click-ai-tools"
    ui_line "│  Select providers, usage display mode, and optional tokens."
    if "${INSTALL_DIR}/${BIN_NAME}" config </dev/tty >/dev/tty 2>&1; then
        ui_done "Configuration complete"
    else
        ui_note "Configuration was not completed. Run '${INSTALL_DIR}/${BIN_NAME} config' later."
    fi
}

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
        curl -fsSL --retry 3 --connect-timeout 20 -o "$dest" "$url"
        return
    fi
    if command -v wget >/dev/null 2>&1; then
        wget -q -O "$dest" "$url"
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

ui_line "┌"
ui_line "│"
ui_step "Installing one-click-ai-tools"
ui_line "│  repo:        ${REPO}"
ui_line "│  version:     ${release_version}"
ui_line "│  platform:    ${os_name}/${arch_name}"
ui_line "│  asset:       ${asset}"
ui_line "│  install dir: ${INSTALL_DIR}"

if [ "$DRY_RUN" = "1" ]; then
    ui_line "│  archive URL:  ${archive_url}"
    ui_line "│  checksum URL: ${checksum_url}"
    ui_note "Dry run: no files were downloaded, installed, or configured."
    exit 0
fi

need_cmd tar
need_cmd awk

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM
archive_path="${tmpdir}/${asset}"
checksums_path="${tmpdir}/checksums.txt"

ui_step "Downloading release asset"
download "$archive_url" "$archive_path"

if [ "$SKIP_CHECKSUM" != "1" ]; then
    ui_step "Verifying checksum"
    download "$checksum_url" "$checksums_path"
    expected=$(awk -v file="$asset" '$2 == file {print $1; exit}' "$checksums_path")
    if [ -n "$expected" ]; then
        actual=$(sha256_file "$archive_path") || fail "sha256sum or shasum is required for checksum verification"
        [ "$actual" = "$expected" ] || fail "checksum mismatch for ${asset}"
    elif [ "$REQUIRE_CHECKSUM" = "1" ]; then
        fail "checksum entry not found for ${asset}"
    else
        ui_note "Checksum entry not found for ${asset}; continuing without checksum verification." >&2
        ui_note "Set OCT_INSTALL_REQUIRE_CHECKSUM=1 to fail instead." >&2
    fi
else
    ui_note "Checksum verification skipped because OCT_INSTALL_SKIP_CHECKSUM=1"
fi

ui_step "Extracting archive"
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

print_install_summary
case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) ui_note "Add ${INSTALL_DIR} to PATH to run '${BIN_NAME}' from any shell." ;;
esac
"${INSTALL_DIR}/${BIN_NAME}" --version || true
ui_line "│"
run_installed_config
ui_line "│"
ui_line "└  Enjoy!"
