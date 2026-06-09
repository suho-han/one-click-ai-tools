# one-click-tools (oct)

**one-click-tools (oct)** is a CLI utility to bootstrap, update, and track usage for popular AI developer tools with a single command.

## Supported AI Agents
- **Claude Code** (`@anthropic-ai/claude-code`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (`@sanchaymittal/antigravity-cli`, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (official `agent` install flow via `cursor.com/install`)
- **OpenCode** (`opencode-ai`)

## Installation

### Via npm
```bash
npm install -g one-click-tools
```

### Via pnpm
```bash
pnpm add -g one-click-tools
```

## Quick Start

Use the `oct` command to manage your tools:

- `oct agent-update`: Update or install all supported AI tools.
- `oct session-refresh`: Probe local auth/session state without sending prompts.
- `oct usage`: View consolidated usage statistics.
- `oct schedule --task session-refresh enable`: Run token-free session probes periodically.
- `oct update`: Update `oct` to the latest version.

## Requirements
- **Node.js/npm** or **pnpm** (All platforms)
- **Homebrew** (macOS)
