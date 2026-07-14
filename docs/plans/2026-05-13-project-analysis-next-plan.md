# one-click-tools 프로젝트 전체 분석 기반 실행 계획 (2026-05-13)

> **For Hermes:** 이 문서는 현재 코드베이스 상태를 기준으로 우선순위 작업을 단계별로 실행하기 위한 계획이다.

**Goal:** monitor 출력/알림룰 고도화 중심으로 제품 품질·배포 일관성·테스트 신뢰도를 동시에 개선한다.

**Architecture:** `cmd`(CLI UX) + `internal/notify`(알림 정책) + `internal/usage`(데이터 수집) 축을 분리 유지하고, 단계별로 테스트/문서/릴리즈 메타데이터를 동기화한다.

**Tech Stack:** Go 1.25, Cobra, Viper, npm wrapper, go test/go build

---

## 0) 현재 상태 스냅샷

- 보안 수정 커밋 push 완료: `126fba6` (`handlebars 4.7.9 override`)
- 빌드: `go build ./...` 통과
- 테스트: `go test ./... -cover` 통과
- 커버리지 관측:
  - `internal/notify` 79.2%
  - `internal/usage` 50.6%
  - `cmd` 26.4%
  - `internal/update` 20.4%
  - `internal/schedule` 5.6%
  - `internal/ui` 4.6%
  - `internal/config` 0.0%
- 코드 구조:
  - Go 파일 50개, 테스트 파일 18개
  - 핵심 경로: `cmd/*`, `internal/{usage,notify,update,schedule,config,ui}`

---

## 1) 우선순위 A — monitor 출력/알림룰 고도화 마무리 (사용자 우선순위 반영)

### Task A1: monitor 출력 규칙 안정화
**Objective:** 운영 화면 가독성/일관성 강화

**Files:**
- Modify: `cmd/monitor.go`
- Test: `cmd/monitor_test.go`

**Steps:**
1. `--compact`, `--sort-by`, `--desc`, `--top` 조합별 출력 스냅샷 테스트 보강
2. 폭(`COLUMNS`)별 메시지 절단/컬럼 정렬 회귀 테스트 추가
3. provider 아이콘 fallback(UTF-8 미지원) 케이스 테스트 추가

**Verification:**
- `go test ./cmd -run Monitor -v`
- `go run main.go monitor --once --sort-by used --desc --top 5 --compact`

### Task A2: alert priority/snooze 정책 테스트 강화
**Objective:** 알림 피로를 줄이면서 CRIT 신호는 반드시 보장

**Files:**
- Modify: `internal/notify/usage_alert.go`
- Test: `internal/notify/usage_alert_test.go`

**Steps:**
1. provider/window별 임계치 우선순위 해석 테스트 추가
2. snooze 중 CRIT override 시나리오(경계값 포함) 추가
3. cooldown + snooze 동시 적용 시 중복 전송 방지 테스트 추가

**Verification:**
- `go test ./internal/notify -v`

---

## 2) 우선순위 B — 릴리즈/버전 메타데이터 정합성 복구

### Task B1: 버전 소스 정합성 통일
**Objective:** 실행 바이너리 버전과 npm 패키지 버전 불일치 제거

**Observation:**
- `package.json`은 `0.4.4`인데 `cmd/root.go`는 `Version: "0.4.0"`

**Files:**
- Modify: `cmd/root.go`
- (옵션) Add: 빌드 시 ldflags 주입 방식 문서

**Steps:**
1. 버전 상수 단일 소스 전략 결정(고정 상수 vs ldflags)
2. `oct version` 출력 테스트 추가
3. 릴리즈 프로세스 문서에 버전 갱신 체크리스트 추가

**Verification:**
- `go test ./cmd -run Root -v`
- `go run main.go --version`

---

## 3) 우선순위 C — 저커버리지 모듈 회귀 방어선 구축

### Task C1: schedule/config/ui 최소 회귀 세트 확장
**Objective:** 변경 잦은 기능에 대한 기본 안전망 확보

**Files:**
- Modify: `internal/schedule/schedule_test.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/ui/image_test.go`

**Steps:**
1. 플랫폼 분기(schedule) 경로별 단위 테스트 추가
2. config 기본값/환경변수 오버라이드(OCT_ prefix) 테스트 추가
3. ui 이미지 렌더 옵션 실패/대체 경로 테스트 추가

**Verification:**
- `go test ./internal/schedule ./internal/config ./internal/ui -v`

---

## 4) 우선순위 D — 문서·운영 가이드 최신화

### Task D1: 실사용 문서와 구현 동기화
**Objective:** 사용자가 README만 보고도 monitor/alert 고급기능 재현 가능

**Files:**
- Modify: `README.md`
- Modify: `CONTEXT/ko/MONITORING.md`
- Modify: `CONTEXT/ko/USAGE_ALERTS.md`

**Steps:**
1. 정렬/compact/top/snooze 예시를 실제 동작 명령으로 정리
2. CRIT override 정책과 quiet hours 관계 명시
3. 트러블슈팅(아이콘 깨짐, 환경변수 prefix) 섹션 갱신

**Verification:**
- 문서 명령 샘플 수동 실행 점검

---

## 5) 실행 순서 제안 (job 단위 커밋)

1. `feat`: A1 + A2 (monitor/alert 로직)
2. `test`: C1 (저커버리지 테스트 보강)
3. `fix`: B1 (버전 정합성)
4. `docs`: D1 (가이드 동기화)

각 job 종료 시 공통 검증:
- `go test ./...`
- `go build ./...`

---

## Done Criteria

- [ ] monitor 출력 옵션 조합 회귀 테스트 확보
- [ ] alert priority/snooze/crit 규칙 테스트 통과
- [ ] `oct --version`과 패키지 버전 정합성 확보
- [ ] `internal/schedule/config/ui` 커버리지 유의미 개선
- [ ] README/CONTEXT 문서가 현재 동작과 일치
