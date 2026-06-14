# Provider Plan Detection Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** `oct usage`/menubar가 각 provider의 현재 usage뿐 아니라 "어떤 plan/tier로 해석했는지"를 함께 보여주도록 만든다.

**Architecture:** provider별 usage fetcher에 plan detection을 붙이고, 공통 `UsageResult`에 `plan`/`plan_source`를 추가한다. plan은 1) 로컬 auth artifact/JWT claim, 2) provider CLI status/about 출력, 3) 이미 호출 중인 공식 API 응답 순으로 탐지하고, 근거가 없으면 `unknown`으로 명시한다.

**Tech Stack:** Go 1.25, Cobra, local auth/config files, JWT payload parsing, existing provider CLIs, Swift menubar JSON consumer

---

## Repository observations

- 현재 `internal/usage/*` fetcher들은 usage만 수집하고 plan/tier 필드는 전혀 없다.
- `UsageResult` compact JSON은 menubar Swift helper가 그대로 decode 하므로, plan 필드를 추가하면 CLI/menubar 양쪽에 동시에 노출할 수 있다.
- 현재 host에서 live probe로 확인된 신호:
  - `codex login status` → `Logged in using ChatGPT`
  - `~/.codex/auth.json` JWT claim → `https://api.openai.com/auth.chatgpt_plan_type = plus`
  - `cursor-agent about` → `Subscription Tier   Unknown`
  - `claude auth status` → login state only, plan field 없음
  - `gh api user` → `plan` field 없음
  - `agy quota` → explicit tier는 안 주고 `Antigravity app Settings > Model`로 안내
- 따라서 provider별 detection quality는 다르며, 일괄적인 API 하나로 해결되지 않는다.

## Provider-specific detection strategy

### Codex
- Primary: `~/.codex/auth.json` 안의 JWT payload claim
- Verified signal: `https://api.openai.com/auth.chatgpt_plan_type`
- Fallback: `codex login status`는 로그인 방식만 알려주므로 보조 힌트로만 취급

### Cursor
- Primary: `cursor-agent about`의 `Subscription Tier` line
- Fallback: `auth.json` access token이 JWT일 경우 generic plan/tier claim scan
- Current host probe result: `Unknown`

### Claude Code
- Primary: existing auth token source (keychain / `~/.claude/.credentials.json` / `CLAUDE_API_TOKEN`)가 JWT면 generic plan/tier claim scan
- Fallback: `claude auth status`는 로그인 여부만 보여주므로 tier 미노출 시 `unknown`

### GitHub Copilot
- Primary: current official usage/billing integration 유지
- Current limitation: current `/user` probe와 existing billing endpoint에서 plan field를 확인하지 못함
- Current action: `unknown`으로 명시하고 근거를 `plan_source`에 남김

### Antigravity
- Primary: explicit tier API/CLI 부재
- Current action: `unknown`; `agy quota`의 app settings guidance를 source note로 보존

### OpenCode
- Primary: local session logs only
- Current action: local logs에는 account/subscription tier가 없으므로 `unknown`

---

## Execution tasks

### Task 1: 공통 schema 추가
- Modify: `internal/usage/usage.go`
- Add: `plan`, `plan_source` to `UsageResult` and compact JSON
- Surface in table and downstream consumers

### Task 2: 공통 plan detection helper 추가
- Create: `internal/usage/plan.go`
- Create: `internal/usage/plan_test.go`
- Include:
  - JWT payload decode helper
  - generic plan/tier claim scan
  - `cursor-agent about` parser
  - provider-specific plan detection wrappers

### Task 3: provider fetchers wiring
- Modify: `internal/usage/claude.go`
- Modify: `internal/usage/codex.go`
- Modify: `internal/usage/cursor.go`
- Modify: `internal/usage/copilot.go`
- Modify: `internal/usage/opencode.go`
- Modify: `internal/usage/gemini.go`
- Set explicit `unknown` when plan is not derivable

### Task 4: menubar / JSON consumer sync
- Modify: `cmd/menubar_state.go`
- Modify: `macos/OctMenubar/Sources/OctMenubarApp/Models/UsageSnapshot.swift`
- Show known plan in provider note/details without reintroducing card bloat

### Task 5: validation
- Run targeted tests for usage/menubar
- Run `go build -o oct main.go`
- Swift compile remains pending on macOS host if touched

---

## Exit criteria

- each provider result includes explicit `plan` value (`plus`, `unknown`, etc.)
- when a plan is detected, `plan_source` explains the evidence path
- Codex plan is detected from local auth JWT on this host
- Cursor/Claude/Copilot/Antigravity/OpenCode unknown states are explicit instead of silent omission
- CLI JSON and menubar details can surface plan information
