# Antigravity Cleanup Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** `gemini` legacy alias 호환성은 유지하면서도, 사용자에게 보이는 문서/도움말/기본 예시/환경 변수 명칭에서 Gemini 중심 표현을 걷어내고 Antigravity 기준으로 정리한다.

**Architecture:** 내부 호환 계층(`NormalizeToolName`, alias matching, legacy config migration)은 유지한다. 정리 작업은 1) 사용자 노출 표면 정리, 2) legacy alias 허용 범위 문서화, 3) 테스트/문서 동기화 순서로 진행한다. 즉, 내부 호환성과 외부 표기를 분리한다.

**Tech Stack:** Go 1.25, Cobra, Viper, Markdown docs, go test, go build

---

## Repository observations

- `internal/update/tools.go`에서 Antigravity는 canonical binary `agy`를 사용하고, alias로 `antigravity`, `gemini`, `gemini-cli`를 허용한다.
- `NormalizeToolName()`는 `antigravity`, `gemini`, `gemini-cli`를 모두 `agy`로 normalize한다.
- 이는 backward compatibility에는 유효하지만, 사용자-facing 문서에서 계속 `gemini`를 예시로 쓰면 migration이 끝난 것처럼 보이지 않는다.
- `CONTEXT/ko/USAGE.md`에는 아직 아래 잔존물이 보인다:
  - 지원 대상: `Gemini CLI (`@google/gemini-cli`)`
  - 환경 변수: `OCT_GEMINI_USAGE_ENDPOINT`, `OCT_GEMINI_API_ENDPOINT`
- `README.en.md`에는 예시 환경 변수 `OCT_ENABLED_TOOLS=codex,gemini`가 남아 있다.
- `internal/usage/usage.go`는 fallback/labeling에서 `gemini` 문자열을 여전히 Antigravity와 동일 그룹으로 취급한다. 이는 내부 호환성 측면에서는 의도적일 수 있으나, 사용자-facing 출력/문서에서 그대로 노출되면 혼란이 생긴다.

## Scope

- README / CONTEXT / help text에서 Gemini 표기 정리
- 환경 변수/설정 예시를 Antigravity 기준으로 교체
- legacy alias 허용 범위를 명시적으로 문서화
- 테스트에서 기본값/표시값이 Antigravity 기준인지 재확인

## Non-goals

- `gemini` alias 자체 제거
- 기존 사용자 config를 깨는 rename-only migration
- Antigravity remote API/usage source 재설계

---

### Task 1: 사용자 노출 문서의 Gemini 표기 inventory 확정

**Objective:** 실제로 교체할 문자열과, 호환성 때문에 남겨야 할 문자열을 구분한다.

**Files:**
- Inspect: `README.md`
- Inspect: `README.en.md`
- Inspect: `CONTEXT/ko/USAGE.md`
- Inspect: `CONTEXT/en/USAGE.md`
- Inspect: `cmd/usage.go`
- Inspect: `cmd/config.go`

**Step 1: Categorize each occurrence**

세 그룹으로 분류:
1. 즉시 교체
   - 사용자 안내 문구
   - README 예시
   - 지원 대상 목록
2. 주석/호환성 설명으로 남김
   - legacy alias compatibility note
3. 내부 구현 유지
   - normalize logic
   - alias matcher

**Step 2: Verification**

Run:

```bash
git diff -- README.md README.en.md CONTEXT/ko/USAGE.md CONTEXT/en/USAGE.md cmd/usage.go cmd/config.go
```

Expected:
- 사용자-facing 표면에서 `gemini`는 compatibility note 외에는 사라짐

---

### Task 2: README / usage docs를 Antigravity 기준으로 교체

**Objective:** 지원 agent 목록, 설치 설명, config 예시를 현재 canonical naming과 일치시킨다.

**Files:**
- Modify: `README.md`
- Modify: `README.en.md`
- Modify: `CONTEXT/ko/USAGE.md`
- Modify: `CONTEXT/en/USAGE.md`

