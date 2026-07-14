# oct Improvement Phases (2026-06)

> **For Hermes:** Implement in order: release lane -> menubar lane -> CLI trust lane.

**Goal:** 최근 릴리스/원격 검증에서 드러난 운영 마찰을 줄이고, `oct`를 배포/운영/디버깅 가능한 CLI로 끌어올린다.

**Architecture:** 새 기능은 기존 명령 축 위에 얇게 얹는다. release lane은 shell script + dedicated doctor command로 강화하고, menubar lane은 helper discovery/diagnostics/빌드 진입면을 추가하며, CLI trust lane은 외부 command 실행 전에 explainable state를 보여주는 플래그/doctor 명령으로 확장한다.

**Tech Stack:** Go/Cobra/Viper, existing `internal/update`, `internal/sessionrefresh`, bash release script, Swift helper discovery hooks.

---

## Phase 1 — release lane hardening

### Scope
- `oct release-doctor` 추가
- npm/git/tag/release preflight 출력
- `scripts/release-package.sh` 에 explicit remote tag verification + fallback push 추가
- docs에 CI publish / local npm auth reality 반영

### Acceptance
- 로컬 릴리스 전에 npm auth / registry latest / working tree / remote tag push readiness를 한 번에 확인 가능
- `git push --follow-tags` 이후 remote tag 누락 시 자동으로 `git push origin refs/tags/<tag>` fallback 수행

---

## Phase 2 — menubar lane productization

### Scope
- `oct menubar doctor`
- `oct menubar build-helper`
- `oct menubar install-helper`
- helper path / current oct path / launch mode / candidate path 진단 출력
- helper build/install은 macOS 전용으로 명시적 실패 메시지 제공

### Acceptance
- 사용자가 helper launch 실패 시 원인(경로 없음, macOS 아님, build 산출물 없음)을 바로 볼 수 있음
- helper build/install 경로가 문서화된 후보 경로와 일치

---

## Phase 3 — CLI trust / explainability

### Scope
- `oct agent-update --dry-run`
- `oct agent-update --explain`
- `oct doctor shell`
- `oct session-refresh` before/after diff summary

### Acceptance
- `agent-update` 가 실제 실행 전 manager/command/version path를 설명 가능
- shell/path doctor가 current PATH와 bootstrap PATH 차이를 보여줌
- `session-refresh` 후 usage 변화가 없더라도 changed/unchanged summary를 제공

---

## Suggested file touch points

### New/expanded commands
- `cmd/root.go`
- `cmd/agent_update.go`
- `cmd/session_refresh.go`
- `cmd/menubar.go`
- `cmd/doctor.go` (new)
- `cmd/release_doctor.go` (new or grouped under doctor)

### Supporting internals
- `internal/update/update.go`
- `internal/update/manager.go`
- `internal/update/tools.go`
- `internal/sessionrefresh/sessionrefresh.go`
- `internal/execenv/execenv.go`

### Tests
- `cmd/*_test.go`
- `internal/update/*_test.go`
- `internal/sessionrefresh/*_test.go`
- `cmd/menubar_helper_test.go`

### Scripts / docs
- `scripts/release-package.sh`
- `README.md`
- `PROJECT_CONTEXT/remote-macos-validation-status.md`

---

## Validation target
- targeted `go test` for each lane
- full `GOTOOLCHAIN=auto go test ./...`
- `GOTOOLCHAIN=auto go build -o oct main.go`
- smoke: `./oct help`, new subcommand help, dry-run/explain outputs
