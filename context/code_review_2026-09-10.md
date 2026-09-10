# Code Review & Improvement Backlog (2026-09-10)

Full-codebase structure survey plus code/UI/CLI UX improvement notes. Review
baseline: v0.1.5 (`d4842af`).
Written while `go vet` was clean and the full test suite passed, so items below
are maintainability/robustness/UX issues, not "it's broken" reports.
**Note: `file:line` references are anchored to that commit and will drift as
new commits land.**

Three structural root problems:
1. `internal/usage` / `internal/update` lack interface abstraction → heavy duplication
2. The menubar exists twice: Go (legacy systray) and Swift (`macos/OctMenubar`)
3. CLI output discipline (exit codes, stderr, `--json`) differs per command

---

## 1. Structure summary

- `main.go` → `cmd/`: Cobra commands (usage, monitor, menubar, config, alert, schedule, agent-update, session-refresh, doctor, release-doctor, update)
- `internal/usage/`: usage collection for 8 AI providers (13 source files, largest package)
- `internal/update/`: package-manager-based tool updates + self-update in `cmd/update.go`
- `internal/` rest: config, schedule (per-platform), notify, sessionrefresh, execenv, netclient, ui
- `macos/OctMenubar/`: SwiftUI menubar app; invokes the Go binary via `oct usage --json` subprocess
- Size: ~31,000 lines Go / ~2,800 lines Swift

---

## 2. Code improvements

### 2-1. usage package: missing abstraction and heavy duplication (top priority)

- **Adding a provider requires edits in 5 places**: the `fetchers` map (`internal/usage/usage.go:68`), default order (`usage.go:58`), color switch (`usage.go:126-149`), display labels (`usage.go:175-192`), and `compactProviderLabel` (`compact.go:35-57`) — all keyed on substring matching of provider names. Consolidate into a `Provider` interface with `Name()`/`Fetch(ctx)` plus one registry holding display info, and a new provider becomes a single new file.
- **Two HTTP client paths**: claude/codex/cursor/copilot go through `netclient` (with retry/timeout policy); commandcode (`commandcode.go:239`) and opencode (`opencode.go:165`) build private `http.Client`s duplicating the same fetch code → silently lose retry policy. Unify behind a shared `getJSON()` helper.
- **Four percent-calculation implementations**: `usage.go:355` (`remainingFromUsed`), `compact.go:112` (`compactPercentLabel`), `gemini.go:163` (`percentUsedFromRemaining`), `commandcode.go:443` (`clampPercent`). Any disagreement shows wrong numbers directly to users.
- **6 latent nil-derefs from swallowed errors**: `req, _ := http.NewRequest(...)` then used unchecked — `claude.go:141`, `cursor.go:55`, `copilot.go:131,162`, `plan.go:175,190`. Also 8 sites of `body, _ := io.ReadAll(...)` (claude.go:179 etc.).
- More duplication: raw `os.Getenv("OCT_USAGE_DEBUG") == "1"` checks in 14 places (the helper `osDebugEnabled` exists at gemini.go:174 but is used once), two exec-with-timeout implementations (`cli_fallback.go:12` / `plan.go:111`), duplicated CLI-fallback scaffolding (claude.go:239 / gemini.go:47), `maxFloat` (`commandcode.go:436`) replaceable by the builtin `max` (Go 1.25).
- Naming inconsistencies: `parseCursorAPIResponse` vs `parseCursorUsageResponse` (cursor.go) — near-identical names, different purposes. The macOS-only `security` keychain call has no GOOS gate (`claude.go:54`).

### 2-2. No timeout ceiling (directly affects UX)

