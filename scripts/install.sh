#!/bin/sh
set -eu

REPO=${OCT_REPO:-suho-han/one-click-ai-tools}
VERSION=${OCT_VERSION:-latest}
INSTALL_DIR=${OCT_INSTALL_DIR:-$HOME/.local/bin}
# Pre-release installs (beta/rc/alpha) land as 'oct-beta' so they coexist
# with the stable 'oct' instead of clobbering it; override with OCT_BIN_NAME.
DEFAULT_BIN_NAME=oct
# Each track installs its own menubar helper next to its binary: the stable
# OctMenubarApp and a beta OctMenubarApp-beta run and are observed as
# separate menubar apps.
STABLE_HELPER_NAME=OctMenubarApp
BETA_HELPER_NAME=OctMenubarApp-beta
SKIP_CHECKSUM=${OCT_INSTALL_SKIP_CHECKSUM:-0}
# Checksum verification is fail-closed by default: a release asset with no
# checksums.txt entry aborts the install. Opt out with =0 (not recommended).
REQUIRE_CHECKSUM=${OCT_INSTALL_REQUIRE_CHECKSUM:-1}
SKIP_ATTESTATION=${OCT_INSTALL_SKIP_ATTESTATION:-0}
REQUIRE_ATTESTATION=${OCT_INSTALL_REQUIRE_ATTESTATION:-0}
DRY_RUN=${OCT_INSTALL_DRY_RUN:-0}
RUN_CONFIG=${OCT_INSTALL_RUN_CONFIG:-1}
# PATH is registered automatically for the login shell (marker-commented rc
# edit). Opt out with =1 to keep shell config files untouched.
SKIP_PATH=${OCT_INSTALL_SKIP_PATH:-0}
# After the interactive part of the install, the darwin menubar helper is
# launched automatically. Opt out with =0.
RUN_MENUBAR=${OCT_INSTALL_RUN_MENUBAR:-1}

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
    print_summary_row "Shell PATH: ${path_summary}"
    print_summary_row "Menubar helper: ${helper_summary}"
    print_summary_row "Install config: $(install_config_status)"
    ui_line "│                                                         │"
    ui_line "├─────────────────────────────────────────────────────────╯"
    ui_line "│"
}

# install_menubar_helper installs the Swift menubar helper bundled at the
# root of darwin release tarballs next to the oct binary (a PATH dir, so the
# helper discovery in 'oct menubar doctor' finds it). Best-effort: a failure
# or an older asset without the helper only notes and continues -- the
# legacy Go menubar remains the fallback.
install_menubar_helper() {
    helper_installed=''
    if [ "$os_name" != "darwin" ]; then
        helper_summary='n/a (macOS only)'
        return 0
    fi
    helper_candidate="${tmpdir}/${STABLE_HELPER_NAME}"
    if [ ! -f "$helper_candidate" ]; then
        helper_summary='not bundled in this asset'
        ui_note "Menubar helper not found in the archive; skipping (older releases do not bundle it)."
        return 0
    fi
    ui_step "Installing menubar helper (${HELPER_NAME})"
    if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
        helper_summary='install failed (legacy menubar still works)'
        ui_note "Menubar helper install failed; continuing without it."
        return 0
    fi
    if command -v install >/dev/null 2>&1; then
        if install -m 0755 "$helper_candidate" "${INSTALL_DIR}/${HELPER_NAME}"; then
            helper_installed="${INSTALL_DIR}/${HELPER_NAME}"
        fi
    else
        if cp "$helper_candidate" "${INSTALL_DIR}/${HELPER_NAME}" && chmod 0755 "${INSTALL_DIR}/${HELPER_NAME}"; then
            helper_installed="${INSTALL_DIR}/${HELPER_NAME}"
        fi
    fi
    if [ -n "$helper_installed" ]; then
        helper_summary="$helper_installed"
        ui_done "Menubar helper installed"
    else
        helper_summary='install failed (legacy menubar still works)'
        ui_note "Menubar helper install failed; continuing without it."
    fi
    return 0
}

