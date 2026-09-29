# 🚀 one-click-ai-tools (oct)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

[한국어](README.md)

> 🎯 **One binary that organizes your AI coding CLIs — update them all, watch every quota plan, schedule the maintenance.**

oct is a single Go binary that pulls scattered AI coding tool management into one place.

- 🔄 **Update them all** — one command updates 10 AI coding CLIs (Claude Code, Codex, Copilot, Cursor, …) by auto-detecting each tool's install manager (brew/npm/official installers)
- 📊 **Watch every quota plan** — collects 14 providers' subscription quotas into one table / JSON / macOS menubar
- ⏰ **Schedule the maintenance** — register updates and session probes with launchd / cron / SchTasks, with threshold-based OS alerts

## 💡 Why oct

Usage tools can't update; updaters can't show usage. oct's core bet is combining both in one binary.

- 📦 **Single Go binary** — no Node/Python runtime, macOS / Linux / Windows
- 🔀 **Hybrid collection** — queries each provider's usage API directly where one exists (remaining percentages, reset state); tools without an API (e.g. Qwen) fall back to local usage records
- 🔑 **Reuses existing credentials** — uses the OAuth tokens / API keys your CLIs already saved; no separate sign-in
- 🔍 **Manager auto-detection** — finds the manager each tool was installed with and runs the right update command (regression-tested)

## ⚡ Quick Start

### 🌐 Installation (GitHub Releases)

```bash
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | sh
```

The installer downloads the matching release binary, verifies it when the release checksum entry is present, and installs `oct` to `~/.local/bin` by default. When run from a terminal it opens `oct config` immediately after installing; once configuration finishes it auto-launches the macOS menubar helper and prints `oct -h` as a quick reference. If `~/.local/bin` is not already on PATH, the installer appends a marker-commented (`# added by one-click-ai-tools installer`) PATH export to the login shell's rc file (`~/.zshrc` for zsh, `~/.bash_profile` for bash on macOS); reruns do not duplicate it.

<details>
<summary><b>🔧 Install options — pinned version · custom path · skip config/PATH/menubar</b></summary>

```bash
# Install a specific version
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_VERSION=v0.1.6 sh

# Install somewhere else
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_DIR=/usr/local/bin sh

# Skip post-install configuration
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_RUN_CONFIG=0 sh

# Skip automatic PATH registration (leaves shell config files untouched)
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_SKIP_PATH=1 sh

# Skip menubar auto-launch
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_RUN_MENUBAR=0 sh
```

</details>

### 🍺 Installation (Homebrew)

```bash
brew install suho-han/tap/oct
```

The formula tracks the latest stable GitHub Release; prereleases stay on the install.sh channel.

### ⏱️ First five minutes

```bash
# 1. Pick your tools/providers (interactive)
oct config

# 2. Update every installed AI CLI — inspect the plan first
oct agent-update --dry-run --explain
oct agent-update

# 3. See all quota plans at once
oct usage            # human-readable table
oct usage --json     # for scripts/pipes
```

## 🗺️ Command map

| Step | Command | What it does |
| --- | --- | --- |
| ⚙️ Setup | `oct config` | pick tools/providers and usage display settings (interactive; notifications are separate) |
| 🔄 Update | `oct agent-update` | update every installed AI CLI (`--dry-run --explain` to preview) |
| 📊 Watch | `oct usage` | one-shot quota snapshot (`--json`, `--compact`, `--format waybar/polybar/swiftbar`, `--notify`) |
| 📊 Watch | `oct monitor` | always-on refreshing screen (`--interval`, `--once`, sort/filter) |
| 📊 Watch | `oct menubar` | persistent macOS menu bar display |
| 🔔 Alert | `oct alert` | bare command opens arrow/key-based interactive alert setup; supports `config`, provider thresholds, `test`, and `snooze` |
| ⏰ Schedule | `oct schedule` | register agent-update / session-refresh with the OS scheduler |
| ⏰ Schedule | `oct session-refresh` | probe session/auth state without sending prompts (`--dry-run`) |
| 🩺 Diagnostics | `oct doctor` | shell PATH / bootstrap diagnostics; `credentials` reports each provider's credential source |
| 🩺 Diagnostics | `oct update` | update oct itself |
| 🧪 Development | `oct release-doctor` | one compact release preflight report |

`oct --help` groups commands by frequency of use (Core / Configuration & Scheduling / Update & Maintenance).

## 🔄 Update them all — `oct agent-update`

