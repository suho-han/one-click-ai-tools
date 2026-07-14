# Package Manager Stability Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** `cargo`, `go install`, `pip`, `pnpm`, `yarn`, `brew`, installer-specific flows를 계속 지원하되, 추가 패키지 매니저를 늘리기 전에 설치/업데이트/버전 감지/실행 바이너리 일관성을 안정화한다.

**Architecture:** package manager 확장은 기능 추가보다 식별 체계 정리가 먼저다. `Tool`이 가진 display name / install package / runtime binary / legacy alias를 분리하고, manager detection은 "무엇이 설치되었는가"보다 "무엇을 실행할 것인가"를 우선 확인해야 한다. npm wrapper/postinstall 경로와 Go updater 경로를 같은 안정성 계약으로 묶는다.

**Tech Stack:** Go 1.25, Cobra CLI, Node postinstall wrapper, npm publish, go test, go build

---

## Repository observations

- `package.json`은 npm 패키지이고 실제 실행 entry는 `scripts/oct-wrapper.js`다.
- npm 설치는 `scripts/postinstall.js`가 GitHub release tarball을 받아 `bin/oct`를 배치하는 구조다.
- wrapper는 `bin/oct`만 직접 실행하고, 없으면 postinstall 재실행으로 self-heal 한다.
- Go updater 쪽 `internal/update/manager.go`는 `npm`, `brew`, `pnpm`, `yarn`, `cargo`, `go-install`, `pip`, `cursor-agent`를 이미 enum으로 갖고 있다.
- 하지만 manager detection은 여전히 package-manager별 list 명령 성공 여부에 크게 의존한다.
- `Tool`에는 `Package`, `BinaryName`, `BinaryAliases`, `BrewPackage`가 섞여 있어 “설치 package”와 “실행 binary”와 “표시 canonical name”이 완전히 분리되어 있지 않다.
- 최근 Antigravity 작업에서 실제 npm package, 실제 binary, canonical binary naming이 어긋나면 UX/검증/실기기 동작이 쉽게 갈라진다는 것이 확인됐다.
- release checklist는 npm publish 안정성만 다루고 있고, multi-manager update path regression은 별도 체크리스트가 없다.

## Core stability risks

### 1. Identity mismatch risk
- 한 tool에 대해 아래 4개가 뒤섞이면 회귀가 난다.
  1. 표시 이름
  2. 설치 package
  3. 실제 실행 binary
  4. legacy alias
- 예: package 이름은 맞는데 binary가 다르거나, binary alias는 허용되는데 installer가 다른 package를 가리키는 경우.

### 2. Detection drift risk
- `DetectManager()`는 실제 설치 provenance보다 `brew list`, `pnpm list -g`, `yarn global list`, `npm list -g` 결과 순서에 의존한다.
- 같은 binary가 여러 manager 경로에 동시에 존재할 때, “어떤 manager로 업데이트해야 안전한가”를 잘못 고를 수 있다.

### 3. Wrapper/postinstall split-brain risk
- npm 설치 경로는 Node wrapper + downloaded binary 구조다.
- Go updater 경로는 각 manager install command를 직접 호출한다.
- manager를 늘리면 둘이 서로 다른 binary path / version source / recovery behavior를 가질 수 있다.

### 4. Version parsing fragility
- `GetInstalledVersion()`는 manager별 stdout 문자열 파싱에 의존한다.
- 새 manager 추가 시 parser가 없거나 형식이 바뀌면 “업데이트 성공했는데 no-change/error로 오판”할 수 있다.

### 5. Unsafe fallback risk
- unknown일 때 기본 `npm install -g`로 떨어진다.
- package prefix 누락이나 detection 실패가 있으면 실제 의도와 다른 manager로 재설치할 수 있다.

### 6. Scheduler/install coupling risk
- postinstall은 install 직후 `schedule enable --task session-refresh`까지 실행할 수 있다.
- 설치된 binary path와 schedule에 박힌 binary path가 다르면 새 manager 지원 시 더 크게 깨진다.

---

## Design principles before adding more managers

1. **Canonical identity first**
   - tool마다 다음 필드를 논리적으로 분리한다.
   - `DisplayName`
   - `InstallPackage`
   - `RuntimeBinary`
   - `LegacyAliases`
   - `PreferredManagers` (optional)

2. **Runtime binary is the source of truth**
   - 어떤 manager로 설치했는지보다, 현재 무엇이 실행되는지부터 확인해야 한다.
   - install/update/schedule/session-refresh verification은 실제 binary path 검증을 포함해야 한다.

3. **No silent manager fallback for ambiguous tools**
   - detection 실패 시 무조건 npm으로 가지 말고, ambiguity를 surfaced warning/error로 돌려야 한다.

4. **Manager support requires 4 contracts**
   - install command
   - installed-version probe
   - no-change output detection
   - path/ownership verification

5. **npm wrapper path is special-case infrastructure**
   - 일반 manager처럼 보이지만 실제로는 wrapper + downloaded release binary 조합이다.
   - 따라서 npm은 “package manager”이면서 동시에 “installer transport”다.

---

## Recommended scope split

### Phase A: Stabilize existing abstraction
목표: manager 종류를 늘리기 전에 현재 구조의 오판 가능성을 줄인다.

- `Tool` metadata semantics 명문화
- detection/installation/version/source-of-truth 계약 문서화
- ambiguity handling 추가
- binary path verification 테스트 보강

### Phase B: Add provenance-aware detection
목표: 현재 실행 binary가 어느 manager prefix에 속하는지 먼저 본다.