- `usage.GetUsage` fans out with errgroup, but every goroutine returns `nil`, making the `g.Wait()` error branch dead code (`usage.go:88-93`). There is no overall deadline; worst case is netclient 30s×4 retries + backoff + 20s CLI fallback + 3s `gh`, stalling the `oct usage` spinner and the menubar refresh together. Set a ceiling with `errgroup.WithContext` and thread ctx into HTTP/exec calls.
- Same in update: `Run()` waits on `context.Background()` forever (`internal/update/update.go:96`); version probes (`manager.go:148-196`) are unbounded and uncancellable. A hung npm hangs all of oct.
- Copilot makes 2 extra GitHub API calls just to fill `PlanSource` (`plan.go:170-206`, invoked from copilot.go:31 and :103) — doubles the latency budget.

### 2-3. Self-update checksum is fail-open (security, one-line fix)

`verifyReleaseAssetChecksum` is implemented, but its only production call passes `require=false` (`cmd/update.go:225`) → a failed checksums.txt download or missing entry silently skips verification. Switch to fail-closed. Also note the self-update HTTP client's 30s whole-request timeout (`cmd/update.go:45`) can abort a large archive download mid-`io.Copy`.

### 2-4. update package: enum + scattered switches

- `Manager` is a string enum, so each new package manager touches 6–7 switch sites. The detection cascade (`manager.go:50-96`) and its de-facto clone `explainResolvedManager` (`update.go:248-282`) must be kept in sync by hand.
- On failure it returns `fmt.Errorf("%w: %d tool(s) failed", errors.New("update failed"), failureCount)` (`update.go:149`) — loses which tools failed. Return structured per-tool results (name/error/before-after versions).
- The `agent-update` help text says "parallelly" but the loop is sequential (`agent_update.go:20`). `brewInstallMu` is a vestigial mutex guarding nothing (`update.go:20`).
- `Run` always calls `ExplainPlans`, probing each tool's version twice (`update.go:66` vs `:113`).
- Of the two y/N prompt implementations, the internal one lacks terminal detection (`update.go:188`) — non-interactive runs hang until EOF.

### 2-5. Dead code & placebo tests cleanup

- Dead code: `cmd/config.go:420` `visibleLen` (monitor uses its own `visibleLenANSI`), `internal/ui/image.go:81` `PrintIcon`, `internal/schedule/linux.go:100` `filepathJoin`, `internal/config/config.go:13` unused `Config` struct, all of `internal/update/command_env.go`, the no-op `if statePath == "" { statePath = "" }` at `cmd/alert.go:111-114`, and `verifyReleaseAssetChecksum`'s unreachable `require=true` branches.
- **Placebo test**: `internal/config/config_test.go:29-50` never calls `MigrateLegacyConfig` — it re-implements the logic inline and compares hardcoded values against themselves, leaving the package's only real function effectively untested. Migration also moves only `enabled_tools` and silently drops other legacy keys (`config.go:50-58`).
- Coverage gaps: `update.Run()` orchestration, the npm fallback chain (`update.go:318-413`), the self-update verification path (the riskiest code), `FetchCopilotUsage` token-resolution chain, `state_snapshot.go`, `cli_fallback.go`, `GetUsage`/`PrintTable`/`PrintJSON`.
- Scheduler test imbalance across platforms: linux has injectable seams (`schedule.go:43-56`) and real tests; macos (`macos.go:103-126`) and windows (`windows.go:23-52`) call exec directly and are untested. Apply the same seam pattern.

### 2-6. Misc

- `internal/notify`: swallows state-file errors (`usage_alert.go:67`) → cooldowns silently reset → alert-spam risk; send failures are never recorded (`:104-108`). `snoozeKey`/`parseHHMM`/`computeAlertPriority` are triplicated across cmd/alert.go.
- `internal/execenv`: the `LookPath` fallback doesn't check the exec bit (`execenv.go:97-109`) → non-executable files count as "found".
- `internal/netclient`: uses deprecated `net.Error.Temporary()` (`client.go:57`); sleep isn't injectable, so tests take 6.6s.
- `internal/ui/image.go`: four effectively-copied renderers (`:331-409`); `commandcode.png` asset is missing (see `tools.go:86`) — no tool↔asset mapping test.
- GitHub token stored in plaintext in config.yaml (`writeConfig` chmods 0600, which mitigates; a keychain option is worth considering).
- `macos.go:5`: uses `html/template` for an XML plist — works, but semantically wrong package.

