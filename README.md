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

## Beta Channel (Other Device)

To install or update to the latest beta release on another machine, use:

```bash
npm install -g one-click-tools@beta
```

Then verify the installed version:

```bash
one-click-tools --version
# or
oct --version
```

## Usage

You can use the full command `one-click-tools` or the shorthand `oct`.

### 1. Update `oct` itself

Update to the latest stable version:

```bash
oct update
```

Update to the latest beta version:

```bash
oct update --beta
```

### 2. Update all AI agents

This command will check and update all supported AI CLI tools.

- On `macOS`: runs `brew update` and `brew upgrade`, then updates/installs agent CLIs via npm.
- On `macOS`, if an npm install/update step fails for a tool, it retries via Homebrew formula/cask upgrade when available.
- On `Ubuntu`: updates/installs agent CLIs via npm.
- If a target command (`claude`, `codex`, `gemini`, `copilot`) already exists but is not npm-managed, it attempts a non-npm update path first (Homebrew formula/cask upgrade or tool self-update when available).
- For GitHub Copilot specifically, non-npm installs are first realigned to npm by running `npm install -g --force @github/copilot`.
- Execution logs are saved to `~/.oct/logs/agent-update-YYYYMMDD-HHMMSS.log`, and the final log path is printed when the command exits.

```bash
oct agent-update
```

### 3. Show description and help

Running the command without arguments or with `help` will show the tool description.

```bash
oct
# or
oct help
```

### 4. Show integrated usage (`codex` / `claude-code` / `gemini` / `copilot`)

Collects usage in one command. `codex`/`claude-code`/`gemini` use `API first -> CLI fallback`, and `copilot` uses the official GitHub Billing Usage REST API.

```bash
oct usage
```

For machine-readable output:

```bash
oct usage --json
```

Experimental mode (opt-in) using local OAuth/session state inspired by `codex-opero`:

```bash
oct usage --experimental-oauth-usage
oct usage --experimental-oauth-usage --json
```

Optional API endpoint environment variables for API-first mode:

- `OCT_CODEX_USAGE_ENDPOINT`
- `OCT_CLAUDE_USAGE_ENDPOINT`
- `OCT_GEMINI_USAGE_ENDPOINT`
- `OCT_COPILOT_USAGE_ENDPOINT`
- `OCT_COPILOT_USAGE_YEAR`
- `OCT_COPILOT_USAGE_MONTH`
- `OCT_COPILOT_USAGE_DAY`
- `OCT_COPILOT_USAGE_MODEL`
- `OCT_COPILOT_USAGE_PRODUCT`

Copilot endpoint auto-resolution (when `OCT_COPILOT_USAGE_ENDPOINT` is not set):

- enterprise billing path: set `OCT_GITHUB_ENTERPRISE` (or `GITHUB_ENTERPRISE`)
- org billing path: set `OCT_GITHUB_ORG` (or `GITHUB_ORG`)
- user billing path: set `OCT_GITHUB_USER` (or `GITHUB_USER`)
- if unset, `gh auth token` + `GET /user` login lookup is attempted for the user path

Resolved endpoints:

- `https://api.github.com/organizations/{org}/settings/billing/premium_request/usage`
- `https://api.github.com/users/{username}/settings/billing/premium_request/usage`
- `https://api.github.com/enterprises/{enterprise}/settings/billing/premium_request/usage`

Additional endpoint for experimental Gemini OAuth path:

- `OCT_GEMINI_API_ENDPOINT`

## Requirements

- **macOS**: [Homebrew](https://brew.sh/) and [Node.js/npm](https://nodejs.org/)
- **Ubuntu**: [Node.js/npm](https://nodejs.org/)

## Notes

- On Ubuntu, global npm operations may require `sudo` permissions.
- The script will automatically attempt to use `sudo` if a permission error is detected during npm operations.
- On macOS, Homebrew upgrade failures are treated as warnings so agent-level npm updates can still proceed.
