# Menubar UX Design

## Goal
`oct menubar`를 “터미널 명령 바로가기”에서 “현재 usage 상태를 한눈에 보고 필요한 액션으로 짧게 진입하는 status item”으로 바꾼다.

## Primary UX path

1. 사용자가 menubar 아이콘/텍스트를 본다.
2. 아이콘/텍스트에서 대략적 상태를 즉시 파악한다.
   - `oct` = 대체로 OK
   - `oct !` = warning 있음
   - `oct !!` = error 있음
   - `oct …` = refresh 중 / 초기 로딩
3. 메뉴를 열면 맨 위 두 줄에서 요약과 마지막 refresh 시각을 본다.
4. provider별 compact 상태를 본다.
5. 필요 시 아래 action으로 이동한다.

## Menu structure

### Top summary block (read-only)
- `6 providers · 4 ok · 1 warn · 1 error`
- `Last refresh: 13:52:10`

### Provider block (read-only)
예시:
- `Claude · 5h 42% · 7d - · ok`
- `Codex · 5h 88% · 7d 64% · warn`
- `Copilot · 5h - · 7d - · error`

원칙:
- provider line은 **짧고 스캔 가능**해야 한다.
- 상세 message 전체를 menubar에 다 넣지 않는다.
- summary-first, provider-detail-second 구조 유지.

### Action block
- `Refresh now`
- `Open Usage`
- `Run Session Refresh`
- `Open Monitor`
- `Run Alert Check`
- `Quit`

## Interaction rules

- startup 시 자동 refresh 1회 실행
- 이후 `menubar_refresh_interval`(default `5m`) 기준으로 자동 refresh 반복
- `Refresh now` 클릭 시 중복 refresh 금지
- refresh 중에는 summary/title을 loading 상태로 전환
- provider row는 compact summary이고, submenu에서 source/message/bucket detail을 본다
- refresh 실패 시 provider block은 숨기지 않고, summary/last refresh에 실패 흔적을 남긴다
- heavy/interactive views는 menubar 내부에서 재구현하지 않고 기존 CLI로 넘긴다

## Why this design

### Keep menubar thin
menubar는 full dashboard가 아니라 glanceable status surface여야 한다. TUI를 menubar 안에 억지로 넣기보다, 요약은 menubar에서 보고 상세는 CLI로 여는 방식이 유지보수에 유리하다.

### Summary before action
사용자는 먼저 “지금 위험한가?”를 보고 싶다. 그래서 action보다 summary block이 위에 와야 한다.

### Current executable over PATH
menubar에서 `oct usage` 같은 plain PATH 호출을 쓰면 다른 글로벌 설치를 탈 수 있다. 따라서 action은 현재 실행 binary path 기준으로 열어야 한다.

## Next iteration from reference UI

참고한 menubar UI는 `header + provider cards + footer actions` 구조가 명확하다. `oct menubar`에는 그대로 복제하지 않고, 현재 compact status menu를 유지하면서 아래 요소를 차용한다.

### Header refinements
- 현재의 text-only summary block은 유지하되, 시각적 title line을 추가하는 방향을 검토한다.
- 후보:
  - `Usage Overview`
  - `one-click tools`
  - `Provider Status`
- 오른쪽 단일 global action(`+` 또는 gear)은 후속 단계에서 `Settings` 또는 `Add Provider` 진입점으로 검토한다.

### Provider rows -> lightweight cards/subsections
- 지금의 provider row submenu 구조는 유지한다.
- 다만 dropdown 안에서 provider를 더 잘 구분할 수 있도록 다음 중 하나를 검토한다.
  - provider별 separator/title 강조
  - 상태 badge(`ok`/`warn`/`error`)를 더 눈에 띄는 label로 표현
  - provider identity(`Claude`, `Codex`, `Copilot`)를 usage 값보다 먼저 보이도록 정렬 강화
- full custom card UI를 즉시 구현하기보다, macOS menu native constraints 안에서 "card-like grouping"으로 먼저 해석한다.

### Footer action grouping
- 현재 action block은 기능적으로 맞다.
- 다음 단계에서는 footer-like grouping으로 정리한다.
  - primary: `Refresh now`
  - navigation: `Open Usage`, `Open Monitor`
  - maintenance: `Run Session Refresh`, `Run Alert Check`
  - exit: `Quit`
- `Settings`는 실제 설정 진입 경로가 준비되면 추가한다.

### Information hierarchy to preserve
- 1차: 전체 상태(summary / last refresh / auto refresh)
- 2차: provider별 상태
- 3차: provider submenu detail
- 4차: 전역 action

### Non-goal for next step
- 웹/SwiftUI처럼 자유로운 완전 custom popover 재구현은 이번 menubar 범위 밖이다.
- 우선은 native menu 안에서 정보 계층과 시각적 그룹핑을 개선하는 데 집중한다.

## Deferred ideas (not in this step)

- alert enable/disable toggle
- config open shortcut
- launch-at-login / launch agent wiring
- menubar 아이콘 이미지 적용
- header title / settings entry
- provider badge or card-like grouping polish

## Validation note
현재 Linux host에서는 macOS systray runtime을 직접 검증할 수 없다. 이번 단계의 성공 조건은 다음이다.
- helper/unit tests pass
- Linux host build passes
- macOS runtime verification remains an explicit follow-up item
