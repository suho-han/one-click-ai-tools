# one-click-tools (oct)

[![npm version](https://img.shields.io/npm/v/one-click-tools.svg?style=flat-square)](https://www.npmjs.com/package/one-click-tools)
[![npm downloads](https://img.shields.io/npm/dm/one-click-tools.svg?style=flat-square)](https://www.npmjs.com/package/one-click-tools)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

**one-click-tools (oct)** is an OS-aware CLI utility to bootstrap and update popular AI developer tools with a single command.

## Key Features

- **Multi-OS Support**: Optimized for both `macOS` and `Ubuntu`.
- **Smart Updates**: Detects if a tool is missing (installs it) or already present (updates it).
- **macOS Brew Maintenance**: Runs `brew update` and `brew upgrade` before agent-level updates.
- **Collision-Safe Install Flow**: If a CLI binary already exists from a non-npm install, skips npm install to avoid `EEXIST` errors.
- **Consolidated Workflow**: No need to remember individual update commands for different agents.

## Supported AI Agents

- **Claude Code** (`@anthropic-ai/claude-code`)
- **OpenAI Codex** (`@openai/codex`)
- **Gemini CLI** (`@google/gemini-cli`)
- **GitHub Copilot** (`@github/copilot`)

## Installation

Install globally via npm:

```bash
npm install -g one-click-tools
```

## Usage

You can use the full command `one-click-tools` or the shorthand `oct`.

### 1. Update all AI agents

This command will check and update all supported AI CLI tools.

- On `macOS`: runs `brew update` and `brew upgrade`, then updates/installs agent CLIs via npm.
- On `Ubuntu`: updates/installs agent CLIs via npm.
- If a target command (`claude`, `codex`, `gemini`, `copilot`) already exists but is not npm-managed, it attempts a non-npm update path first (Homebrew formula/cask upgrade or tool self-update when available).
- For GitHub Copilot specifically, non-npm installs are first realigned to npm by running `npm install -g --force @github/copilot`.
- Execution logs are saved to `~/.oct/logs/agent-update-YYYYMMDD-HHMMSS.log`, and the final log path is printed when the command exits.

```bash
oct agent-update
```

### 2. Show description and help

Running the command without arguments or with `help` will show the tool description.

```bash
oct
# or
oct help
```

## Requirements

- **macOS**: [Homebrew](https://brew.sh/) and [Node.js/npm](https://nodejs.org/)
- **Ubuntu**: [Node.js/npm](https://nodejs.org/)

## Notes

- On Ubuntu, global npm operations may require `sudo` permissions.
- The script will automatically attempt to use `sudo` if a permission error is detected during npm operations.
- On macOS, Homebrew upgrade failures are treated as warnings so agent-level npm updates can still proceed.