```bash
oct agent-update                      # update everything
oct agent-update --dry-run --explain  # print each tool's detected manager and plan, execute nothing
```

oct detects the install manager per tool. Supported: `brew`, `npm`, `pnpm`, `yarn`, `cargo`, `go-install`, `pip`, plus the Cursor/Antigravity official installers — see the [Manager Support Matrix](#-manager-support-matrix) below.

## 📊 Watch every quota plan — `oct usage` / `oct monitor` / `oct menubar` / `oct alert`

```bash
oct usage                        # one-shot snapshot
oct usage --compact              # compact summary (C-45% X-25%)
oct usage --json                 # JSON output
oct usage --notify               # send alerts per threshold/cooldown rules
```

Script authors: the `--json` output shape is a documented stable contract —
see [docs/usage-json-schema.md](docs/usage-json-schema.md).

<details>
<summary><b>🖥️ Statusbar integration — waybar / polybar / swiftbar (for script authors)</b></summary>

Statusbar integration via `--format` (colors escalate at warn 85% / crit 95%):

```bash
oct usage --format waybar        # JSON for a waybar custom/script module (text/tooltip/class)
oct usage --format polybar       # one line for a polybar custom script (%{F#...} colors, escaped %)
oct usage --format swiftbar      # SwiftBar plugin protocol (title + dropdown menu)
oct usage --format waybar --from-snapshot   # render the last oct monitor snapshot instead of fetching live
```

`--from-snapshot` reads `~/.oct/state/usage-latest.json` (written every
`oct monitor` cycle), so a statusbar can re-run oct on a short poll interval
without triggering a full provider fan-out. waybar/polybar/swiftbar and
`--compact` all show remaining quota.

Statusline formats hide providers with no usable value (unconfigured, no data
— the "?" tokens) while keeping real failures (401s) visible; the tooltip's
last line counts what was hidden.

</details>

```bash
oct monitor --interval 10s       # always-on view, 10s refresh
oct monitor --once --sort-by used --desc --top 5 --compact
```

<details>
<summary><b>🔔 Advanced alert settings — oct alert config / snooze</b></summary>

Running `oct alert` with no subcommand opens the arrow/key-based interactive alert setup.

```bash
oct alert

# fine-tune from the CLI
oct alert config show
oct alert config set enabled true
oct alert config set threshold_percent 85
oct alert config set quiet 2h
oct alert config set-provider-threshold 5h 90 --provider codex
oct alert test --provider codex --window 5h --value 91
oct alert snooze set --duration 2h
```

</details>

## ⏰ Schedule the maintenance — `oct schedule` / `oct session-refresh`

```bash
oct schedule enable --task agent-update --interval daily --hour 9   # daily 9am full update
oct schedule enable --task session-refresh --interval 6h            # session probe every 6h
oct schedule --task agent-update                                    # show registration status
oct session-refresh --dry-run                                       # manual probe, zero tokens
```

## 🤖 Supported AI Agents (update targets)

- **Claude Code** (`@anthropic-ai/claude-code`)
- **Command Code** (`command-code`, binary: `commandcode` / `cmd`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (official installer, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (official `agent` install flow via `cursor.com/install`)
- **OpenCode** (`opencode-ai`)
- **Kimi Code** (`@moonshot-ai/kimi-code`, binary: `kimi`) — (experimental)
- **Qwen Code** (`@qwen-code/qwen-code`, binary: `qwen`) — (experimental)
- **MiniMax** (`mmx-cli`, binary: `mmx`) — (experimental)

### 👀 Standalone usage providers (usage-only, no CLI managed)

These are plan/account services with no installable CLI; `oct usage` reports them only when listed in `agent_order` or `enabled_tools` (e.g. `oct config set tools zai`).

- **Z.ai (GLM Coding Plan)** — `ZAI_API_KEY` / `ZHIPU_API_KEY`, or an OpenCode `zai-coding-plan` login — (experimental)
- **DeepSeek** — `DEEPSEEK_API_KEY` — (experimental)
- **OpenRouter** — `OPENROUTER_API_KEY`
- **Grok (xAI SuperGrok)** — `grok login` credential or `GROK_OAUTH_TOKEN` — (experimental)

`(experimental)` marks usage integrations whose live API responses have not been verified against a real subscription yet (verified live: Codex, Claude, Command Code, OpenCode, Antigravity, OpenRouter).

## 👥 Multi-account (codex, kimi, qwen, grok)

If you hold several accounts for the same provider, register each account's credential directory so their usage shows up together.

<details>
<summary><b>🧭 Managing accounts from the interactive screen</b></summary>

The simplest path is the interactive screen. Pick the **"➕ Add codex account…"** entry under the codex row and a browser login opens right away; when sign-in finishes, oct derives the account name from the account email and registers it (e.g. `hansuho36@...` → `codex:han***o36`, home `~/.codex-hansuho36`) — you only sign in, nothing to type.

Once an account is registered, a **"Manage existing accounts (N)"** row appears under codex. Expand it with Enter to see the masked account names (`codex:han***o36`); pressing Enter again on an account row offers **🔄 Reconnect** (sign in again for fresh tokens) and **❌ Disconnect** (removes it from oct's config only; the stored credential files stay on disk).

</details>

```bash
oct config                                              # ➕ Add codex account… → type a name → sign in
oct config account add codex work                       # CLI: with just a name, ~/.codex-work is derived
oct config account add kimi personal ~/.kimi-code-alt   # or specify the home dir explicitly
oct config account list                                 # show registered accounts
oct config account remove codex work                    # unregister an account
```

Once registered, rows like `codex:work` or `kimi:personal` appear directly under the provider row in the `oct usage` table, and compact/statusline tokens get the alias's initial appended (`XW`, `KP`). Account rows show whenever their base row is shown, and they work as alert targets too — e.g. `oct alert config set-provider-threshold ... --provider codex:work`.

## 🍎 Menubar helper (macOS)

macOS release tarballs bundle the Swift menubar helper: the install script and `oct update` install it next to `oct` (default `~/.local/bin`). You can still build it from source yourself.

```bash
oct menubar                # run the menu bar app
oct menubar doctor         # inspect helper resolution / launch mode
oct menubar build-helper   # build the Swift helper
oct menubar install-helper # install to ~/.local/bin/OctMenubarApp
```

<details>
<summary><b>⚙️ Settings screen scope and fetch timeout</b></summary>

In the menubar app's Settings, General is merged into the Configuration screen. Its alert section exposes only safe global alert settings; it does not expose provider-specific thresholds or snooze controls. Use `oct alert config ...` and `oct alert snooze ...` for those advanced controls.

The overall fetch deadline of `oct usage --json` (15s by default) is tunable via the `OCT_USAGE_FETCH_TIMEOUT` environment variable (e.g. `OCT_USAGE_FETCH_TIMEOUT=8s oct usage`). It is clamped to the 5–18s range, so a partial result always arrives within the menubar helper's 20s process timeout.

</details>

## 🧰 Manager Support Matrix

<details>
<summary><b>Open — detection strategy · install command · built-in use</b></summary>

| Manager | Detection strategy | Install path | Built-in use |
| --- | --- | --- | --- |
| `brew` | binary under `brew --prefix` / `brew list` | `brew upgrade <formula>` | Claude, Cursor, OpenCode, Codex when Homebrew-owned |
| `npm` | `npm prefix -g` / `npm list -g` | `npm install -g <package>` | default fallback for Claude, Command Code, OpenCode, Codex, Copilot |
| `pnpm` | `pnpm bin -g` / `pnpm list -g` | `pnpm add -g <package>` | provenance-based detection only |
| `yarn` | `yarn global bin` / `yarn global list` | `yarn global add <package>` | provenance-based detection only |
| `cargo` | `cargo:` package prefix / cargo bin path | `cargo install <crate> --locked` | explicit package override |
| `go-install` | `go:` package prefix / `go env GOPATH` bin path | `go install <package>@latest` | explicit package override |
| `pip` | `pip:` package prefix / `python3 -m site --user-base` bin path | `python3 -m pip install --upgrade <package>` | explicit package override |
| `cursor-agent` | tool identity (`cursor-agent` / `cursor` / `agent`) | `curl https://cursor.com/install -fsS \| bash` | Cursor CLI |
| `antigravity-installer` | tool identity (`agy` / `antigravity`, legacy `gemini*`) | `curl -fsSL https://antigravity.google/cli/install.sh \| bash` | Antigravity CLI |

The built-in support matrix is regression-tested in `internal/update/manager_test.go` so manager fallback changes stay explicit.

</details>

## ✅ Requirements

- **Users**: Homebrew for agent-update support on macOS (Linux/Windows get the CLI features only)
- **Developers (build/test from source)**: **Go >= 1.25**

## 🚢 Release

- Primary binary distribution: GitHub Releases + `scripts/install.sh`
- Local release wrapper: `bash scripts/release-package.sh vX.Y.Z`

## 📄 License

MIT © Suho Han
