# P6 Usage Alert Detailed Settings Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** 사용량 알림을 provider/버킷(5h/7d 등) 기준으로 세밀하게 설정하고, 쿨다운/조용한 시간대/테스트 명령까지 지원한다.

**Architecture:** 기존 `internal/notify/usage_alert.go`의 단일 임계치 모델을 확장해 규칙 기반(`global + provider override`) 평가 엔진으로 변경한다. 설정은 `viper` 기반으로 로드하고, `oct alert` 커맨드에서 조회/검증/테스트를 제공한다.

**Tech Stack:** Go, Cobra, Viper, existing notify/usage modules

---

### Task 1: 설정 스키마 정의
**Objective:** 알림 규칙을 표현하는 구조체와 기본값을 확정한다.

**Files:**
- Modify: `internal/notify/usage_alert.go`
- Modify: `cmd/root.go`

**Steps:**
1. `UsageAlertConfig`를 확장한다.
   - global thresholds (기본값)
   - provider별 thresholds
   - cooldown
   - quiet hours
   - timezone
2. 기존 단일 `ThresholdPct`도 하위 호환으로 유지한다.
3. `viper` 기본값을 추가한다.

**Verification:**
- `go test ./internal/notify -v`

---

### Task 2: 규칙 평가 엔진 구현
**Objective:** provider/window별 임계치, quiet hours, 쿨다운을 반영한 알림 결정 로직 구현

**Files:**
- Modify: `internal/notify/usage_alert.go`
- Modify: `internal/notify/usage_alert_test.go`

**Steps:**
1. 임계치 계산 우선순위 구현
   - provider+window > provider global > global window > global default
2. quiet hours 파서(`HH:MM-HH:MM`) 구현
3. 동일 key 재알림 쿨다운 유지 + 상위 임계치 돌파는 알림
4. 상태 파일 포맷 확장

**Verification:**
- `go test ./internal/notify -v`

---

### Task 3: CLI 명령 추가 (`oct alert`)
**Objective:** 알림 설정/테스트를 사용자 명령으로 제어 가능하게 만든다.

**Files:**
- Create: `cmd/alert.go`
- Modify: `cmd/usage.go`

**Steps:**
1. `oct alert config show`
2. `oct alert test --provider codex --window 5h --value 96`
3. `oct usage --notify`와 엔진 연결 유지

**Verification:**
- `go run main.go alert config show`
- `go run main.go alert test --provider codex --window 5h --value 96`

---

### Task 4: 문서화
**Objective:** 실제 사용자가 바로 설정 가능한 문서 제공

**Files:**
- Modify: `README.md`
- Create: `CONTEXT/USAGE_ALERTS.md`

**Steps:**
1. YAML 예시 추가
2. 주요 키 설명
3. 운영 팁(쿨다운/quiet hours)

**Verification:**
- 문서 예시 명령을 로컬에서 실행

---

### Done Criteria
- [ ] provider/window별 임계치 동작
- [ ] quiet hours/쿨다운 동작
- [ ] `oct alert config show`, `oct alert test` 제공
- [ ] 테스트 통과 및 문서 반영
