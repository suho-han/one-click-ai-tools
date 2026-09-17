# one-click-ai-tools (oct)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

[한국어](README.md)

> **One binary that organizes your AI coding CLIs — update them all, watch every quota plan, schedule the maintenance.**

oct is a single Go binary that pulls scattered AI coding tool management into one place.

- **Update them all** — one command updates 10 AI coding CLIs (Claude Code, Codex, Copilot, Cursor, …) by auto-detecting each tool's install manager (brew/npm/official installers)
- **Watch every quota plan** — collects 14 providers' subscription quotas into one table / JSON / macOS menubar
- **Schedule the maintenance** — register updates and session probes with launchd / cron / SchTasks, with threshold-based OS alerts

## Why oct

Usage tools can't update; updaters can't show usage. oct's core bet is combining both in one binary.

- **Single Go binary** — no Node/Python runtime, macOS / Linux / Windows
- **Hybrid collection** — queries each provider's usage API directly where one exists (remaining percentages, reset state); tools without an API (e.g. Qwen) fall back to local usage records
- **Reuses existing credentials** — uses the OAuth tokens / API keys your CLIs already saved; no separate sign-in
- **Manager auto-detection** — finds the manager each tool was installed with and runs the right update command (regression-tested)

## Quick Start

### Installation (GitHub Releases)

```bash
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | sh
```

The installer downloads the matching release binary, verifies it when the release checksum entry is present, and installs `oct` to `~/.local/bin` by default. When run from a terminal it opens `oct config` immediately after installing.

```bash
# Install a specific version
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_VERSION=v0.1.5 sh

# Install somewhere else
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_DIR=/usr/local/bin sh

# Skip post-install configuration
curl -fsSL https://raw.githubusercontent.com/suho-han/one-click-ai-tools/main/scripts/install.sh | OCT_INSTALL_RUN_CONFIG=0 sh
```

### First five minutes

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

## Command map

| Step | Command | What it does |
| --- | --- | --- |
| Setup | `oct config` | pick tools/providers and usage display settings (interactive; notifications are separate) |
| Update | `oct agent-update` | update every installed AI CLI (`--dry-run --explain` to preview) |
| Watch | `oct usage` | one-shot quota snapshot (`--json`, `--compact`, `--notify`) |
| Watch | `oct monitor` | always-on refreshing screen (`--interval`, `--once`, sort/filter) |
| Watch | `oct menubar` | persistent macOS menu bar display |
| Alert | `oct alert` | bare command opens arrow/key-based interactive alert setup; supports `config`, provider thresholds, `test`, and `snooze` |
| Schedule | `oct schedule` | register agent-update / session-refresh with the OS scheduler |
| Schedule | `oct session-refresh` | probe session/auth state without sending prompts (`--dry-run`) |
| Diagnostics | `oct doctor` | shell PATH / bootstrap diagnostics |
| Diagnostics | `oct update` | update oct itself |
| Development | `oct release-doctor` | one compact release preflight report |

`oct --help` groups commands by frequency of use (Core / Configuration & Scheduling / Update & Maintenance).

### 1) Update them all — `oct agent-update`

```bash
oct agent-update                      # update everything
oct agent-update --dry-run --explain  # print each tool's detected manager and plan, execute nothing
```

oct detects the install manager per tool. Supported: `brew`, `npm`, `pnpm`, `yarn`, `cargo`, `go-install`, `pip`, plus the Cursor/Antigravity official installers — see the [Manager Support Matrix](#manager-support-matrix) below.

### 2) Watch every quota plan — `oct usage` / `oct monitor` / `oct menubar` / `oct alert`

```bash
oct usage                        # one-shot snapshot
oct usage --compact              # compact summary (C-45% X-25%)
oct usage --json                 # JSON output
oct usage --notify               # send alerts per threshold/cooldown rules

oct monitor --interval 10s       # always-on view, 10s refresh
oct monitor --once --sort-by used --desc --top 5 --compact

# bare command: arrow/key-based interactive alert setup
oct alert

# advanced alert controls remain under oct alert
oct alert config show
oct alert config set enabled true
oct alert config set threshold_percent 85
oct alert config set quiet_hours 00:00-08:00
oct alert config set-provider-threshold 5h 90 --provider codex
oct alert test --provider codex --window 5h --value 91
oct alert snooze set --duration 2h
```

### 3) Schedule the maintenance — `oct schedule` / `oct session-refresh`

```bash
oct schedule enable --task agent-update --interval daily --hour 9   # daily 9am full update
oct schedule enable --task session-refresh --interval 6h            # session probe every 6h
oct schedule --task agent-update                                    # show registration status
oct session-refresh --dry-run                                       # manual probe, zero tokens
```

## Supported AI Agents (update targets)

- **Claude Code** (`@anthropic-ai/claude-code`)
- **Command Code** (`command-code`, binary: `commandcode` / `cmd`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (official installer, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (official `agent` install flow via `cursor.com/install`)
- **OpenCode** (`opencode-ai`)
- **Kimi Code** (`@moonshot-ai/kimi-code`, binary: `kimi`)
- **Qwen Code** (`@qwen-code/qwen-code`, binary: `qwen`)
- **MiniMax** (`mmx-cli`, binary: `mmx`)

### Standalone usage providers (usage-only, no CLI managed)

These are plan/account services with no installable CLI; `oct usage` reports them only when listed in `agent_order` or `enabled_tools` (e.g. `oct config set tools zai`).

- **Z.ai (GLM Coding Plan)** — `ZAI_API_KEY` / `ZHIPU_API_KEY`, or an OpenCode `zai-coding-plan` login
- **DeepSeek** — `DEEPSEEK_API_KEY`
- **OpenRouter** — `OPENROUTER_API_KEY`
- **Grok (xAI SuperGrok)** — `grok login` credential or `GROK_OAUTH_TOKEN`

## Menubar helper (macOS)

The Swift menubar helper can be built and installed separately.

In the menubar app's Settings, General is merged into the Configuration screen. Its alert section exposes only safe global alert settings; it does not expose provider-specific thresholds or snooze controls. Use `oct alert config ...` and `oct alert snooze ...` for those advanced controls.

```bash
oct menubar                # run the menu bar app
oct menubar doctor         # inspect helper resolution / launch mode
oct menubar build-helper   # build the Swift helper
oct menubar install-helper # install to ~/.local/bin/OctMenubarApp
```

## Manager Support Matrix

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

## Requirements

- **Users**: Homebrew for agent-update support on macOS (Linux/Windows get the CLI features only)
- **Developers (build/test from source)**: **Go >= 1.25**

## Release

- Primary binary distribution: GitHub Releases + `scripts/install.sh`
- Local release wrapper: `bash scripts/release-package.sh vX.Y.Z`

## License

MIT © Suho Han
