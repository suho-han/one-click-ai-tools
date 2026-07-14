# Scheduler E2E Validation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** `agent-update`와 `session-refresh`의 플랫폼별 scheduler 동작과 install-time wiring을 실환경에서 검증하고, 플랫폼별 잔여 리스크를 줄인다.

**Architecture:** 기존 단위 테스트는 유지하되, 실환경 검증은 1) install-time postinstall 흐름, 2) scheduler enable/disable 동작, 3) generated entry/plist/task action inspection, 4) 재실행/중복 방지 확인 순서로 나눈다. 플랫폼별 차이는 `internal/schedule` 구현에 맞춰 Linux(cron), macOS(launchctl), Windows(Task Scheduler) 체크리스트로 분리하고, 공통 성공 조건은 “현재 실행 binary를 정확히 가리키는가”로 통일한다.

**Tech Stack:** Go 1.25, Cobra CLI, npm postinstall, cron, launchctl, schtasks, go test, go build

---

## Repository observations

- `internal/schedule/*`는 OS별 구현이 완전히 분리되어 있으므로 실환경 검증도 파일 단위가 아니라 OS 단위로 해야 한다.
- 이번 session-refresh 수동 검증 중 scheduler가 PATH 상의 다른 `oct`를 가리킬 수 있는 실버그가 발견되었고, `fix(schedule): prefer current oct binary for scheduled tasks`로 수정되었다.
- 현재 `CONTEXT/ko/PLATFORM_E2E_CHECKLIST.md`는 원래 Linux/Windows의 `agent-update` 중심이라 `session-refresh`, macOS, install-time prompt 검증이 부족했다.
- install-time wiring은 `scripts/postinstall.js`가 `session_refresh_*` config 저장과 `oct schedule enable --task session-refresh ...`를 직접 호출하는 구조다.

## Scope

- install-time `session-refresh` prompt/auto-enable 검증
- Linux/macOS/Windows의 `agent-update` / `session-refresh` scheduler entry 검증
- current binary path correctness 검증
- duplicate entry 방지와 disable cleanup 검증
- 체크리스트/문서 최신화

## Non-goals

- 각 scheduled task의 실제 장시간 실행 결과를 CI에서 완전 E2E로 보장
- remote quota API나 provider-level usage 정확도 개선
- tray/widget/monitoring UI 기능 추가

---

### Task 1: 체크리스트를 task-aware / platform-complete 상태로 보강

**Objective:** Linux-only/agent-update 중심 체크리스트를 현재 기능 집합에 맞게 확장한다.

**Files:**
- Modify: `CONTEXT/ko/PLATFORM_E2E_CHECKLIST.md`
- Modify: `CONTEXT/en/PLATFORM_E2E_CHECKLIST.md`

**Step 1: Add install-time validation section**

추가 항목:
- non-interactive install
- interactive prompt
- `OCT_INSTALL_ENABLE_SESSION_REFRESH=yes`
- generated `~/.oct/config.yaml` defaults

**Step 2: Split scheduler checks by task**

각 OS 섹션을 아래 두 task로 분리:
- `agent-update`
- `session-refresh`

**Step 3: Add current-binary-path verification**

각 OS에서 생성된 entry/plist/task action이 현재 실행 binary를 가리키는지 확인 항목 추가.

**Step 4: Add duplicate-entry / cleanup checks**

반복 enable 시 중복 엔트리 생성 여부와 disable 후 정리 여부를 종료 조건에 포함.

**Step 5: Verify doc diff**

Run:

```bash
git diff -- CONTEXT/ko/PLATFORM_E2E_CHECKLIST.md CONTEXT/en/PLATFORM_E2E_CHECKLIST.md
```

Expected:
- macOS, session-refresh, install-time validation 항목이 모두 포함됨

---

### Task 2: Linux 실환경 수동 검증 절차 고정

**Objective:** Linux에서 cron 기반 동작을 반복 가능하게 검증한다.

**Files:**
- Modify: `CONTEXT/ko/LOCAL_TEST.md`
- (Optional) Modify: `CONTEXT/en/LOCAL_TEST.md`
- (Optional) Create: `scripts/dev/check-scheduler-linux.sh`

**Step 1: Capture pre-clean commands**

문서에 다음 명령 추가:

```bash
oct schedule disable --task agent-update || true
oct schedule disable --task session-refresh || true
crontab -l || true
```

**Step 2: Add task-by-task validation commands**

문서에 아래 흐름 명시:

```bash
oct schedule enable --task agent-update --interval daily --hour 3
crontab -l

oct schedule enable --task session-refresh --interval daily --hour 9
crontab -l
```

**Step 3: Add current-binary comparison**

문서에 비교 명령 추가:

```bash
command -v oct
crontab -l | grep oct-managed
```

