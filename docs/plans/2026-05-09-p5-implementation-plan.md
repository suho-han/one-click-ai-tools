# P5 Implementation Plan

> **For Hermes:** use this plan for implementation from P5 onward.

**Goal:** Linux/Windows 실제 환경에서 스케줄/업데이트/usage 흐름을 안정적으로 검증하고, 릴리즈 전 자동 품질 게이트를 완성한다.

**Architecture:** 현재 추가된 smoke matrix + 단위테스트 기반 위에, P5에서는 E2E 실행 스크립트/문서화/릴리즈 게이트를 분리해 구성한다. 로컬 수동 검증은 체크리스트 기반으로 수행하고, CI는 빠른 smoke와 선택적 장기 E2E를 분리한다.

**Tech Stack:** Go test, GitHub Actions, cron/schtasks, one-click-tools CLI

---

## Scope (P5)
- 플랫폼별 E2E 실행 절차 자동화
- CI 게이트(필수 smoke + 선택적 E2E) 정리
- 운영 문서 보강(장애 대응/검증 로그 수집)

---

### Task 1: E2E 실행 스크립트 골격 추가
**Objective:** Linux/Windows 각각 동일한 검증 단계를 호출할 수 있는 스크립트 진입점 확보

**Files:**
- Create: `scripts/e2e/linux-smoke.sh`
- Create: `scripts/e2e/windows-smoke.ps1`
- Modify: `CONTEXT/PLATFORM_E2E_CHECKLIST.md`

**Steps:**
1. 체크리스트 항목을 스크립트 단계로 매핑한다.
2. 성공/실패 시 표준 exit code와 로그 출력 형식을 통일한다.
3. README 또는 CONTEXT에 실행 예시를 추가한다.

**Verification:**
- Linux: `bash scripts/e2e/linux-smoke.sh`
- Windows: `pwsh -File scripts/e2e/windows-smoke.ps1`
- 두 스크립트 모두 실패 지점을 명확히 출력

---

### Task 2: CI 워크플로우 분리 (smoke / e2e)
**Objective:** PR 기본 검증은 빠르게 유지하고, E2E는 수동/스케줄 트리거로 분리

**Files:**
- Modify: `.github/workflows/smoke-matrix.yml`
- Create: `.github/workflows/platform-e2e.yml`

**Steps:**
1. smoke는 `go test ./...`, `go build ./...`만 유지한다.
2. platform-e2e 워크플로우를 `workflow_dispatch` + nightly 스케줄로 추가한다.
3. 아티팩트(로그) 업로드 단계 추가

**Verification:**
- `workflow_dispatch` 수동 실행 성공
- 로그 아티팩트 다운로드 가능

---

### Task 3: 스케줄 상태 검증 강화
**Objective:** enable/disable/status 회귀를 줄이기 위한 테스트 보강

**Files:**
- Modify: `internal/schedule/schedule_test.go`
- Modify: `internal/schedule/linux.go`
- Modify: `internal/schedule/windows.go`

**Steps:**
1. 정상 경로 + 실패 경로 케이스를 분리
2. 상태 문자열/에러 메시지 스냅샷 검증 추가
3. 경계값(hour/minute/interval) 검증 케이스 추가

**Verification:**
- `go test ./internal/schedule -v`
- 회귀 테스트 통과

---

### Task 4: usage/provider 확장 회귀 점검
**Objective:** cursor/opencode 포함 provider 파서 안정화

**Files:**
- Modify: `internal/usage/usage_test.go`
- Modify: `internal/usage/cursor_test.go`
- Modify: `internal/usage/opencode_skeleton_test.go`

**Steps:**
1. 샘플 입력 변형(누락 필드/0값/예상외 문자열) 추가
2. 파싱 실패 시 fallback 동작 검증
3. JSON 출력 모드 회귀 검증

**Verification:**
- `go test ./internal/usage -v`

---

### Task 5: 릴리즈 게이트 문서화
**Objective:** 릴리즈 전 점검 항목을 재사용 가능한 체크리스트로 표준화

**Files:**
- Create: `CONTEXT/RELEASE_GATE_P5.md`
- Modify: `README.md` (필요 시 링크 추가)

**Steps:**
1. smoke/e2e/수동 검증/롤백 체크리스트를 분리 작성
2. 실패 시 대응 절차(로그 위치, 재현 명령) 명시
3. 최종 서명 기준 정의

**Verification:**
- 신규 문서 기준으로 제3자가 동일 절차 재현 가능

---

## Done Criteria
- [ ] smoke와 e2e 워크플로우가 역할 분리됨
- [ ] 플랫폼별 실행 스크립트로 체크리스트 재현 가능
- [ ] schedule/usage 회귀 테스트 강화 완료
- [ ] 릴리즈 게이트 문서화 완료
