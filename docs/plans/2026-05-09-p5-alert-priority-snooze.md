# P5 Alert Priority & Snooze 고도화 구현 계획

> **For Hermes:** subagent-driven-development 없이 현재 세션에서 TDD로 직접 구현한다.

**Goal:** usage alert에 우선순위(critical/high/normal)를 부여하고, 알림 메시지/테스트를 해당 우선순위 규칙에 맞게 일관화한다.

**Architecture:** 기존 `internal/notify/usage_alert.go`의 임계치 판정 결과(`alertHit`)는 유지하고, 우선순위 산정 함수를 추가해 전송 메시지/quiet-hour 예외 기준을 공통 처리한다. snooze 범위(global/provider/window/provider+window) 로직은 유지하되 우선순위 기반 override를 명확히 검증한다.

**Tech Stack:** Go, testing package, existing Cobra/Viper config.

---

## Task 1: 우선순위 규칙 테스트(RED) 추가

**Objective:** 값/임계치/critical 기준으로 priority가 예상대로 계산되는지 실패 테스트부터 작성

**Files:**
- Modify: `internal/notify/usage_alert_test.go`

**Steps:**
1. `TestAlertPriority` 추가
2. 케이스:
   - value >= critical -> `critical`
   - value >= threshold && value < critical -> `high`
   - value < threshold -> `normal`
3. 테스트 실행 시(구현 전) 실패 확인

## Task 2: 메시지 포맷 테스트(RED) 추가

**Objective:** 전송 메시지에 priority 라벨이 포함되는지 실패 테스트로 보장

**Files:**
- Modify: `internal/notify/usage_alert_test.go`

**Steps:**
1. `notifyFn` 훅으로 메시지 캡처
2. `MaybeSendUsageAlerts` 실행 후 메시지에 `[HIGH]` 또는 `[CRITICAL]` 포함 assert
3. 구현 전 실패 확인

## Task 3: 우선순위 함수/메시지 구현(GREEN)

**Objective:** 최소 구현으로 RED 테스트 통과

**Files:**
- Modify: `internal/notify/usage_alert.go`

**Steps:**
1. `type alertPriority string` 정의 (`normal|high|critical`)
2. `computeAlertPriority(value, threshold, criticalPct)` 구현
3. 메시지 포맷을 `[PRIORITY] provider window usage ...`로 변경
4. quiet hours 조건의 magic number(95) 제거하고 `priority != critical` 기준으로 억제

## Task 4: 회귀 테스트

**Objective:** 기존 동작(cooldown/escalation/snooze override) 유지 확인

**Files:**
- Modify: `internal/notify/usage_alert_test.go` (필요 시 기대문구만 보정)

**Steps:**
1. `go test ./internal/notify -v`
2. `go test ./cmd -v`
3. 가능하면 `go test ./...` (로컬 Go 버전 호환 시)

## Task 5: 문서 반영 + 분리 커밋

**Objective:** 사용자 가이드와 계획 문서를 분리 커밋 후 push

**Files:**
- Modify: `CONTEXT/USAGE_ALERTS.md`
- Add: `docs/plans/2026-05-09-p5-alert-priority-snooze.md`

**Commit strategy (job split):**
1. `feat(alert): add priority-based usage alert labeling`
2. `test(alert): add priority and message assertions`
3. `docs(plan): add p5 alert priority/snooze implementation plan`
4. `docs(alert): document priority labels and quiet-hours behavior`
