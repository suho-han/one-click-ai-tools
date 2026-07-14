# Antigravity 전환 + Token-free Session Refresh Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** deprecated된 Gemini CLI 지원을 제거하고 Antigravity CLI 지원으로 전환하면서, 각 agent의 세션을 토큰 소모 없이 최신화/활성 상태로 유지하기 위한 별도 session refresh 기능을 추가한다.

**Architecture:** 기존 `internal/update`의 provider registry, `internal/usage`의 provider fetcher, `cmd/*`의 UX를 분리 유지한다. 전환 작업은 1) provider identity 교체, 2) usage/auth/session capability 정리, 3) token-free session refresh 명령 추가, 4) 문서/테스트 동기화 순서로 진행한다. 불확실한 외부 CLI 세부사항은 먼저 capability probe로 검증하고, 토큰 소모 가능성이 있는 네트워크/모델 호출은 session refresh 경로에서 금지한다.

**Tech Stack:** Go 1.25, Cobra, Viper, npm, local CLI probing, go test, go build

---

## Current repository observations

- 현재 registry에는 `Gemini CLI`가 `@google/gemini-cli` / `gemini` / `gemini-cli`로 하드코딩되어 있다. (`internal/update/tools.go`)
- 사용량 수집 코드는 `internal/usage/gemini.go`에 있으나, local fallback 경로가 이미 `~/.gemini/antigravity/conversations`를 읽고 있어 내부적으로는 Antigravity 흔적이 존재한다.
- 설정/문서/테스트 전반에 `gemini` 문자열이 넓게 퍼져 있다. (`cmd/config.go`, `cmd/usage.go`, `cmd/alert.go`, `README*`, `CONTEXT/*`, 각종 테스트)
- npm 확인 결과 plain `antigravity-cli` 패키지는 현재 `0.0.1` placeholder이며 bin도 `kirox`라서 채택 대상이 아니다.
- npm 확인 결과 `@sanchaymittal/antigravity-cli@0.1.1` 는 실제 CLI 설명과 `ag` bin을 제공한다.
- 현재 설치된 로컬 바이너리 중 Codex만 확인 가능했고, `codex login status` / `codex doctor` 같은 non-token auth-health 명령이 존재한다. 다른 provider는 이 호스트에서 미설치라 capability는 구현 단계에서 probe 기반으로 검증해야 한다.

## Scope

- Gemini 제거 및 Antigravity 도입
- provider naming / help text / auth guidance / usage source 설명 정리
- token-free session refresh 명령 신설
- provider별 session refresh capability matrix 도입
- 문서/테스트/계획 문서 최신화

## Non-goals (이번 작업에서 바로 하지 않음)

- session refresh에서 실제 모델 요청/토큰 차감 발생 가능성이 있는 prompt 전송
- 검증되지 않은 외부 Antigravity API 호출 추가
- provider별 원격 quota/usage 프로토콜 대규모 재설계

---

### Task 1: Antigravity 대상 패키지/bin/capability 기준 확정

**Objective:** placeholder 패키지 오인 도입을 막고 코드에 넣을 canonical provider identity를 고정한다.

**Files:**
- Modify: `docs/plans/2026-06-09-antigravity-session-refresh-plan.md`
- Modify: `CONTEXT/ko/TODO.md`
- (Optional) Modify: `CONTEXT/en/TODO.md`

**Step 1: Confirm install target**

Record the decision explicitly:

```text
Provider display name: Antigravity CLI
npm package candidate: @sanchaymittal/antigravity-cli
binary candidate: ag
Do not use: antigravity-cli (placeholder, bin=kirox)
```

**Step 2: Define canonical internal naming**

Use one internal slug consistently:

```text
binary / slug: antigravity
display: Antigravity CLI
legacy alias accepted temporarily: gemini
```

**Step 3: Define migration rule**

During migration, accept old config values and map them to the new provider:

```text
agent_order: gemini -> antigravity
enabled_tools: gemini -> antigravity
provider thresholds / alert keys: gemini -> antigravity
```

**Step 4: Verification note**

Run:

```bash
npm view @sanchaymittal/antigravity-cli name version bin description
npm view antigravity-cli name version bin description
```

Expected:
- scoped package exposes `ag`
- unscoped package is clearly placeholder

**Step 5: Commit**

```bash
git add docs/plans/2026-06-09-antigravity-session-refresh-plan.md CONTEXT/ko/TODO.md
git commit -m "docs: add antigravity migration and session refresh plan"
```

---

### Task 2: Provider registry에 Antigravity 추가하고 Gemini alias migration 추가

**Objective:** 설치/선택/정렬/표시 계층에서 provider identity를 Gemini에서 Antigravity로 전환한다.

