# Menubar Custom UI (NSPopover/SwiftUI) Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** `oct menubar`를 기본 `NSMenu` 목록형에서 벗어나, `NSStatusItem + NSPopover + SwiftUI` 기반의 custom menubar UI로 전환한다.

**Architecture:** 기존 Go `oct` binary는 usage/session-refresh/monitor/alert 실행과 JSON 출력 책임을 유지한다. 새 macOS 전용 Swift helper 앱이 메뉴바 아이콘, popover UI, 주기 refresh, action routing을 담당하고 필요할 때 `oct usage --json` 등 기존 CLI를 호출한다.

**Tech Stack:** Swift 6, SwiftUI, AppKit (`NSStatusItem`, `NSPopover`), Foundation `Process`, existing Go CLI (`oct`)

---

## Verified environment reality

- Local Linux host: `swift --version` 실패 (`swift: command not found`)
- Remote macOS host (`macmini-tailscale`): `Swift 6.3.1` 사용 가능
- 결론:
  - 계획/문서/파일 scaffold는 현재 host에서도 가능
  - 실제 compile/run 검증은 remote macOS host에서 수행해야 함

## Why a new lane is required

- 현재 menubar는 `github.com/getlantern/systray` 기반이며 내부적으로 `NSStatusItem + NSMenu`를 사용한다.
- 이 구조는 native menu item / submenu / separator 렌더링만 제공한다.
- 참고 UI처럼 card-like header, grouped rows, richer footer, custom spacing, progress, badge styling은 `NSMenu`만으로 재현하기 어렵다.
- 따라서 “native menu 재해석”이 아니라 “custom popover lane”으로 분기해야 한다.

## Proposed repo shape

### New macOS app surface
- Create: `macos/OctMenubar/Package.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/OctMenubarApp.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/StatusBarController.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/PopoverView.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/ViewModels/UsageViewModel.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/Models/UsageSnapshot.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/Services/OctCLIService.swift`
- Create: `macos/OctMenubar/Tests/OctMenubarTests/...`

### Existing Go surface kept
- Keep: `cmd/menubar*.go` as legacy/native-menu mode initially
- Later option:
  - `oct menubar` -> launch Swift helper if installed
  - `oct menubar --legacy` -> old systray/NSMenu mode

## UX mapping from reference UI

### Primary path
1. 메뉴바 아이콘 클릭
2. custom popover 표시
3. 상단 header에서 전체 상태 인지
4. provider cards/list에서 usage 상태 확인
5. footer action으로 CLI 기능 진입

### Information hierarchy
1. Header
   - app title (`Usage Overview` or `one-click tools`)
   - summary counts
   - last refresh
   - next refresh
2. Provider section
   - provider identity
   - status badge
   - compact metrics (`5h`, `7d`)
   - warning/error preview
3. Footer action section
   - Refresh now
   - Open Usage
   - Open Monitor
   - Run Session Refresh
   - Run Alert Check
   - Quit

### Non-goals for first custom-ui step
- multi-window settings app
- drag/drop card reorder
- live charts/animations beyond light polish
- replacing Go usage logic with Swift-native fetchers

---

## Data contract between Swift helper and Go CLI

### Command source of truth
- `oct usage --json`
- `oct monitor --once`
- `oct session-refresh`
- `oct usage --notify`

### Negative contract
- periodic refresh must **not** submit prompts
- periodic refresh must **not** intentionally consume model tokens
- periodic refresh must only read existing usage/account state exposed by current CLI paths

### Required modeling step
Swift helper must decode a stable projection instead of binding directly to raw CLI output everywhere.

Suggested model split:
- raw CLI decode model
- UI projection model (`UsageSnapshot`, `ProviderCardModel`)

---

## Task breakdown

### Task 1: Lock the Swift-helper architecture and launch contract

**Objective:** custom popover app과 기존 `oct` binary 사이 책임 경계를 고정한다.

**Files:**
- Modify: `PROJECT_CONTEXT/menubar-ux-design.md`
- Modify: `PROJECT_CONTEXT/menubar-custom-ui-plan.md`

**Step 1: Record launch modes**
- `oct menubar` legacy 유지 여부
- Swift helper binary 이름
- helper가 `oct` path를 어떻게 찾을지 정의

**Step 2: Record verification lanes**
- Linux: docs / scaffolding only
- macOS: build / run / visual verification

**Step 3: Commit**
```bash
git add PROJECT_CONTEXT/menubar-ux-design.md PROJECT_CONTEXT/menubar-custom-ui-plan.md
git commit -m "docs(menubar): plan custom popover migration"
```

---

### Task 2: Scaffold Swift Package menubar app

**Objective:** `NSStatusItem + NSPopover + SwiftUI App` 최소 골격을 만든다.

**Files:**
- Create: `macos/OctMenubar/Package.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/OctMenubarApp.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/StatusBarController.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/PopoverView.swift`

**Step 1: Write failing compile expectation**
Run on macOS:
```bash
cd macos/OctMenubar
swift build
```
Expected: FAIL until files are scaffolded correctly

**Step 2: Implement minimal app shell**
- status bar item 생성
- button click 시 popover toggle
- placeholder SwiftUI view 노출