**Step 1: Replace outdated support list entries**

예:
- `Gemini CLI (@google/gemini-cli)` -> `Antigravity CLI (@sanchaymittal/antigravity-cli, binary: agy)`

**Step 2: Update env var examples**

예:
- `OCT_ENABLED_TOOLS=codex,gemini` -> `OCT_ENABLED_TOOLS=codex,agy`

**Step 3: Add compatibility note**

문서에 짧게 명시:

```text
Legacy config values like gemini/gemini-cli are still accepted and normalized to agy.
```

**Step 4: Decide env var naming policy**

아래 둘 중 하나를 문서에 명확히 선택:
- `OCT_GEMINI_*` env vars를 legacy compatibility로만 문서화하고 deprecated 표시
- 또는 Antigravity용 새 이름을 도입할 계획이 있으면 그 계획만 문서화하고 이번 변경에서는 deprecated note만 유지

이 단계에서 새 env var를 실제 코드에 도입하지 않는다면, 문서에서 “legacy testing override”라는 표현으로 축소한다.

---

### Task 3: CLI help text를 migration-complete 형태로 다듬기

**Objective:** `oct usage --help`와 config/help 메시지가 Antigravity 중심으로 보이게 만든다.

**Files:**
- Modify: `cmd/usage.go`
- Modify: `cmd/config.go`
- (Optional) Modify: `cmd/alert.go`

**Step 1: Keep runtime binary guidance explicit**

유지/정리할 안내:
- `Antigravity: Local session artifacts are scanned first (binary: 'agy')`

**Step 2: Remove Gemini-first wording**

도움말/에러/팁 문구에서 Gemini가 canonical provider처럼 보이는 표현 제거.

**Step 3: Add compatibility hint only where useful**

필요 시 짧게:

```text
Legacy config values 'gemini' and 'gemini-cli' still map to 'agy'.
```

---

### Task 4: 테스트를 canonical naming 기준으로 정리

**Objective:** 기본 순서/설정 예시/표시값이 Antigravity 기준으로 유지되도록 한다.

**Files:**
- Modify: `internal/usage/usage_test.go`
- Modify: `cmd/usage_test.go`
- Modify: `cmd/config_test.go`
- (Optional) Modify: `internal/update/update_test.go`

**Step 1: Update default-order tests**

`agent_order` 예시에서 `gemini` 대신 `agy` 또는 `antigravity`를 우선 사용.

**Step 2: Preserve compatibility tests separately**

별도 테스트로 유지:
- legacy `gemini` config input still resolves
- displayed provider/tool remains Antigravity/agy

**Step 3: Run targeted tests**

```bash
GOTOOLCHAIN=auto go test ./cmd ./internal/usage ./internal/update -run 'Antigravity|Gemini|Usage|Config|Tool' -v
```

Expected:
- canonical naming tests pass
- legacy alias compatibility tests still pass

---

### Task 5: Full verification and commit

**Objective:** 문서/도움말/테스트 변경이 함께 검증되도록 한다.

**Files:**
- Modify: touched files from tasks 1-4

**Step 1: Full validation**

```bash
GOTOOLCHAIN=auto go test ./...
go build -o oct main.go
./oct usage --help
```

Expected:
- tests pass
- build succeeds
- help text shows Antigravity/agy as canonical naming

**Step 2: Commit**

```bash
git add README.md README.en.md CONTEXT/ko/USAGE.md CONTEXT/en/USAGE.md cmd/usage.go cmd/config.go internal/usage/usage_test.go cmd/usage_test.go cmd/config_test.go internal/update/update_test.go
git commit -m "docs: finish antigravity naming cleanup"
```

---

## Exit criteria

- README / CONTEXT / help text에서 Antigravity가 canonical provider로 보임
- legacy `gemini`는 compatibility note 또는 테스트 범위로만 남음
- 기본 예시와 테스트 입력이 `agy`/`antigravity` 기준으로 정리됨
- 기존 사용자 config compatibility는 유지됨
