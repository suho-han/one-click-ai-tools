# Cursor/OpenCode Usage Quality Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Cursor/OpenCode usage 출력이 단순 수집값 나열을 넘어서, 사용자가 “지금 어떤 근거로 표시됐는지 / 값이 비었는지 / 로컬 fallback인지”를 즉시 해석할 수 있도록 품질을 개선한다.

**Architecture:** fetcher 로직 자체를 크게 바꾸기보다 `UsageResult`의 메시지/버킷/source/source_detail 해석을 정교화하고, table/JSON summary 출력에서 “no data / local estimate / remote+local / partial”를 더 잘 구분한다. 즉, 정확도보다 해석 가능성을 먼저 올린다.

**Tech Stack:** Go 1.25, Cobra, table/json output, local log parsing, go test, go build

---

## Repository observations

- `internal/usage/cursor.go`는 세 단계 소스를 가진다:
  1. custom endpoint
  2. local auth token + Cursor API
  3. workspace storage fallback
- Cursor는 remote/local fallback이 섞일 수 있지만, 현재 표 출력은 `period`, `5h`, `1w`, `used`, `limit`, `source`, `status`, `message`에 그 차이가 완전히 드러나지 않는다.
- `internal/usage/opencode.go`는 최신 JSONL 로그 하나에서 값을 추출하며, 없으면 `No local OpenCode session logs found` 또는 `no usage metrics in latest session` 정도만 보여준다.
- `internal/usage/usage.go`는 summary classification에서 일부 메시지를 보고 `warn`로 격하하지만, 사용자 관점에서는 “정상 0”과 “데이터 없음 0”의 구분이 여전히 약하다.
- 현재 버킷 표시는 5h/7d 중심인데, Cursor API는 월간 request 중심 구조이고 OpenCode는 latest local session 중심이라 period/used/buckets의 의미가 provider별로 다르다.

## Scope

- Cursor/OpenCode 결과 메시지와 상태 구분 정교화
- table/JSON에서 local estimate / remote fallback / partial parse 구분 강화
- 테스트에서 no-data vs real-zero vs partial-data 시나리오 보강
- 필요 시 source/source_detail 포맷 표준화

## Non-goals

- Cursor/OpenCode의 새로운 원격 API reverse-engineering
- provider별 복잡한 UI 대시보드 추가
- 전체 usage schema 대규모 breaking change

---

### Task 1: 사용자가 헷갈리는 상태를 taxonomy로 먼저 정리

**Objective:** 코드 수정 전에 어떤 상태를 별도 메시지로 구분할지 고정한다.

**Files:**
- Modify: `docs/plans/2026-06-09-usage-quality-plan.md`
- Inspect: `internal/usage/cursor.go`
- Inspect: `internal/usage/opencode.go`
- Inspect: `internal/usage/usage.go`

**Step 1: Define status classes**

최소 분류:
- `ok`: 신뢰 가능한 값 수집 성공
- `warn/no-data`: 데이터 소스는 봤지만 usage metric이 없음
- `warn/local-estimate`: 원격 실패 후 로컬 추정치만 있음
- `warn/partial`: 일부 bucket/field만 확보
- `error`: parse 또는 request 자체 실패

**Step 2: Define message style**

메시지는 provider별 자유문장이 아니라 공통 톤으로 정리:
- `Fetched from Cursor API`
- `Estimated from local Cursor workspace storage`
- `OpenCode logs found, but latest session has no usage metrics`
- `Remote fetch failed; showing local estimate`

---

### Task 2: Cursor result semantics 개선

**Objective:** Cursor의 remote/local/combined 상태를 표와 JSON에서 더 잘 읽히게 만든다.

**Files:**
- Modify: `internal/usage/cursor.go`
- Modify: `internal/usage/usage.go`
- Test: `internal/usage/usage_test.go`

**Step 1: Separate remote+local vs local-only messages**

예:
- `remote+local`: API success + local session count detail exists
- `local-auth-api-failed`: auth token was present but API failed; local estimate shown
- `local-auth-missing`: no auth token; local estimate only

**Step 2: Improve period/source semantics**

예:
- API monthly response는 `period=monthly:YYYY-MM`
- local fallback은 `period=local`
- `source`는 `local`, `local-auth`, `remote`, `remote+local` 중 하나로 제한