# ensure_path_configured registers INSTALL_DIR on PATH for the user's login
# shell ($SHELL, not the shell piping this script): zsh -> ~/.zshrc, bash ->
# ~/.bash_profile on darwin / ~/.bashrc elsewhere, anything else -> ~/.profile.
# uv/rustup-style: a marker-commented append, so a rerun is a no-op and the
# block is easy to find and remove; fish gets a fish_add_path hint instead
# (different PATH syntax). Best-effort like install_menubar_helper: any
# failure only notes and continues, and OCT_INSTALL_SKIP_PATH=1 touches no
# rc file. Sets path_summary for the installation summary box.
ensure_path_configured() {
    path_summary=''
    if [ "$SKIP_PATH" = "1" ]; then
        path_summary='skipped (OCT_INSTALL_SKIP_PATH=1)'
        return 0
    fi
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*)
            path_summary='already on PATH'
            return 0
            ;;
    esac

    marker='# added by one-click-ai-tools installer'
    case "$(basename "${SHELL:-sh}")" in
        zsh) rc_file="${ZDOTDIR:-$HOME}/.zshrc" ;;
        bash)
            if [ "$os_name" = "darwin" ]; then
                rc_file="$HOME/.bash_profile"
            else
                rc_file="$HOME/.bashrc"
            fi
            ;;
        fish)
            path_summary='not updated (fish)'
            ui_note "fish shell detected: run 'fish_add_path ${INSTALL_DIR}' once to persist PATH."
            return 0
            ;;
        *) rc_file="$HOME/.profile" ;;
    esac

    if [ -f "$rc_file" ] && grep -qF "$marker" "$rc_file" 2>/dev/null; then
        path_summary="already in ${rc_file}"
        ui_note "PATH entry already present in ${rc_file}; restart the shell to pick it up."
        return 0
    fi

    # The leading blank line also guards against gluing our comment onto a
    # rc file whose last line lacks a trailing newline.
    if ! {
        printf '\n%s\n' "$marker"
        printf 'export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    } >> "$rc_file" 2>/dev/null; then
        path_summary='not updated (write failed)'
        ui_note "Could not write to ${rc_file}; add ${INSTALL_DIR} to PATH manually."
        return 0
    fi

    path_summary="added to ${rc_file}"
    ui_done "PATH updated: added ${INSTALL_DIR} to ${rc_file}"
    ui_note "Run 'source ${rc_file}' or open a new terminal to use '${BIN_NAME}' right away."
}

# launch_menubar_app starts the Swift menubar helper placed by
# install_menubar_helper once the interactive part of the install is over.
# The helper is a user-session app: darwin-only, and a rerun must not stack
# duplicate instances, so an already-running one is kept. Opt out with
# OCT_INSTALL_RUN_MENUBAR=0. Best-effort like the rest of the install: a
# failure here never aborts the install.
launch_menubar_app() {
    if [ "$os_name" != "darwin" ]; then
        return 0
    fi
    if [ "$RUN_MENUBAR" = "0" ]; then
        ui_note "Menubar auto-launch skipped because OCT_INSTALL_RUN_MENUBAR=0."
        return 0
    fi
    if [ -z "$helper_installed" ]; then
        ui_note "Menubar helper not installed; skipping auto-launch."
        return 0
    fi
    if command -v pgrep >/dev/null 2>&1 && pgrep -x "$HELPER_NAME" >/dev/null 2>&1; then
        ui_note "Menubar helper already running; keeping the existing instance."
        return 0
    fi
    ui_step "Launching menubar helper"
    nohup "$helper_installed" >/dev/null 2>&1 &
    ui_done "Menubar helper launched"
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
        wget -q --timeout=20 --tries=3 -O "$dest" "$url"
        return
    fi
    fail "curl or wget is required"
}

# sha256_file prints the hex digest of $1. The hash command's exit status is
# checked directly (POSIX sh has no pipefail), so a failing hash surfaces as
# an error instead of an empty digest that reads as a bogus "checksum
# mismatch".
sha256_file() {
    hash=''
    if command -v sha256sum >/dev/null 2>&1; then
        hash=$(sha256sum "$1") || return 1
    elif command -v shasum >/dev/null 2>&1; then
        hash=$(shasum -a 256 "$1") || return 1
    else
        return 1
    fi
    printf '%s\n' "$hash" | awk '{print $1}'
}

normalize_version() {
    case "$VERSION" in
        latest) echo latest ;;
        v*) echo "$VERSION" ;;
        *) echo "v$VERSION" ;;
    esac
}