---

## 3. UI improvements — Menubar & Settings

### 3-1. The dual menubar implementation is the biggest problem

The Go legacy menubar (systray/NSMenu, `cmd/menubar_darwin.go`) and the Swift app (`macos/OctMenubar`) implement the same features entirely separately: usage fetch, refresh loop, title mode, 4 actions (usage/monitor/session-refresh/alert), and provider rendering all exist twice. Swift is the more polished one (popover + settings UI), so **converge on Swift and keep legacy only as a `--legacy` debug path**.

Ongoing mismatches caused by the split:
- **Refresh interval**: the Go menubar honors `menubar_refresh_interval` (`menubar_state.go:257`); the Swift app hardcodes 60s (`OctCLIService.swift:7`) — config changes don't reach the Swift app.
- **Refresh UX**: the Go menubar immediately overwrites live data with a loading snapshot on every refresh (`menubar_runtime_darwin.go:69`) → everything flickers to "loading…" every 60s. Swift keeps previous data with a spinner (the correct pattern).
- Missing helper silently falls back to legacy with **no warning** (`menubar_darwin.go:26-30`) — users can't tell why their menubar looks different.
- Helper discovery walks up to 6 parent directories from cwd/exec path (`menubar_helper.go:96-107`) → a dev build can be launched from anywhere inside the repo.

### 3-2. Swift app