**Files:**
- Modify: `internal/update/tools.go`
- Modify: `internal/update/update_test.go`
- Modify: `internal/usage/usage.go`
- Modify: `internal/usage/usage_test.go`
- Modify: `cmd/root_test.go`
- Modify: `cmd/usage_test.go`
- Modify: `cmd/config_test.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing tests for alias migration**

Example expectations:

```go
func TestGetFilteredTools_MigratesLegacyGeminiToAntigravity(t *testing.T) {
    ordered := GetOrderedTools([]string{"antigravity", "codex"})
    result := GetFilteredTools([]string{"gemini"}, ordered)
    if len(result) != 1 || result[0].BinaryName != "antigravity" {
        t.Fatalf("expected legacy gemini to resolve to antigravity, got %#v", result)
    }
}
```

**Step 2: Replace registry entry**

Target structure in `internal/update/tools.go`:

```go
{
    Name:       "Antigravity CLI",
    Package:    "@sanchaymittal/antigravity-cli",
    BinaryName: "antigravity",
    Icon:       "🪂",
    LobeIcon:   "GeminiCLI",
    HexColor:   "#4285F4",
}
```

Notes:
- `BrewPackage`는 검증되기 전까지 비워두거나 npm-only provider로 취급.
- `LobeIcon`은 신규 아이콘이 없다면 임시로 기존 asset alias를 유지.

**Step 3: Add legacy alias normalizer**

Introduce a small normalizer before filtering/ordering:

```go
func normalizeToolAlias(name string) string {
    switch strings.ToLower(strings.TrimSpace(name)) {
    case "gemini", "gemini-cli", "ag":
        return "antigravity"
    default:
        return strings.ToLower(strings.TrimSpace(name))
    }
}
```

**Step 4: Update default ordering**

In `internal/usage/usage.go`, change defaults from:

```go
[]string{"gemini", "claude", "cursor-agent", "copilot", "opencode", "codex"}
```

to:

```go
[]string{"antigravity", "claude", "cursor-agent", "copilot", "opencode", "codex"}
```

**Step 5: Run targeted tests**

Run:

```bash
GOTOOLCHAIN=auto go test ./internal/update ./internal/usage ./cmd ./internal/config -run 'Antigravity|Gemini|Tool|Order|Config' -v
```

Expected:
- legacy `gemini` config values still resolve
- newly saved values prefer `antigravity`

**Step 6: Commit**

```bash
git add internal/update/tools.go internal/update/update_test.go internal/usage/usage.go internal/usage/usage_test.go cmd/root_test.go cmd/usage_test.go cmd/config_test.go internal/config/config_test.go
git commit -m "feat: migrate gemini provider identity to antigravity"
```

---

### Task 3: usage/auth/help text를 Antigravity 기준으로 전환

**Objective:** 사용자-facing 문구와 usage fetcher naming을 현재 provider reality에 맞춘다.

**Files:**
- Modify: `internal/usage/gemini.go`
- (Optional, later) Create: `internal/usage/antigravity.go`
- Modify: `internal/usage/gemini_test.go`
- Modify: `cmd/usage.go`
- Modify: `cmd/config.go`
- Modify: `cmd/monitor.go`
- Modify: `cmd/alert.go`

**Step 1: Write failing tests for provider labels**

Examples:

```go
func TestProviderOptions_UsesAntigravityInsteadOfGemini(t *testing.T) {}
func TestSelectedTools_DefaultOrderIncludesAntigravity(t *testing.T) {}
```

**Step 2: Convert fetcher labeling first, file rename later**

Keep diff minimal initially:
- leave filename as `gemini.go` for one commit if needed
- change returned `Provider` values and help text to `antigravity`
- add compatibility comments that this file is the legacy Gemini implementation path being repurposed

**Step 3: Update auth guidance**

Replace messages like:

```text
Run 'gemini' once and complete browser sign-in
```

with explicit Antigravity guidance:

```text
Run 'ag' once and complete browser sign-in
```

If verified command differs during implementation, update all docs/tests in same commit.

**Step 4: Preserve local fallback semantics**

The local fallback already reads Antigravity-style conversation storage:

```go
filepath.Join(home, ".gemini", "antigravity", "conversations")
```

During migration, support both:
- old Gemini-local path(s)
- confirmed Antigravity session/auth path(s)

If actual `ag` storage differs, add a multi-path collector instead of replacing the old path immediately.

**Step 5: Run targeted tests**

Run:

```bash
GOTOOLCHAIN=auto go test ./internal/usage ./cmd -run 'Antigravity|Usage|Alert|Monitor|Config' -v
```

Expected:
- help text no longer advertises deprecated Gemini CLI
- alert provider options expose `antigravity`

**Step 6: Commit**

```bash
git add internal/usage/gemini.go internal/usage/gemini_test.go cmd/usage.go cmd/config.go cmd/monitor.go cmd/alert.go
git commit -m "feat: switch usage and auth flows to antigravity labels"
```

---

### Task 4: token-free session refresh capability matrix 추가

**Objective:** 세션 활성화 기능이 토큰 소모 경로를 절대 타지 않도록 provider별 capability와 안전 제약을 명문화한다.

**Files:**
- Create: `internal/sessionrefresh/sessionrefresh.go`
- Create: `internal/sessionrefresh/sessionrefresh_test.go`
- Modify: `internal/update/tools.go`
- Modify: `cmd/root.go`

**Step 1: Define refresh result model**

Example:

```go
type RefreshResult struct {
    Provider   string
    Supported  bool
    Mode       string // probe, auth-status, local-session-touch, daemon-start
    Status     string // ok, skipped, unsupported, error
    Message    string
    SourcePath string
}
```

**Step 2: Add explicit safety contract**

Document in code comments:
- no HTTP usage/quota calls
- no prompt submission
- no model execution
- no token-count event generation on purpose

**Step 3: Add provider capability registry**

Example initial matrix:

```go
var refreshers = map[string]Refresher{
    "antigravity": probeAntigravitySession,
    "claude":      probeClaudeSession,
    "codex":       probeCodexSession,
    "cursor-agent": probeCursorSession,
    "copilot":     probeCopilotSession,
    "opencode":    probeOpenCodeSession,
}
```

Important:
- if a provider has no verified token-free activation command, return `Supported=false` and explain why
- do not fake activation by performing a normal usage fetch

**Step 4: Implement smallest safe behavior first**

Safe first-pass behaviors:
- Codex: `codex login status` or `codex doctor --summary` probe if it does not emit token usage
- file-backed providers: verify auth/session artifacts and newest session timestamp
- unsupported providers: structured `unsupported` result

**Step 5: Write tests before integration**

Examples:

```go
func TestNormalizeProviderAlias_GeminiMapsToAntigravity(t *testing.T) {}
func TestRefreshResult_UnsupportedProviderDoesNotError(t *testing.T) {}
func TestSessionRefresh_NeverCallsUsageFetcher(t *testing.T) {}
```

**Step 6: Run tests**

Run:

```bash
GOTOOLCHAIN=auto go test ./internal/sessionrefresh -v
```

Expected:
- unsupported providers are reported cleanly
- no usage fetch functions are referenced in refresh path

**Step 7: Commit**

```bash
git add internal/sessionrefresh/sessionrefresh.go internal/sessionrefresh/sessionrefresh_test.go internal/update/tools.go cmd/root.go
git commit -m "feat: add token-free session refresh capability matrix"
```

---

### Task 5: `oct session-refresh` CLI 추가

**Objective:** 사용자가 설치된 agent 세션을 토큰 사용 없이 최신화/활성 점검할 수 있는 명령을 제공한다.

**Files:**
- Create: `cmd/session_refresh.go`
- Modify: `cmd/root.go`
- Modify: `cmd/root_test.go`
- Modify: `README.md`
- Modify: `README.en.md`

**Step 1: Write failing command tests**

Examples:

```go
func TestRootCommands_IncludeSessionRefresh(t *testing.T) {}
func TestSessionRefresh_JSONMode_EmitsStructuredResults(t *testing.T) {}
```

**Step 2: Add command surface**

Preferred shape:

```text
oct session-refresh
oct session-refresh --provider codex
oct session-refresh --json
oct session-refresh --dry-run
```

Flags:
- `--provider` repeatable or comma-separated
- `--json`
- `--dry-run` (report what would be probed)
- `--strict` (unsupported provider => non-zero exit)

**Step 3: Keep output operationally useful**

Human output example:

```text
provider      status        mode               message
codex         ok            auth-status        login status confirmed
antigravity   unsupported   probe              binary not installed on this host
opencode      ok            local-session      latest session file discovered
```

**Step 4: Verify no-token guarantee in UX**

Help text must state:

```text
This command does not fetch usage/quota and does not intentionally send prompts.
```

**Step 5: Run tests**

Run:

```bash
GOTOOLCHAIN=auto go test ./cmd -run 'SessionRefresh|Root' -v
```

Expected:
- command registered in help ordering
- JSON/human output stable

**Step 6: Commit**

```bash
git add cmd/session_refresh.go cmd/root.go cmd/root_test.go README.md README.en.md
git commit -m "feat: add token-free session refresh command"
```

---

### Task 6: provider별 세션 probe 구현 세부화

**Objective:** 각 provider별로 실제 안전한 session refresh 동작을 검증하고 점진적으로 지원 범위를 넓힌다.

**Files:**
- Modify: `internal/sessionrefresh/sessionrefresh.go`
- Modify: `internal/sessionrefresh/sessionrefresh_test.go`
- Modify: `CONTEXT/ko/USAGE.md`
- Modify: `CONTEXT/en/USAGE.md`

**Step 1: Codex support first**

Start with the only locally verifiable provider:

```bash
codex login status
codex doctor --summary --json
```

Use whichever path proves session/auth health without creating a chat request.

**Step 2: Antigravity support only after storage/CLI verification**

Probe in order:
1. binary exists (`ag`)
2. auth/session file exists
3. help/status subcommand exists
4. command completes without network/prompt side effects

If any step fails, return `unsupported` or `skipped`, not fake success.

**Step 3: Local-session providers**

For providers whose only safe no-token path is local filesystem inspection:
- find latest session file
- report its timestamp/path
- optionally update oct-owned cache/state, not provider-owned log files, unless provider docs explicitly allow touching session metadata

**Step 4: Run tests and manual smoke checks**

Run:

```bash
GOTOOLCHAIN=auto go test ./internal/sessionrefresh ./cmd -v
go run main.go session-refresh --json
```

Expected:
- per-provider result includes exact probe mode
- unsupported providers are transparent

**Step 5: Commit**

```bash
git add internal/sessionrefresh/sessionrefresh.go internal/sessionrefresh/sessionrefresh_test.go CONTEXT/ko/USAGE.md CONTEXT/en/USAGE.md
git commit -m "feat: add provider-specific session refresh probes"
```

---

### Task 7: 문서/컨텍스트/릴리즈 노트 동기화

**Objective:** deprecated Gemini 문구를 제거하고 Antigravity/session-refresh 동작을 모든 진입 문서에 반영한다.

**Files:**
- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `CONTEXT/ko/USAGE.md`
- Modify: `CONTEXT/en/USAGE.md`
- Modify: `CONTEXT/ko/ICONS.md`
- Modify: `CONTEXT/en/ICONS.md`
- Modify: `CONTEXT/ko/TODO.md`
- Modify: `CHANGELOG.md` (when implementation actually ships)
- Modify: `bin/README.md`
- Modify: `bin/README.en.md`

**Step 1: Replace provider lists**

Update all supported-agent lists from Gemini to Antigravity.

**Step 2: Add session-refresh usage examples**

```bash
oct session-refresh
oct session-refresh --provider codex --json
```

**Step 3: Document migration behavior**

Explicitly state:
- old `gemini` config values are auto-mapped
- deprecated provider name will be removed after transition window
- session-refresh is best-effort and token-free, not quota refresh

**Step 4: Run repo-wide verification**

Run:

```bash
GOTOOLCHAIN=auto go test ./...
go build -o oct main.go
```

Expected:
- tests pass
- binary builds
- no user docs still recommend deprecated Gemini CLI unless intentionally noted in migration section

**Step 5: Commit**

```bash
git add README.md README.en.md CONTEXT/ko/USAGE.md CONTEXT/en/USAGE.md CONTEXT/ko/ICONS.md CONTEXT/en/ICONS.md CONTEXT/ko/TODO.md CHANGELOG.md bin/README.md bin/README.en.md
git commit -m "docs: document antigravity migration and session refresh"
```

---

## Risks / open questions

1. **Antigravity package ambiguity**
   - `antigravity-cli` is placeholder.
   - `@sanchaymittal/antigravity-cli` looks real, but long-term maintenance and install UX should be rechecked before release.

2. **Binary naming mismatch**
   - package exposes `ag`, not `antigravity`.
   - internal slug can still be `antigravity`, but install/exec logic must map to real binary name deliberately.

3. **Session refresh semantics differ by provider**
   - some providers may support auth-status probes
   - others only allow local-session inspection
   - “latest화” should therefore be documented as **best-effort session activation/health probe**, not guaranteed upstream keepalive

4. **Do not silently consume tokens**
   - any command that might create a real prompt/event must be excluded from refresh mode

5. **File rename timing**
   - `internal/usage/gemini.go` can stay in place initially for minimal diffs, then rename in a later cleanup commit once behavior stabilizes

---

## Recommended commit order

1. `docs`: add migration/session-refresh plan
2. `feat`: migrate provider identity from gemini to antigravity
3. `feat`: switch user-facing usage/auth/help text to antigravity
4. `feat`: add token-free session refresh capability matrix
5. `feat`: add `oct session-refresh`
6. `docs`: sync README/CONTEXT/bin docs

---

## Done Criteria

- [ ] deprecated Gemini CLI is removed from supported provider lists and help text
- [ ] Antigravity is the canonical provider identity in code and docs
- [ ] legacy `gemini` config values migrate cleanly
- [ ] `oct session-refresh` exists and is explicitly token-free
- [ ] provider-specific refresh results are transparent (`ok/skipped/unsupported/error`)
- [ ] `GOTOOLCHAIN=auto go test ./...` passes
- [ ] `go build -o oct main.go` passes
