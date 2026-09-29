# Menubar Improvement Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** `oct menubar`를 단순 launcher에서 상태형 macOS status item으로 끌어올려, usage 상태를 즉시 보여주고 refresh/action 진입을 더 짧게 만든다.

**Architecture:** menubar 전용 상태 모델을 OS-agnostic helper로 분리하고, darwin systray 구현은 그 상태 모델을 렌더링하는 thin layer로 유지한다. 실제 macOS runtime 검증은 현재 Linux host에서 불가능하므로, 이번 단계는 1) pure formatting/state tests, 2) darwin wiring 구현, 3) Linux host 기준 `go test`/`go build` 보존까지를 완료 기준으로 둔다.

**Tech Stack:** Go 1.25, Cobra, getlantern/systray, internal/usage, macOS Terminal/osascript

---

## Repository observations

- 현재 menubar 구현은 `cmd/menubar_darwin.go`에 직접 박힌 MVP이며 메뉴 항목이 `Usage (once)`, `Monitor (once)`, `Quit` 세 개뿐이다.
- 현재는 menu state model, refresh lifecycle, provider summary formatting이 분리되어 있지 않아 테스트 가능한 단위가 거의 없다.
- `usage.GetUsage()`와 `session-refresh`는 이미 존재하므로, menubar는 새 데이터 소스보다 기존 명령의 축약된 진입면으로 설계하는 편이 맞다.
- 현재 Linux host에서는 Swift/Xcode도 없고 macOS systray runtime도 검증할 수 없으므로, 동작 신뢰성은 pure Go tests + build 보존에 의존해야 한다.
- `runInTerminal("oct ...")` 는 PATH에 의존하므로, menubar에서 현재 실행 binary path를 직접 쓰는 쪽이 더 안전하다.

## Scope

- menubar 상단에 현재 상태(summary / last refresh) 표시
- provider별 compact usage line 표시
- provider 상세 submenu 표시
- refresh-now 액션 추가
- 주기적 auto-refresh ticker 추가
- alert shortcut(`usage --notify`) 추가
- `usage`, `session-refresh`, `monitor` 진입을 current executable 기준으로 실행
- 상태 모델/명령 문자열 helper에 대한 unit tests 추가

## Non-goals

- macOS host에서 실제 systray 클릭 동작 완전 E2E 검증
- menubar 내부에서 full TUI/monitor를 직접 embed
- config/alert/schedule의 모든 설정 화면을 menubar에 노출
- launch agent 자동 등록

---

### Task 1: menubar 상태 모델을 pure helper로 분리

**Objective:** systray 없이도 검증 가능한 snapshot/summary/row formatting 계층을 만든다.

**Files:**
- Create: `cmd/menubar_state.go`
- Create: `cmd/menubar_state_test.go`

**Step 1: Write failing tests**

검증 대상:
- loading snapshot은 `oct …` title과 loading summary를 가진다
- usage snapshot은 ok/warn/error count를 요약한다
- provider line은 `5h`/`7d`/status를 compact하게 포함한다
- error snapshot은 마지막 refresh 시간과 오류 요약을 남긴다

**Step 2: Run targeted test to verify failure**

```bash
GOTOOLCHAIN=auto go test ./cmd -run 'TestMenubar' -v
```

Expected:
- FAIL — helper symbols not defined

**Step 3: Implement minimal snapshot helpers**

포함 함수:
- loading snapshot builder
- usage results -> top summary/title builder
- provider line formatter
- refresh timestamp formatter

**Step 4: Run targeted test to verify pass**

```bash
GOTOOLCHAIN=auto go test ./cmd -run 'TestMenubar' -v
```

Expected:
- PASS

---

### Task 2: current executable 기반 terminal command helper 추가

**Objective:** menubar action이 PATH의 다른 `oct` 대신 현재 실행 binary를 호출하게 만든다.

**Files:**
- Modify: `cmd/menubar_state.go`
- Modify: `cmd/menubar_state_test.go`
- Modify: `cmd/menubar_darwin.go`

**Step 1: Write failing tests**

검증 대상:
- shell quoting이 공백/작은따옴표를 안전하게 처리한다
- `buildMenubarExecCommand("/tmp/oct", "usage")` 가 current binary를 직접 호출한다
- AppleScript payload builder가 command string을 안전하게 포함한다

**Step 2: Run targeted test to verify failure**

```bash
GOTOOLCHAIN=auto go test ./cmd -run 'TestMenubar(Command|AppleScript|ShellQuote)' -v
```

**Step 3: Implement helpers and darwin call sites**

- shell quote helper
- current executable command builder
- `runInTerminal` AppleScript string builder
- 기존 `oct usage` / `oct monitor --once` 하드코딩 제거

**Step 4: Re-run targeted tests**

```bash
GOTOOLCHAIN=auto go test ./cmd -run 'TestMenubar(Command|AppleScript|ShellQuote)' -v
```

Expected:
- PASS

---

### Task 3: darwin systray를 상태형 UI로 교체

**Objective:** static launcher를 loading → refreshed summary → action menu 구조로 바꾼다.

**Files:**
- Modify: `cmd/menubar_darwin.go`

**Step 1: Add menu structure**

초기 구조:
- summary (disabled)
- last refresh (disabled)
- provider rows (disabled)
- separator
- Refresh now
- Open Usage
- Run Session Refresh
- Open Monitor
- Quit

**Step 2: Add async refresh lifecycle**

- startup 시 즉시 refresh
- refresh 중 중복 클릭 방지
- 성공 시 summary/title/provider rows 갱신
- 실패 시 error snapshot 노출

**Step 3: Keep terminal-launch actions narrow**

- `usage`
- `session-refresh`
- `monitor --once`

**Step 4: Manual static review**

```bash
git diff -- cmd/menubar_darwin.go cmd/menubar_state.go cmd/menubar_state_test.go
```

Expected:
- systray wiring은 thin layer이고 formatting/business logic는 helper로 분리됨

---

### Task 4: Regression validation

**Objective:** Linux host에서도 회귀 없이 merge 가능한 상태로 검증한다.

**Files:**
- Modify: `cmd/menubar_state.go`
- Modify: `cmd/menubar_state_test.go`
- Modify: `cmd/menubar_darwin.go`

**Step 1: Run targeted tests**

```bash
GOTOOLCHAIN=auto go test ./cmd -run 'TestMenubar' -v
```

**Step 2: Run full tests**

```bash
GOTOOLCHAIN=auto go test ./...
```

**Step 3: Run local build**

```bash
GOTOOLCHAIN=auto go build -o oct main.go
```

**Step 4: Record macOS runtime gap explicitly**

최종 보고에 아래를 명시:
- systray click/runtime verification is pending on a real macOS host
- current Linux host can only verify test/build integrity

---

## Exit criteria

- `oct menubar` 구현이 static launcher가 아니라 상태형 snapshot UI 구조를 가짐
- top summary / last refresh / provider rows / refresh action이 존재함
- terminal-launch actions가 current executable path를 사용함
- menubar helper tests가 추가됨
- `go test ./...` 와 `go build -o oct main.go` 가 통과함
- macOS 실기기 검증 미완료가 명시됨
