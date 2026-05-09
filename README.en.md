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

### Basic Usage

```bash
# Update all AI agents (Claude, Codex, Gemini, Copilot, Cursor, OpenCode)
oct agent-update

# Check AI tool usage/quota
oct usage

# Show help
oct help
```

## ✅ Common Commands

These 4 command groups cover most use cases.

### 1) Update agents

```bash
# Update Claude/Codex/Gemini/Copilot/Cursor/OpenCode
oct agent-update
```

### 2) Check usage

```bash
# Check current usage
oct usage
```

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
- **Gemini CLI** (`@google/gemini-cli`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor** (`cursor-agent`)
- **OpenCode** (`opencode-ai`)

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
