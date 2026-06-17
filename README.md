# one-click-tools (oct)

**one-click-tools (oct)** is a CLI utility to bootstrap, update, and inspect popular AI developer tools from one command.

## Supported AI Agents
- **Claude Code** (`@anthropic-ai/claude-code`)
- **OpenAI Codex** (`@openai/codex`)
- **Antigravity CLI** (official installer, binary: `agy`)
- **GitHub Copilot** (`@github/copilot`)
- **Cursor CLI** (official `agent` install flow via `cursor.com/install`)
- **OpenCode** (`opencode-ai`)

## Installation

### Via npm
```bash
npm install -g one-click-tools
```

공식 패키지 릴리스 경로는 `npm publish` 입니다.

Install-time note:
- In interactive terminals, `postinstall` asks whether to enable periodic token-free `session-refresh`.
- Defaults written to config: disabled, `daily`, `09:00`.

### Via pnpm
```bash
pnpm add -g one-click-tools
```

공식 패키지 릴리스 경로는 `npm publish` 입니다.

## Quick Start

자주 쓰는 흐름만 먼저 보면:

```bash
# 전체 agent 업데이트
oct agent-update

# 실행 없이 update plan만 확인
oct agent-update --dry-run --explain

# 토큰 없이 세션 상태 probe
oct session-refresh --dry-run

# usage 확인
oct usage

# release 전 점검
oct release-doctor

# shell PATH/bootstrap 점검
oct doctor shell
```

## Menubar helper (macOS)

Swift menubar helper를 따로 빌드/설치할 수 있습니다.

```bash
# helper 탐색/launch 상태 점검
oct menubar doctor

# Swift helper build
oct menubar build-helper

# ~/.local/bin/OctMenubarApp 로 설치
oct menubar install-helper
```

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

## Requirements
- **Node.js/npm** or **pnpm** (All platforms)
- **Homebrew** (macOS)
- **Go >= 1.25** (source build/test)

## Release
- dependency management: `pnpm`
- official package publish: `npm` via GitHub Actions `goreleaser` workflow
- local release wrapper: `npm run release:npm`

### Release preflight
```bash
# local version vs npm registry
node -p "require('./package.json').version"
npm view one-click-tools version --registry=https://registry.npmjs.org/

# local npm auth / registry reachability
npm whoami
npm ping --registry=https://registry.npmjs.org/

# release integrity + Go validation
bash scripts/verify-release-integrity.sh
GOTOOLCHAIN=auto go test ./...
GOTOOLCHAIN=auto go build ./...
```

### Publish lanes
1. Canonical release path
   - `npm run release:npm`
   - the wrapper bumps/tag/pushes locally, then waits for GitHub Actions `goreleaser` to publish with `NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}`
2. Manual CI rerun
   - GitHub Actions → `goreleaser` → `Run workflow`
   - `release_mode=release`
   - `git_ref=vX.Y.Z`

### Notes
- `npm run release:npm -- --help` is **not** a safe help path; it still enters the release wrapper.
- Local `npm whoami` can still be useful as preflight, but the package publish itself is CI-driven.
- If local npm auth is broken, separate `E401` (local auth failure) from CI publish wiring.
