# P8 Implementation Plan — Monitor 출력 포맷/알림룰 고도화

> **For Hermes:** 이 계획을 기준으로 P8을 구현한다.

**Goal:** `oct monitor` 가 운영 환경에서 즉시 읽기 쉬운 출력(정렬/간소 모드/강조)을 제공하고, `oct alert` 는 우선순위·스누즈 정책까지 반영해 알림 피로를 줄인다.

**Architecture:** 기존 `cmd/monitor.go` 와 `internal/notify/usage_alert.go` 를 확장하되, 수집(`internal/usage`)과 표현/정책 계층을 분리한다. CLI 플래그와 viper 설정키를 추가하고, 동작 규칙은 단위 테스트로 잠근다.

**Tech Stack:** Go, Cobra, Viper, go test

---

## Scope (P8)
- monitor 출력 포맷 개선
  - 정렬 기준(`--sort-by provider|used|5h|7d`, `--desc`)
  - 컴팩트 모드(`--compact`) + 상위 N 표시(`--top`)
  - 상태/임계치 강조 라벨(OK/WARN/CRIT)
- alert 룰 고도화
  - provider/window별 우선순위 키 정리
  - 스누즈(`--snooze <duration>`)와 상태 확인/해제
  - 스누즈 중 CRIT(예: 98%+)는 override
- 설정/문서/테스트 반영

---

### Task 1: monitor 정렬/컴팩트 플래그 추가
**Objective:** 운영자가 필요한 provider를 먼저 보도록 출력 제어 기능 제공

**Files:**
- Modify: `cmd/monitor.go`
- Modify: `cmd/root.go` (기본값 필요시)
- Test: `cmd/monitor_test.go` (신규)

**Steps:**
1. `--sort-by`, `--desc`, `--top`, `--compact` 플래그 추가.
2. monitor 출력 전 정렬 함수(`sortMonitorResults`)를 분리.
3. compact 모드에서 컬럼 축소 및 message 생략 옵션 적용.

**Verification:**
- `go test ./cmd -run Monitor -v`
- `go run main.go monitor --once --sort-by used --desc --top 5 --compact`

---

### Task 2: monitor 상태 라벨(OK/WARN/CRIT) 표준화
**Objective:** 퍼센트 사용량을 즉시 해석 가능하도록 시각적 라벨 추가

**Files:**
- Modify: `cmd/monitor.go`
- Test: `cmd/monitor_test.go`

**Steps:**
1. threshold 기반 라벨 함수(`usageSeverity`) 추가.
2. monitor row에 severity 컬럼 추가.
3. percent 외 unit에서는 UNKNOWN 처리.

**Verification:**
- `go test ./cmd -run Severity -v`
- `go run main.go monitor --once`

---

### Task 3: alert snooze 상태 파일/판정 추가
**Objective:** 특정 기간 알림 중지(스누즈) 지원

**Files:**
- Modify: `internal/notify/usage_alert.go`
- Modify: `internal/notify/usage_alert_test.go`

**Steps:**
1. alert state에 `snoozed_until`(global/provider/window) 구조 추가.
2. `MaybeSendUsageAlerts` 에 스누즈 판정 삽입.
3. CRIT 임계치(기본 98+)는 스누즈 중에도 override 허용.

**Verification:**
- `go test ./internal/notify -v`

---

### Task 4: `oct alert snooze` CLI 추가
**Objective:** 운영 중 즉시 스누즈/해제를 CLI로 제어

**Files:**
- Modify/Create: `cmd/alert.go`
- Modify: `CONTEXT/USAGE_ALERTS.md`

**Steps:**
1. `oct alert snooze set --duration 2h [--provider codex] [--window 5h]`
2. `oct alert snooze show`
3. `oct alert snooze clear [--provider] [--window]`
4. 설정 저장/로드 및 에러 메시지 정비.

**Verification:**
- `go run main.go alert snooze set --duration 1h`
- `go run main.go alert snooze show`
- `go run main.go alert snooze clear`

---

### Task 5: 문서/회귀 테스트/커밋
**Objective:** 사용자 문서와 회귀 안전성 확보

**Files:**
- Modify: `README.md`
- Modify: `CONTEXT/MONITORING.md`
- Modify: `CONTEXT/USAGE_ALERTS.md`

**Steps:**
1. 신규 플래그/명령 사용 예시 추가.
2. 스누즈 + CRIT override 동작 규칙 문서화.
3. `go test ./...` 전체 통과 확인.

**Verification:**
- `go test ./...`
- 필요 시 `gofmt -w ...`

---

## Done Criteria
- [ ] monitor 정렬/compact/top 기능 동작
- [ ] severity 라벨이 명확히 출력
- [ ] alert snooze set/show/clear 동작
- [ ] 스누즈 중 CRIT override 테스트 통과
- [ ] 문서/CI 통과 후 main 반영