# resolve_bin_name picks the installed command name: an explicit
# OCT_BIN_NAME always wins; otherwise pre-release versions install as
# oct-beta so a beta can be tried without touching the stable oct.
resolve_bin_name() {
    if [ -n "${OCT_BIN_NAME:-}" ]; then
        printf '%s\n' "$OCT_BIN_NAME"
        return
    fi
    case "$1" in
        *-beta*|*-rc*|*-alpha*|*-test*|*-dev) printf '%s\n' oct-beta ;;
        *) printf '%s\n' "$DEFAULT_BIN_NAME" ;;
    esac
}

os_name=$(detect_os)
arch_name=$(detect_arch)
release_version=$(normalize_version)
BIN_NAME=$(resolve_bin_name "$release_version")
case "$BIN_NAME" in
    oct-beta) HELPER_NAME=$BETA_HELPER_NAME ;;
    *) HELPER_NAME=$STABLE_HELPER_NAME ;;
esac
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
        actual=$(sha256_file "$archive_path") || fail "failed to compute sha256 for ${asset} (is sha256sum or shasum available?)"
        [ "$actual" = "$expected" ] || fail "checksum mismatch for ${asset}"
    elif [ "$REQUIRE_CHECKSUM" != "0" ]; then
        fail "checksum entry not found for ${asset} (set OCT_INSTALL_REQUIRE_CHECKSUM=0 to install without verification)"
    else
        ui_note "Checksum entry not found for ${asset}; continuing without checksum verification (OCT_INSTALL_REQUIRE_CHECKSUM=0)." >&2
    fi
else
    ui_note "Checksum verification skipped because OCT_INSTALL_SKIP_CHECKSUM=1"
fi

# Artifact attestation: proves the archive was built by this repository's
# GitHub Actions (Sigstore provenance), on top of the checksum which only
# proves the download matches whatever was published. Best-effort: the gh
# CLI is not an install prerequisite, so a missing or too-old gh is a skip
# unless OCT_INSTALL_REQUIRE_ATTESTATION=1. An actual verification failure
# always aborts the install.
if [ "$SKIP_ATTESTATION" = "1" ]; then
    ui_note "Attestation verification skipped because OCT_INSTALL_SKIP_ATTESTATION=1"
elif command -v gh >/dev/null 2>&1 && gh attestation --help 2>/dev/null | grep -q "verify"; then
    ui_step "Verifying artifact attestation"
    if gh attestation verify "$archive_path" -R "$REPO" --digest-alg sha256 >/dev/null 2>&1; then
        ui_line "│  attestation: verified (Sigstore provenance from ${REPO})"
    else
        fail "artifact attestation verification failed for ${asset} (set OCT_INSTALL_SKIP_ATTESTATION=1 to override)"
    fi
elif [ "$REQUIRE_ATTESTATION" = "1" ]; then
    fail "attestation verification required (OCT_INSTALL_REQUIRE_ATTESTATION=1) but gh CLI with attestation support is unavailable"
else
    ui_note "Skipping artifact attestation verification (gh CLI not found or too old)."
    ui_note "Set OCT_INSTALL_REQUIRE_ATTESTATION=1 to fail instead." >&2
fi

ui_step "Extracting archive"
tar -xzf "$archive_path" -C "$tmpdir"

# The archive always carries the binary as 'oct' regardless of what the
# installed command will be named (pre-releases install as oct-beta).
ARCHIVE_BIN_NAME=oct
candidate="${tmpdir}/${ARCHIVE_BIN_NAME}"
if [ ! -f "$candidate" ]; then
    candidate=""
    for path in "$tmpdir"/*/"${ARCHIVE_BIN_NAME}"; do
        if [ -f "$path" ]; then
            candidate=$path
            break
        fi
    done
fi
[ -n "$candidate" ] && [ -f "$candidate" ] || fail "binary '${ARCHIVE_BIN_NAME}' not found in archive"

mkdir -p "$INSTALL_DIR"
if command -v install >/dev/null 2>&1; then
    install -m 0755 "$candidate" "${INSTALL_DIR}/${BIN_NAME}"
else
    cp "$candidate" "${INSTALL_DIR}/${BIN_NAME}"
    chmod 0755 "${INSTALL_DIR}/${BIN_NAME}"
fi

install_menubar_helper

ensure_path_configured

print_install_summary
"${INSTALL_DIR}/${BIN_NAME}" --version || true
ui_line "│"
run_installed_config
ui_line "│"
launch_menubar_app
ui_step "Quick reference: ${BIN_NAME} -h"
"${INSTALL_DIR}/${BIN_NAME}" -h || true
ui_line "│"
ui_line "└  Enjoy!"
