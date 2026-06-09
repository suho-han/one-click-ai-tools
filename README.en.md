# one-click-tools (oct)

[![npm version](https://img.shields.io/npm/v/one-click-tools.svg?style=flat-square)](https://www.npmjs.com/package/one-click-tools)
[![pnpm](https://img.shields.io/badge/maintained%20with-pnpm-cc00ff.svg?style=flat-square&logo=pnpm)](https://pnpm.io/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

[한국어](README.md)

**one-click-tools (oct)** is a high-performance CLI for installing and updating popular AI developer tools in one command.

## 🚀 Quick Start

### Installation

```bash
# Install via npm
npm install -g one-click-tools

# Install via pnpm
pnpm add -g one-click-tools
```

Install-time note:
- In interactive terminals, `postinstall` asks whether to enable periodic token-free `session-refresh`.
- Defaults written to config: disabled, `daily`, `09:00`.

### Basic Usage

```bash
# Update all AI agents (Claude, Codex, Antigravity, Copilot, Cursor, OpenCode)
oct agent-update

# Probe local auth/session state without sending prompts
oct session-refresh --dry-run

# Check AI tool usage/quota
oct usage

# Show help
oct help
```

## ✅ Common Commands

These 4 command groups cover most use cases.

### 1) Update agents

```bash
# Update Claude/Codex/Antigravity/Copilot/Cursor/OpenCode
oct agent-update
```

### 1-1) Token-free session refresh

```bash
# Dry-run token-free probes
oct session-refresh --dry-run

# Schedule periodic session refresh
oct schedule enable --task session-refresh --interval daily --hour 9
```

### 2) Check usage

```bash
# Check current usage
oct usage
```

### 2-1) Interactive keys for `oct config`

```bash
oct config
```

- `↑/↓`: move cursor
- `Enter`: toggle current row
- `Enter` on `Choose all / Choose none`: toggle all rows
- `Enter` on final `Confirm` row: save and exit
- `Ctrl+C` / `Ctrl+Q` / `q`: cancel

Environment-variable overrides now require the `OCT_` prefix.
- Example: `OCT_ENABLED_TOOLS=codex,agy`
- Non-prefixed vars like `ENABLED_TOOLS` are ignored.
- Legacy config values `gemini` and `gemini-cli` are still accepted and normalized to `agy`.

### 3) Always-on monitoring

```bash
# Refresh every 10 seconds
oct monitor --interval 10s

# Run once
oct monitor --once

# Show top 5 by highest usage
oct monitor --once --sort-by used --desc --top 5 --compact
```

### 4) Usage alert configuration

```bash
# Show current alert config
oct alert config show

# Enable alerts / set base config
oct alert config set enabled true
oct alert config set cooldown_minutes 120
oct alert config set threshold_percent 85
oct alert config set critical_percent 98
oct alert config set quiet_hours 00:00-08:00
oct alert config set timezone Asia/Seoul
```

What each attribute does:
- `enabled`: turn usage alerts on/off
- `cooldown_minutes`: minimum resend interval (minutes) for the same provider/window
- `threshold_percent`: default warning threshold (%)
- `critical_percent`: CRITICAL threshold (%), overrides quiet hours/snooze
- `quiet_hours`: mute window for non-critical alerts (`HH:MM-HH:MM`)
- `timezone`: timezone used for quiet-hours evaluation (e.g. `Asia/Seoul`)

#### Detailed thresholds (window/provider)

```bash
# Global thresholds by window
oct alert config set threshold.5h 90
oct alert config set threshold.7d 92

# Provider-specific thresholds
oct alert config set provider.codex.5h 94
oct alert config set provider.codex.default 88
oct alert config set provider.cursor.5h 93
oct alert config set provider.opencode.default 87
```

#### Snooze

```bash
# Snooze all alerts for 2 hours
oct alert snooze set --duration 2h

# Snooze specific provider/window
oct alert snooze set --duration 1h --provider codex --window 5h
oct alert snooze set --duration 1h --provider cursor --window 5h
oct alert snooze set --duration 1h --provider opencode --window 7d

# Show/clear snooze
oct alert snooze show
oct alert snooze clear --provider codex --window 5h
```

## 🛠 Supported Agents

- **Claude Code** (`@anthropic-ai/claude-code`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (official installer, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (official `agent` install flow via `cursor.com/install`)
- **OpenCode** (`opencode-ai`)

## 🧭 Manager Support Matrix

| Manager | Detection strategy | Install path | Built-in use |
| --- | --- | --- | --- |
| `brew` | binary under `brew --prefix` / `brew list` | `brew upgrade <formula>` | Claude, Cursor, OpenCode, Codex when Homebrew-owned |
| `npm` | `npm prefix -g` / `npm list -g` | `npm install -g <package>` | default fallback for Claude, OpenCode, Codex, Copilot |
| `pnpm` | `pnpm bin -g` / `pnpm list -g` | `pnpm add -g <package>` | provenance-based detection only |
| `yarn` | `yarn global bin` / `yarn global list` | `yarn global add <package>` | provenance-based detection only |
| `cargo` | `cargo:` package prefix / cargo bin path | `cargo install <crate> --locked` | explicit package override |
| `go-install` | `go:` package prefix / `go env GOPATH` bin path | `go install <package>@latest` | explicit package override |
| `pip` | `pip:` package prefix / `python3 -m site --user-base` bin path | `python3 -m pip install --upgrade <package>` | explicit package override |
| `cursor-agent` | tool identity (`cursor-agent` / `cursor` / `agent`) | `curl https://cursor.com/install -fsS \| bash` | Cursor CLI |
| `antigravity-installer` | tool identity (`agy` / `antigravity`, legacy `gemini*`) | `curl -fsSL https://antigravity.google/cli/install.sh \| bash` | Antigravity CLI |

The built-in support matrix is regression-tested in `internal/update/manager_test.go` so manager fallback changes stay explicit.

## 📖 Documentation

For English documentation, start here:

- [CONTEXT Docs Index](CONTEXT/README.en.md)
- [Detailed Usage Guide](CONTEXT/en/USAGE.md)
- [Usage Alerts](CONTEXT/en/USAGE_ALERTS.md)
- [Always-on Monitoring](CONTEXT/en/MONITORING.md)
- [Local Development & Testing](CONTEXT/en/LOCAL_TEST.md)

## Requirements

- **Runtime users**
  - **macOS**: Homebrew and Node.js/npm
  - **Ubuntu/Linux**: Node.js/npm
  - **Windows**: Node.js/npm (Experimental)
- **Developers (build/test from source)**
  - **Go >= 1.25**

## License

MIT © Suho Han