- `exec.LookPath(binary)` 기반 실제 경로 수집
- 경로가 npm global / pnpm global / Homebrew cellar / cargo bin / GOPATH/bin / pip script dir 중 어디에 있는지 분류
- package-manager list command는 secondary confirmation으로만 사용

### Phase C: Add managers only after contracts exist
목표: 새로운 manager는 같은 체크리스트를 모두 만족할 때만 추가

- install command
- version probe
- no-change detection
- path ownership probe
- local regression test
- one smoke test on real machine if installer semantics are unusual

---

## Bite-sized implementation plan

### Task 1: Tool identity schema inventory 문서화

**Objective:** 현재 각 tool의 package/binary/alias/manager semantics를 표로 확정한다.

**Files:**
- Inspect: `internal/update/tools.go`
- Inspect: `internal/update/manager.go`
- Create: `docs/plans/package-manager-inventory.md` or append to this plan in a follow-up task

**Step 1: Build per-tool table**

필수 컬럼:
- tool slug
- display name
- install package
- runtime binary
- binary aliases
- brew package
- intended manager priority

**Step 2: Verification**

Run:
```bash
GOTOOLCHAIN=auto go test ./internal/update -v
```

Expected:
- inventory 작업으로 기존 semantics를 깨지 않음

---

### Task 2: DetectManager 설계를 provenance-first로 바꾸는 테스트 추가

**Objective:** 실제 binary path가 manager 선택의 우선 근거라는 점을 failing test로 고정한다.

**Files:**
- Modify: `internal/update/manager_test.go`
- Modify: `internal/update/manager.go`

**Step 1: Write failing tests**

테스트 케이스 예시:
- binary path가 Homebrew cellar면 Brew 선택
- binary path가 npm global prefix면 Npm 선택
- list command 여러 개가 성공해도 path owner 우선
- provenance 불명 + prefix 없는 package면 Unknown 유지

**Step 2: Run targeted test**

```bash
GOTOOLCHAIN=auto go test ./internal/update -run 'DetectManager|PreferredBinaries|Provenance' -v
```

Expected:
- 초기에는 fail 가능

**Step 3: Minimal implementation**
- `LookPath` injection point 도입
- prefix classifier helper 추가
- detection fallback 순서 재정렬

**Step 4: Verify pass**

```bash
GOTOOLCHAIN=auto go test ./internal/update -run 'DetectManager|PreferredBinaries|Provenance' -v
```

---

### Task 3: Unknown/ambiguous fallback를 npm 강제에서 분리

**Objective:** detection 실패가 다른 manager 재설치로 이어지지 않게 막는다.

**Files:**
- Modify: `internal/update/manager.go`
- Modify: update flow caller(s) that consume `DetectManager()`
- Test: `internal/update/manager_test.go`

**Step 1: Write failing test**
- ambiguous tool on multi-manager system should not auto-select npm

**Step 2: Implement**
- `Unknown` 유지
- caller에서 warning + explicit recommendation 제공
- 정말 npm-only tool만 npm default 허용

**Step 3: Verify**

```bash
GOTOOLCHAIN=auto go test ./internal/update -run 'Unknown|Ambiguous|Npm' -v
```

---

### Task 4: Manager contract helper 추출

**Objective:** 새 manager 추가 시 필요한 4계약을 한 곳에서 강제한다.

**Files:**
- Modify: `internal/update/manager.go`
- Create: optional `internal/update/manager_contract.go`
- Test: `internal/update/manager_test.go`

**Step 1: Introduce explicit helper surface**
- install command provider
- version probe provider
- no-change matcher
- provenance matcher

**Step 2: Move manager-specific literals under one switch/table**

**Step 3: Verify**

```bash
GOTOOLCHAIN=auto go test ./internal/update -v
```

---

### Task 5: npm wrapper/install path regression tests 보강

**Objective:** npm path가 새 manager 확장 중에도 특수 동작을 유지하게 한다.

**Files:**
- Modify: `scripts/postinstall.js`
- Modify: `scripts/oct-wrapper.js`
- Add test harness if feasible under `scripts/` or document shell verification in `docs/release-checklist.md`

**Step 1: Verify current binary path contract**
- wrapper always executes package-local `bin/oct`
- self-heal only restores package-local binary
- postinstall schedule wiring passes the just-installed binary path

**Step 2: Add regression coverage or documented release checks**

**Step 3: Verify**

```bash
npm pack --dry-run
GOTOOLCHAIN=auto go test ./...
go build -o oct main.go
```

---

### Task 6: Real-manager support matrix 문서 추가

**Objective:** 앞으로 manager 추가 시 필요한 체크리스트를 명문화한다.

**Files:**
- Modify: `docs/release-checklist.md`
- Create: `docs/package-manager-support-matrix.md`

**Include:**
- manager name
- supported OS
- install command
- version command
- no-change pattern
- path ownership rule
- real-machine smoke-test requirement

---

## Acceptance criteria

- `Tool` identity semantics가 문서/테스트에서 명확하다.
- manager detection이 actual binary provenance를 우선한다.
- ambiguous detection이 silent npm fallback로 이어지지 않는다.
- npm wrapper/postinstall 경로의 binary-path correctness contract가 유지된다.
- 새로운 manager를 추가할 때 필요한 contract checklist가 문서화된다.

## Recommended first implementation slice

가장 먼저 할 일:
1. `internal/update/manager_test.go`에 provenance-first failing tests 추가
2. `internal/update/manager.go`에서 silent npm fallback 제거 또는 축소
3. `docs/release-checklist.md`에 manager support matrix 링크/검증 항목 추가

이 3개가 끝나면 그 다음부터 `cargo`/`go install`/`pip` 확장은 비교적 안전하게 들어갈 수 있다.