**Step 3: Improve zero handling**

`Used=0`이라도 아래를 구분:
- 진짜 0 usage
- local storage 없음
- token 없음 + fallback 없음

**Step 4: Add tests**

필수 시나리오:
- API success
- API fail + local fallback
- token missing + local fallback
- token missing + local no-data

---

### Task 3: OpenCode result semantics 개선

**Objective:** OpenCode가 최신 로그 단건 기반이라는 한계를 더 명확히 드러내고, no-data와 parse-failure를 구분한다.

**Files:**
- Modify: `internal/usage/opencode.go`
- Test: `internal/usage/usage_test.go`
- (Optional) Create: `internal/usage/opencode_test.go`

**Step 1: Distinguish missing logs vs metric-missing logs**

현재 메시지를 아래처럼 더 선명하게 분리:
- `No local OpenCode session logs found`
- `Latest OpenCode session log has no usage metrics`

**Step 2: Clarify bucket meaning**

문서 또는 message에 반영:
- `5h` = primary window if present
- `7d` = secondary/weekly window when found

**Step 3: Improve parse-failure path**

에러 시:
- `status=error`
- `message`는 짧고 일관되게
- 상세 파일 경로는 `OCT_USAGE_DEBUG=1`일 때만 `source_detail`

**Step 4: Add tests**

필수 시나리오:
- no files
- files exist but no metrics
- valid primary only
- valid primary + weekly secondary
- malformed latest file

---

### Task 4: Table/JSON summary 해석 가능성 개선

**Objective:** 사용자가 표만 보고도 “왜 warn인지” 이해할 수 있게 한다.

**Files:**
- Modify: `internal/usage/usage.go`
- Test: `internal/usage/usage_test.go`
- (Optional) Modify: `cmd/usage.go`

**Step 1: Tighten summary classification**

`classifySummaryStatus()`가 아래를 더 잘 반영하도록 보강:
- `Used=0` + no-data message => warn
- `Used=n/a` + partial message => warn
- `remote+local` + numeric data => ok

**Step 2: Standardize message prefixes**

예:
- `No data:`
- `Local estimate:`
- `Partial:`
- `Remote failed:`

**Step 3: Consider source-detail exposure rule**

TTY 표에서는 과도한 경로 노출 금지
- 상세 경로/파일명은 `OCT_USAGE_DEBUG=1` 또는 JSON에서만

**Step 4: Add JSON regression tests**

summary counts가 no-data/partial/error를 제대로 반영하는지 검증.

---

### Task 5: Docs/help/test sync

**Objective:** 사용자가 Cursor/OpenCode usage의 신뢰도와 fallback 의미를 문서에서 이해할 수 있게 한다.

**Files:**
- Modify: `CONTEXT/ko/USAGE.md`
- Modify: `CONTEXT/en/USAGE.md`
- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `cmd/usage.go`

**Step 1: Document source priority**

Cursor:
1. custom endpoint
2. local auth + API
3. local workspace fallback

OpenCode:
1. local session logs
2. latest log metric extraction
3. no-data / parse-failure fallback

**Step 2: Document interpretation**

짧게 표기:
- `source=remote+local` means API data plus local session hint
- `source=local` may be an estimate, not official quota

**Step 3: Run validation**

```bash
GOTOOLCHAIN=auto go test ./internal/usage ./cmd -run 'Usage|Cursor|OpenCode|Summary' -v
GOTOOLCHAIN=auto go test ./...
go build -o oct main.go
```

Expected:
- usage-related tests pass
- full build succeeds

**Step 4: Commit**

```bash
git add internal/usage/cursor.go internal/usage/opencode.go internal/usage/usage.go internal/usage/usage_test.go internal/usage/opencode_test.go CONTEXT/ko/USAGE.md CONTEXT/en/USAGE.md README.md README.en.md cmd/usage.go
git commit -m "feat(usage): improve cursor and opencode usage clarity"
```

---

## Exit criteria

- Cursor/OpenCode usage 결과에서 no-data / local-estimate / partial / error가 분리되어 보임
- 표와 JSON summary가 같은 해석을 공유함
- `Used=0`이 실제 0인지 데이터 없음인지 더 분명해짐
- 문서에 source priority와 fallback 의미가 반영됨