**Step 3: Re-run build**
```bash
cd macos/OctMenubar
swift build
```
Expected: PASS

**Step 4: Commit**
```bash
git add macos/OctMenubar
git commit -m "feat(menubar): scaffold swift popover app"
```

---

### Task 3: Add CLI bridge service and decode model

**Objective:** Swift helper가 기존 `oct usage --json`를 호출하고 strongly-typed snapshot으로 변환한다.

**Files:**
- Create: `macos/OctMenubar/Sources/OctMenubarApp/Services/OctCLIService.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/Models/UsageSnapshot.swift`
- Create: `macos/OctMenubar/Tests/OctMenubarTests/UsageSnapshotTests.swift`

**Step 1: Write failing tests**
- sample JSON decode
- status projection (`ok` / `warn` / `error`)
- next refresh label derivation

**Step 2: Run tests to verify failure**
```bash
cd macos/OctMenubar
swift test --filter UsageSnapshotTests
```

**Step 3: Implement minimal bridge**
- `Process` wrapper
- stdout capture
- decode + projection

**Step 4: Re-run tests**
```bash
cd macos/OctMenubar
swift test --filter UsageSnapshotTests
```
Expected: PASS

**Step 5: Commit**
```bash
git add macos/OctMenubar
git commit -m "feat(menubar): bridge swift helper to oct usage json"
```

---

### Task 4: Build reference-style popover UI

**Objective:** header / provider section / footer action 구성을 SwiftUI로 구현한다.

**Files:**
- Modify: `macos/OctMenubar/Sources/OctMenubarApp/PopoverView.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/Views/HeaderView.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/Views/ProviderCardView.swift`
- Create: `macos/OctMenubar/Sources/OctMenubarApp/Views/FooterActionsView.swift`

**Step 1: Add static mock-driven previews**
- header
- 2~3 provider cards
- footer buttons

**Step 2: Bind live view model**
- loading state
- success state
- error state

**Step 3: Manual visual verification on macOS**
- popover width/spacing
- provider card readability
- footer grouping

**Step 4: Commit**
```bash
git add macos/OctMenubar
git commit -m "feat(menubar): add swiftui custom popover layout"
```

---

### Task 5: Add timer refresh and footer actions

**Objective:** popover가 주기적으로 usage를 refresh하고 기존 CLI actions를 실행하게 한다.

**Files:**
- Modify: `macos/OctMenubar/Sources/OctMenubarApp/ViewModels/UsageViewModel.swift`
- Modify: `macos/OctMenubar/Sources/OctMenubarApp/Services/OctCLIService.swift`
- Modify: `macos/OctMenubar/Sources/OctMenubarApp/Views/FooterActionsView.swift`

**Step 1: Write failing tests for timer/view-model behavior**
- initial load triggers one refresh
- periodic refresh updates `lastRefresh`
- duplicate refresh blocked

**Step 2: Run tests to verify failure**
```bash
cd macos/OctMenubar
swift test
```

**Step 3: Implement timer + action wiring**
- default refresh interval 1m
- Refresh now
- Open Usage
- Open Monitor
- Run Session Refresh
- Run Alert Check

**Step 4: Re-run tests**
```bash
cd macos/OctMenubar
swift test
```
Expected: PASS

**Step 5: Manual validation on macOS**
- `Last refresh` and `Next refresh` visibly move
- actions launch correct `oct` commands

**Step 6: Commit**
```bash
git add macos/OctMenubar
git commit -m "feat(menubar): add timed refresh and footer actions"
```

---

### Task 6: Integrate with existing CLI packaging

**Objective:** 사용자 입장에서 custom helper를 실행/배포 가능한 상태로 연결한다.

**Files:**
- Modify: `cmd/menubar.go`
- Modify: install/release docs as needed
- Optional: create packaging script under `scripts/`

**Step 1: Decide command contract**
Options:
- `oct menubar` launches Swift helper when present
- `oct menubar --legacy` keeps old systray mode

**Step 2: Verify binary path correctness**
- generated launch path must point to actual installed helper / current `oct`
- no PATH-only assumptions

**Step 3: Build + smoke test on macOS**
```bash
go build -o oct main.go
cd macos/OctMenubar && swift build
```

**Step 4: Commit**
```bash
git add cmd/menubar.go macos/OctMenubar
git commit -m "feat(menubar): wire swift helper launch path"
```

---

## Validation plan

### Linux host
- docs updated
- file scaffolding reviewed
- Go tests/build remain green

### macOS host
- `swift build`
- `swift test`
- helper launch success
- popover visual inspection
- timer refresh visible
- footer action clicks verified

### Explicit gap policy
- Linux host에서는 Swift compile/runtime verified라고 주장하지 않는다.
- GUI visual verification은 반드시 macOS host에서 수행한다.

## Exit criteria

- custom menubar UI가 `NSPopover + SwiftUI`로 표시된다.
- header / provider section / footer actions가 참고 UI와 유사한 정보 구조를 가진다.
- Swift helper가 기존 `oct usage --json`를 읽어 상태를 갱신한다.
- periodic refresh가 visible (`last refresh`, `next refresh`) 하다.
- action 버튼이 기존 CLI workflow를 실행한다.
- macOS host에서 build/test/run/visual verification이 완료된다.