- **Synchronous process execution on the main thread**: `SettingsView`'s `loadConfiguration`/`saveConfiguration` run `oct config list --json` synchronously from `onAppear` (`SettingsView.swift:22,73-85`). With a 20s process timeout (`OctCLIService.swift:12`), a slow or missing oct freezes the settings window up to 20s (beach ball). Move to a background task.
- **runProcess polls and reads pipes only after exit**: waits via `while process.isRunning { Thread.sleep(0.05) }` (`OctCLIService.swift:203-210`) and reads the pipes after the process exits. If output exceeds the pipe buffer (64KB), the process blocks on write and gets misdiagnosed as a timeout. Start `readabilityHandler` or a background `readDataToEndOfFile` first.
- **Config re-fetched every refresh**: `fetchUsageSnapshot` additionally runs `config list --json` every 60s (`OctCLIService.swift:34`), with errors swallowed by `try?` — a failed config read silently reverts the title mode to default. Reading config once at launch and on settings save is enough.
- **Action failure overwrites data**: `UsageViewModel.runAction` replaces the whole snapshot with `.error` on failure (`UsageViewModel.swift:38-44`) → one failed terminal launch wipes all usage cards. Separate action feedback from data state (the settings tab's `lastActionFeedback` already does this right).
- Popover size is computed from hardcoded estimates (`PopoverView.swift:10-11`, chrome 294pt + card 122pt) — drifts when content changes. Just set `maxHeight` and let SwiftUI size it.
- Terminal launching is Terminal.app-only (`open -a Terminal`, `OctCLIService.swift:167-171`). The Go legacy `runInTerminal` ignores osascript errors via `_ =` (`menubar_runtime_darwin.go:131-133`).
- Done well: settings draft-vs-source diff feedback, zero-enabled-providers warning, accessibility labels, revert, and a 633-line UsageSnapshot test suite.

### 3-3. Settings

- **Swift settings ↔ CLI settings feature gap**: the Swift settings tab covers only Providers/display mode/session-refresh. Alert thresholds & quiet hours, scheduling, and the GitHub token can't be edited from the Swift UI → even menubar users must drop back to the CLI. At minimum add entry points/guidance.
- Closing and reopening the settings window reloads via `onAppear`, **losing unsaved drafts**.

### 3-4. CLI settings TUI (`oct config`)

- **No spacebar toggle** — only Enter toggles (`cmd/config.go:99`). No `j`/`k` navigation either.
- **No reordering**: it persists `agent_order` but offers no in-TUI reordering (arrows exist only in Swift settings).
- **`promptUsageMode` hardcodes remaining as the default** (`config.go:297-299`) — re-running `oct config` always re-asks the mode.
- **No non-TTY guard**: piping/scripting `oct config` breaks the bubbletea alt-screen. Reuse `oct usage`'s TTY detection pattern (`usage.go:129-138`).
- The GitHub token prompt echoes input (no masking, `config.go:349-353`).

---

## 4. CLI UX improvements

### 4-1. Commands that exit 0 on failure (systemic problem)

- `monitor`, all of `alert`, all of `schedule`, and the `config` tree use `Run` (printing errors themselves via fmt.Println) → **exit code 0 even on failure** (e.g. `cmd/alert.go:50`, `cmd/schedule.go:19`, `cmd/config.go:432`). Meanwhile usage/doctor/release-doctor use `RunE`.
- Root prints errors to stdout (`cmd/root.go:26`).
- Converting all to `RunE` + routing root errors to stderr fixes exit codes, stdout pollution, and raw-error output in one pass (~25 call sites).

### 4-2. `--json` flag meaning collision

- `oct config update --json '<payload>'` (`config_api_command.go:49`) uses `--json` as an **input payload** while every other command uses it as an output toggle. The Swift app depends on this path, so an immediate rename is hard, but migrate to `--payload` or stdin.
- `monitor` has no `--json` at all (though it periodically writes a JSON snapshot file, `monitor.go:48`); `alert config show` is conversely JSON-only (`alert.go:32-40`). Standardize on `usage`'s "auto-JSON when non-TTY" pattern.

### 4-3. monitor's pipe-friendliness

Piping `monitor --once` emits ANSI clear-screen codes plus a "Ctrl+C to stop" line (`cmd/monitor.go:83,132`). Detect TTY and emit plain text for one-shot/non-interactive runs.

### 4-4. Help & naming

- **No command defines an `Example:` field.** Start with flag-heavy `schedule`, `alert`, `monitor`. `monitor`/`schedule`/`alert` also lack Long descriptions.
- `oct update` (self-update) vs `oct agent-update` (tool updates) — inverted intuition. Long term, route as `oct self-update` or `oct update --self`.
- `schedule enable --interval`'s "Update interval" description actually covers any task's interval (`schedule.go:199`). `schedule config` only supports session-refresh but says so only via a runtime message (`schedule.go:100-102`).
- Success wording drift: "Config updated." (`config.go:487`) vs "Config updated successfully." (`config.go:446`) vs "alert config updated." (`alert.go:57`).
- Raw lowercase errors shown to users: schedule (`schedule.go:19,24,44-52`), monitor (`monitor.go:38`), and session-refresh exposing raw CombinedOutput (`internal/sessionrefresh/sessionrefresh.go:334-340` → `cmd/session_refresh.go:190`).

---

## 5. Recommended priorities (overall Top 10)

| # | Improvement | Area | Why |
|---|---|---|---|
| 1 | Checksum fail-closed (`cmd/update.go:225`) | Security | Verification failure silently passes today |
| 2 | Context timeouts for `GetUsage`/`Run()` | Code+UX | One hung provider/npm blocks the whole CLI |
| 3 | Run→RunE + stderr unification (~25 sites) | CLI UX | Prerequisite for script reliability |
| 4 | Converge menubar on the Swift implementation | UI | Root of double-maintenance cost & mismatches |
| 5 | Swift settings: sync process → background | UI | Reproducible 20s beach ball |
| 6 | usage Provider interface + netclient unification | Code | Provider onboarding: 5 sites → 1 file |
| 7 | Swift app honors refresh interval; drop per-minute config re-fetch | UI | Config applies differently per app |
| 8 | Resolve `config update --json` collision | CLI UX | Every script author hits this once |
| 9 | monitor TTY detection; add Examples to schedule/alert | CLI UX | Piped-output pollution & learning cost |
| 10 | Dead code & placebo test cleanup | Code | config package is effectively untested |

Items 1–3 each split into small PRs and can start risk-free since the test suite currently passes.
