# one-click-tools (oct)

**one-click-tools (oct)** is a CLI utility to bootstrap, update, and track usage for popular AI developer tools with a single command.

## Supported AI Agents
- **Claude Code** (`@anthropic-ai/claude-code`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (official installer, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (official `agent` install flow via `cursor.com/install`)
- **OpenCode** (`opencode-ai`)

## Manager Support Matrix

| Manager | 감지 기준 | 설치 경로 | built-in 사용처 |
| --- | --- | --- | --- |
| `brew` | `brew --prefix` 하위 binary 또는 `brew list` | `brew upgrade <formula>` | Homebrew로 설치된 Claude/Cursor/OpenCode/Codex |
| `npm` | `npm prefix -g` 또는 `npm list -g` | `npm install -g <package>` | Claude/OpenCode/Codex/Copilot 기본 fallback |
| `pnpm` | `pnpm bin -g` 또는 `pnpm list -g` | `pnpm add -g <package>` | provenance 기반 감지만 지원 |
| `yarn` | `yarn global bin` 또는 `yarn global list` | `yarn global add <package>` | provenance 기반 감지만 지원 |
| `cargo` | `cargo:` package prefix 또는 cargo bin path | `cargo install <crate> --locked` | explicit package override |
| `go-install` | `go:` package prefix 또는 `go env GOPATH` bin path | `go install <package>@latest` | explicit package override |
| `pip` | `pip:` package prefix 또는 `python3 -m site --user-base` bin path | `python3 -m pip install --upgrade <package>` | explicit package override |
| `cursor-agent` | tool identity (`cursor-agent` / `cursor` / `agent`) | `curl https://cursor.com/install -fsS \| bash` | Cursor CLI |
| `antigravity-installer` | tool identity (`agy` / `antigravity`, legacy `gemini*`) | `curl -fsSL https://antigravity.google/cli/install.sh \| bash` | Antigravity CLI |

이 built-in support matrix는 `internal/update/manager_test.go`에서 회귀 테스트로 고정합니다.

## Installation

### Via npm
```bash
npm install -g one-click-tools
```

Install-time note:
- In interactive terminals, `postinstall` asks whether to enable periodic token-free `session-refresh`.
- Defaults written to config: disabled, `daily`, `09:00`.

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
