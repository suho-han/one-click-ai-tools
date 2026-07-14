# P7 Always-On Monitoring Screen Plan (Linux/Windows)

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** 리눅스/윈도우에서 상시 확인 가능한 사용량 모니터링 화면을 `oct monitor`로 제공하고, 상태 스냅샷(JSON) 저장을 지원한다.

**Architecture:** 초기에 플랫폼 GUI 대신 공통 터미널 라이브 대시보드(TUI-lite)를 구현한다. 수집 결과는 `~/.oct/state/usage-latest.json`에 저장하여 이후 윈도우 트레이/위젯과 분리 가능한 구조로 만든다.

**Tech Stack:** Go, Cobra, existing usage module, JSON state persistence

---

### Task 1: monitor 커맨드 추가
**Objective:** 주기 갱신되는 기본 화면 구현

**Files:**
- Create: `cmd/monitor.go`
- Modify: `cmd/root.go` (if needed via init side effects)

**Steps:**
1. `oct monitor --interval 30s` 플래그 추가
2. 주기적으로 `usage.GetUsage()` 호출
3. Ctrl+C 종료 처리

**Verification:**
- `go run main.go monitor --interval 5s`

---

### Task 2: 화면 출력 포맷
**Objective:** 상시 모니터링에 적합한 간결 출력

**Files:**
- Modify: `cmd/monitor.go`

**Steps:**
1. 타임스탬프/요약/provider별 핵심 수치 출력
2. 표시 모드 옵션(used/remaining 연동)
3. 에러 provider는 별도 표기

**Verification:**
- 로컬 실행시 반복 갱신 확인

---

### Task 3: 상태 스냅샷 저장
**Objective:** 외부 UI(향후 트레이/위젯)에서 읽을 상태 파일 생성

**Files:**
- Create: `internal/usage/state_snapshot.go`
- Modify: `cmd/monitor.go`

**Steps:**
1. `~/.oct/state/usage-latest.json` 저장
2. 저장 실패 시 모니터는 계속 실행
3. optional `--state-path` 플래그 지원

**Verification:**
- `cat ~/.oct/state/usage-latest.json`으로 최신 갱신 확인

---

### Task 4: 문서화
**Objective:** tmux/백그라운드 운영 방식 문서화

**Files:**
- Modify: `README.md`
- Create: `CONTEXT/MONITORING.md`

**Steps:**
1. Linux/Windows 실행 예시
2. 상시 운영 권장 방법(tmux/PowerShell)
3. 향후 트레이/위젯 연계 포인트 설명

**Verification:**
- 문서의 명령 복붙 실행 가능

---

### Done Criteria
- [ ] `oct monitor` 주기 갱신 동작
- [ ] 상태 스냅샷 파일 생성/갱신
- [ ] Linux/Windows 운영 문서 추가
- [ ] 테스트/빌드 통과