Expected:
- cron entry binary path == current `oct`

**Step 4: Add cleanup verification**

```bash
oct schedule disable --task agent-update
oct schedule disable --task session-refresh
crontab -l || true
```

Expected:
- 관련 marker가 모두 제거됨

**Step 5: Commit**

```bash
git add CONTEXT/ko/LOCAL_TEST.md CONTEXT/en/LOCAL_TEST.md scripts/dev/check-scheduler-linux.sh
git commit -m "docs: add linux scheduler e2e validation steps"
```

---

### Task 3: macOS 수동 검증 절차 작성

**Objective:** launchctl/plist 기반 검증 절차를 명확히 문서화한다.

**Files:**
- Modify: `CONTEXT/ko/PLATFORM_E2E_CHECKLIST.md`
- Modify: `CONTEXT/en/PLATFORM_E2E_CHECKLIST.md`
- (Optional) Modify: `CONTEXT/ko/LOCAL_TEST.md`

**Step 1: Add plist inspection commands**

```bash
plutil -p ~/Library/LaunchAgents/com.oct.agent-update.plist
plutil -p ~/Library/LaunchAgents/com.oct.session-refresh.plist
```

**Step 2: Add launchctl status commands**

```bash
launchctl list | grep com.oct.agent-update
launchctl list | grep com.oct.session-refresh
```

**Step 3: Add binary-path verification**

Expected:
- `ProgramArguments[0]` equals current `oct` binary path
- `ProgramArguments[1]` equals `agent-update` or `session-refresh`

**Step 4: Add disable/unload verification**

Expected:
- plist removed
- `launchctl list` no longer returns the labels

**Step 5: Verification note**

실기기는 현재 세션 Linux 호스트가 아니라 별도 macOS host에서 수행해야 함을 명시.

---

### Task 4: Windows 수동 검증 절차 작성

**Objective:** schtasks action string과 quoting 리스크를 명시적으로 검증한다.

**Files:**
- Modify: `CONTEXT/ko/PLATFORM_E2E_CHECKLIST.md`
- Modify: `CONTEXT/en/PLATFORM_E2E_CHECKLIST.md`
- (Optional) Modify: `CONTEXT/en/LOCAL_TEST.md`

**Step 1: Add query commands**

```powershell
schtasks /Query /TN OneClickToolsUpdate /V /FO LIST
schtasks /Query /TN OneClickToolsSessionRefresh /V /FO LIST
```

**Step 2: Add action-path verification**

Expected:
- Task action uses the current installed `oct.exe`
- Action string safely handles spaces in install paths

**Step 3: Add task separation verification**

Expected:
- `OneClickToolsUpdate` and `OneClickToolsSessionRefresh` are distinct tasks

**Step 4: Add disable verification**

```powershell
oct schedule disable --task agent-update
oct schedule disable --task session-refresh
```

Expected:
- both task names disappear from `schtasks /Query`

**Step 5: Verification note**

현재 세션에서는 Windows 실기기 검증이 불가하므로 문서 우선으로 정리하고, 이후 실제 host에서 체크리스트 따라 실행.

---

### Task 5: 최소 회귀 검증 유지

**Objective:** 수동 검증 전후에 로컬 회귀가 깨지지 않도록 한다.

**Files:**
- Modify: `internal/schedule/schedule_test.go`
- (Optional) Modify: `cmd/*_test.go`

**Step 1: Keep binary-resolution regression test**

이미 추가된 검증을 유지:
- current executable preferred over PATH lookup
- PATH fallback still works

**Step 2: Run targeted tests**

```bash
GOTOOLCHAIN=auto go test ./internal/schedule -v
```

Expected:
- binary resolution tests pass

**Step 3: Run full regression**

```bash
GOTOOLCHAIN=auto go test ./...
go build -o oct main.go
```

Expected:
- all tests pass
- build succeeds

**Step 4: Commit**

```bash
git add internal/schedule/schedule_test.go
git commit -m "test(schedule): keep binary resolution regression coverage"
```

---

## Suggested execution order

1. 문서/체크리스트 보강
2. Linux 재현 절차 문서화
3. macOS 절차 문서화
4. Windows 절차 문서화
5. 남은 실제 host 검증 실행

## Exit criteria

- `CONTEXT/*/PLATFORM_E2E_CHECKLIST.md`가 현재 기능 범위를 모두 반영
- install-time `session-refresh` 검증 절차가 문서화됨
- 플랫폼별로 `agent-update`와 `session-refresh`가 분리된 task 이름/entry를 가진다는 점이 검증 가능해짐
- current-binary-path 검증이 모든 플랫폼 절차에 포함됨
- Linux는 즉시 실행 가능한 수동 절차가 문서에 고정됨
