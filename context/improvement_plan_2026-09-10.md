# Improvement Plan — Code Review Follow-up (2026-09-10)

Execution plan (fix design + test plan per item) for the Top 10 priorities in
[code_review_2026-09-10.md](code_review_2026-09-10.md). Review baseline: v0.1.5
(`d4842af`); `file:line` references below are anchored to that commit and will
drift. Status: **executed 2026-09-10, pushed 2026-09-11** (landed on `main`;
`origin/dev` now carries the same tip, `a75108f`). Batch 1 (items 1, 3, 10-a, 2, 10-b),
batch 2 (items 6, 9), batch 3 (items 7, 8, 5), and item 4-A are done. Full Go
suite green (`go test ./...`, 10 packages). Swift changes are compile- and
test-verified: the default CommandLineTools toolchain lacks the SwiftUI
macro plugin (fails identically on untouched HEAD), so builds run via
`DEVELOPER_DIR=~/Downloads/Xcode-beta.app/Contents/Developer swift test
--package-path macos/OctMenubar` — 26/26 pass, plus three swift-6
concurrency defects found and fixed during that first real compile
(ebb7f0d). Note: `oct menubar build-helper` on this host needs the same
DEVELOPER_DIR export. Item 4-B implemented 2026-09-22 (darwin-assets builds a stamped
universal helper into both darwin tars, install.sh / `oct update` /
verify-release-integrity.sh handle it, `oct menubar doctor` reports
oct/helper version skew); first real-release verification still pending.
Remaining: item 4-C (legacy removal after its completion criteria),
Swift-side manual UI gates (responsiveness, helper launch with the new
helper), `config update --json` input-removal follow-up (two releases
after 061fde4). Revised same-day
after external plan review; 13 findings are folded in below.

## 0. Corrections to the review document (found during planning investigation)

1. `internal/update/command_env.go` is **not** fully dead. `commandWithEnv`,
   `commandContextWithEnv`, and `lookPathWithBootstrap` are live (manager.go,
   update.go call sites); only `commandEnv` (command_env.go:10) and
   `withPathEnv` (:14) are dead. Deleting the whole file would break the build.
2. The Run→RunE conversion target is **exactly 20 `Run` commands** (not "~25
   sites"), plus root `Execute` and the `config list` `--json` re-wrap.
3. Percent-math duplication is **five** sites, not four — `cmd/monitor.go:356`
   (`usageRemaining`) duplicates `remainingFromUsed`. Folded into item 6.
4. `oct menubar --legacy` **already exists** (menubar.go:240). Item 4's
   remaining work is "Swift-first default + fallback warning + helper
   distribution + legacy freeze", not flag introduction.
5. Release darwin tarballs ship no Swift helper, so release-binary users
   currently run the **legacy-fallback** path today — item 4 is phased
   convergence, not deletion.

## Execution strategy

- **Plan approval ≠ publish approval.** Implementation runs batch-by-batch, but
  every commit stays **local**. Pushing to `main` (and any release it might
  trigger) happens only after an explicit publish approval for that batch.
  The auto-release workflow state check
  (`gh api repos/suho-han/one-click-ai-tools/actions/workflows/auto-release.yml --jq .state`)
  runs immediately before an actual push, never as a standing assumption;
  if unverifiable, work stays on a topic branch.
- The working tree currently holds another session's changes (`context/` doc
  restructuring, `cmd/menubar_helper_test.go`) — leave untouched and exclude
  from staging.
- Per item: `GOTOOLCHAIN=auto go test ./...`; Swift items additionally
  `swift test --package-path macos/OctMenubar`.
- **Batch completion gates** (all required, not just the suite): full test
  suite green **plus** the batch's item-specific manual verifications — e.g.
  Swift UI responsiveness for item 5, real helper launch/discovery on the
  macOS host for item 4 (see macbook_air_smoke_test.md). A green suite alone
  does not complete a UI batch.
- Behavior changes and pure deletions are separate commits, each with its
  regression test written/confirmed before the change lands (see item 10 and
  the execution order).

---

## 1. Self-update checksum fail-closed (security)

**Fix** (`cmd/update.go`)

- Change the call at `:225` to require verification; since it becomes the only
  caller, **remove the `require` parameter** entirely —
  `verifyReleaseAssetChecksum(ctx, repo, asset, archivePath)` (pre-absorbs the
  item-10 "unreachable branch" cleanup).
- Make the two silent-skip sites state their reason:
  `"checksums.txt unavailable for %s: %w (refusing to install without
  verification)"` and `"checksum entry not found for %s (refusing to install)"`.
- Secondary: the client-wide 30s `http.Client.Timeout` (`:45`) can abort a
  large archive mid-`io.Copy` — switch to `Timeout: 0` + Transport-level
  timeouts, with per-call `context.WithTimeout` (120s archive / 30s checksums)
  inside `downloadReleaseFile`.

**Tests** (`cmd/update_test.go`)

- Introduce a `var checksumBaseURL` seam (existing `commandCodeAPIBaseURL`
  pattern) so `httptest` can serve checksums.txt despite the hardcoded
  `https://github.com` URL.
- Table test `TestVerifyReleaseAssetChecksum`: (a) match → nil, (b) mismatch →
  error (existing behavior), (c) **checksums.txt download failure → error (new,
  the core assertion)**, (d) **missing entry → error (new)**, (e) hash-read
  failure → error. Existing 4 tests must stay green.

**Commits**: `fix(update): fail closed on self-update checksum verification`

## 2. Context deadlines for GetUsage / update.Run() (code + UX)

**Fix — internal/usage**

- Fetcher type becomes `func(context.Context) UsageResult` (`usage.go:68` map +
  the 10 `Fetch*` functions; mechanical propagation). All HTTP call sites move
  to `http.NewRequestWithContext`; the inline clients at commandcode.go:239 /
  opencode.go:165 also get ctx on the request (full netclient unification is
  item 6).
- **Keychain lookup is in scope**: claude.go:54's
  `execCommand("security", ...).Output()` takes no ctx today, so a hung
  keychain helper would keep `errgroup.Wait()` open forever — and
  `errgroup.WithContext` does not kill running goroutines. Route it through the
  ctx-aware command runner (same helper as the CLI fallbacks).
- CLI fallbacks `cli_fallback.go:12` / `plan.go:111`: accept a parent ctx and
  derive `context.WithTimeout(parent, timeout)`.
- `GetUsage(ctx)`: `errgroup.WithContext` + overall ceiling via a
  `var usageFetchTimeout = 15 * time.Second` seam. Rationale: below the Swift
  app's 20s process timeout (OctCLIService.swift:12), so the menubar receives
  partial-error JSON instead of a killed process.
- **Return rule on timeout** (explicit): when the ceiling fires, every
  still-running provider gets an error `UsageResult` (`status: "error"`,
  message naming the provider + timeout); already-finished results are
  preserved verbatim. `GetUsage` therefore never returns a bare fatal error —
  per-provider degradation only, and `oct usage --json` still emits the full
  payload.
- Callers `cmd/usage.go`, `cmd/monitor.go`, `cmd/session_refresh.go` pass
  `cmd.Context()`.

**Fix — internal/netclient**

- `DoWithRetry`: check `req.Context().Err()` before each attempt and each
  backoff; replace `time.Sleep` with a ctx-aware `sleep(ctx, d)`
  (Timer + select) and a test seam `var retrySleep`. Side effect: the existing
  6.6s hard-sleep tests become instant (also resolves review 2-6).

**Fix — internal/update**

- `Run(ctx)`: `cmd/agent_update.go` passes `cmd.Context()` with a ceiling seam
  (10 minutes). The ceiling covers **all** work paths, not just probes:
  `ExplainPlans` → `explainResolvedManager` → `matchesInstalledPackage` →
  `packageListCommand` → `commandOutput` all gain ctx, and `anyBrewManaged`'s
  manager re-resolution is ctx-aware too.
- **Ceiling scope vs user input**: the clock starts after the interactive
  confirmation prompt (prompt waiting is user interaction, excluded from the
  ceiling); everything after it — probes, `brew update`, installs, npm
  recovery — is bounded and checks ctx before starting each subsequent install
  or recovery step, so cancellation prevents new work from starting.
- `manager.go:143 GetInstalledVersion` gains a ctx variant (internally
  `commandWithEnv` → `commandContextWithEnv`, 9 probe sites); `update.go:88`
  `brew update` becomes ctx-aware.
- While touching this code, reuse probe results so `Run` stops probing each
  tool's version twice via `ExplainPlans` (`update.go:66` vs `:113`; absorbs
  part of review 2-4).

**Tests**

- `internal/netclient`: counting RoundTripper stub — canceled ctx stops
  retries; `retrySleep` no-op swap speeds up the two existing tests.
- `internal/usage`: blocking fetcher on `ctx.Done()` → `GetUsage` returns
  promptly with `usageFetchTimeout=100ms`; **hung keychain helper** (swap
  `execCommand` seam with a blocking process) → deadline still enforced;
  **mixed fast + slow providers** → fast provider's result preserved in the
  output while the slow one carries the timeout error result (return-rule
  assertion).
- `internal/update`: helper-process re-exec pattern (release_doctor_test.go:52-87)
  — hung `--version` probe and **hung package-list command** each + short ctx →
  immediate error; cancel before later installs → subsequent install/recovery
  step never starts.
- Full regression + Swift `usage --json` contract unchanged (PrintJSON schema
  test).

**Commits**: `refactor(netclient): make retries context-aware` /
`fix(usage): enforce overall fetch deadline` /
`fix(update): context-aware version probes and install ceiling`

## 3. Run → RunE + stderr unification (CLI UX, 20 commands)

**Fix**

- Convert all 20 `Run` commands (schedule ×4, alert ×7, config ×6, monitor,
  agent-update, update) to `RunE`; each `fmt.Println(err)` becomes
  `return fmt.Errorf("user-friendly message: %w", err)`.
- Exception: monitor's in-loop transient errors (usage failure `:38`, snapshot
  failure `:49`) print to stderr and keep looping (daemon behavior); only fatal
  startup failures return.
- Root error output must stay **testable through the same channel it prints
  to**: errors print via `rootCmd.ErrOrStderr()` (honors `SetErr`), and
  `Execute()`'s `fmt.Println(err)` (`:26`) is replaced by a small testable
  entry (`runCLI(args, stdout, stderr) int` used by main.go) that returns the
  exit code instead of calling `os.Exit` inline. Add `SilenceErrors: true`;
  the `os.UserHomeDir` failure at `:81` also moves to stderr. Apply
  `SilenceUsage: true` to all subcommands at the existing `reorderRootCommands()`
  init point (same precedent as menubarCmd) so runtime errors don't spam usage.
- Drop the direct `os.Exit(1)` handling in `updateCmd`/`agentUpdateCmd` — plain
  RunE returns; root handles stderr + exit code.
- Preserve the `config list --json` re-wrap interaction
  (config_api_command.go:35-47).

**Tests**

- Representative RunE error-return tests: `schedule enable` (state error),
  `alert config set-threshold` (validation), `config set-tools` (unknown tool),
  `alert snooze show` (load failure) — extend the existing direct-RunE-call
  pattern from usage_test.go.
- Root routing via `runCLI` + `rootCmd.SetOut/SetErr` buffers + forced failure:
  error string present **only** in the stderr buffer, stdout empty, exit code
  non-zero — asserted on the same writer abstraction the production path uses
  (implementation and tests share the channel; no `os.Stderr` bypass).
- Keep the real-subprocess e2e test (root_test.go:128 `go run` pattern),
  extended to assert non-zero exit and channel separation as an end-to-end
  backstop.
- Existing exact-match stdout tests (`"X-45% C-88%"`, `"menubar daemon
  started"`) stay green — success output unchanged.
- Manual smoke (system-mutating commands excluded): `oct config set-tools nope
  </dev/null; echo $?` → 1.

**Commits**: `fix(cli): convert Run commands to RunE and route errors to stderr`
(testable root entry split out as its own commit if needed)

## 4. Menubar convergence on Swift (UI, phased)

Facts: `--legacy` exists; the Swift app is an SPM single binary (no Xcode);
release tarballs contain no helper → release users are on legacy-fallback today.

**Fix (three phases)**

- **4-A (this batch)**: fix the silent fallback — when Swift launch fails at
  menubar_darwin.go:26-30, print once to stderr: `"Swift helper unavailable;
  falling back to legacy menubar (reason)"`. Strengthen `oct menubar doctor`'s
  `legacy-fallback` guidance. Document Swift as the canonical path.
- **4-B**: helper distribution. **The darwin packaging owner is the
  `darwin-assets` job in `.github/workflows/release.yml`, not
  `.goreleaser.yaml`** — that job builds the per-arch Go binaries and tars.
  Extend it to build the helper (per-arch `swift build -c release` for amd64
  and arm64, or a universal binary) and include `OctMenubarApp` in each tar;
  `scripts/install.sh` installs it to `~/.local/bin`. Extend
  `verify-release-integrity.sh` to check helper presence, architecture match,
  exec permission, and checksum. **Self-update coverage**: `cmd/update.go`
  currently extracts and replaces only `oct` — add a best-effort helper
  install/update step from the same tar (skip with a warning when the helper
  asset is absent, e.g. older releases). Policy: oct replacement is critical
  (failure aborts); helper installation is best-effort (failure warns, legacy
  fallback remains); both binaries come from the same tar so they match by
  construction, and `oct menubar doctor` reports both versions to surface
  skew from partial manual installs.
- **4-C completion criteria (explicit, not just "one cycle later")**: delete
  the Go legacy menubar only after — (1) existing installs and the
  `oct update` path demonstrably transition to the helper, (2) the helper is
  verified actually launching on the supported macOS versions/architectures,
  (3) `oct menubar doctor` reports `swift-helper` for release installs, and
  (4) `--legacy` removal guidance is written. `menubar_state.go` rendering
  stays — it is shared with monitor.

**Tests**: 4-A — capture test that the fallback warns (menubar_ready_test
pattern). 4-B — CI: helper built per-arch, tar contains it with exec
permission; `verify-release-integrity.sh` helper checks; smoke: install →
helper discovery → real launch on the macOS host. 4-C — `go test ./...` +
`oct menubar doctor` smoke after deletion.

**Commits**: `fix(menubar): warn when falling back to legacy` →
`feat(release): ship Swift menubar helper` →
`feat(update): install menubar helper on self-update`

## 5. Swift settings: synchronous process → background (UI)

**Fix** (`macos/OctMenubar`)

- **Execution isolation, stated precisely**: all `Process` I/O in
  `runProcess` executes off the main thread (async wrapper dispatches the
  runner to a background queue; results hop back to `@MainActor` for state
  updates). UI state changes happen only on the main actor; the runner never
  touches view state directly. `SettingsView.swift:22,73-85`
  `loadConfiguration`/`saveConfiguration` call the async wrapper from
  `Task { ... }`; add loading spinner/disabled state.
  Verification: with a deliberately slow command (e.g. a 5s sleep stub), the
  settings window stays interactive — manual check plus a test asserting the
  load future resolves without blocking the main run loop.
- **Stale-result guard**: a new load/save cancels any in-flight load
  (structured task cancellation, plus a generation counter so a late-arriving
  load result can never overwrite a newer draft).
- Fix the **pipe-capacity deadlock** in the same `runProcess` (:178-225):
  replace the `while process.isRunning` poll (:203-210) with
  `terminationHandler`, and start draining pipes via `readabilityHandler`
  **before** waiting — output exceeding the pipe buffer capacity currently
  blocks the child on write and gets misdiagnosed as a timeout.
- **Completion condition, stated precisely**: `runProcess` resolves only after
  (a) process termination **and** (b) EOF on both stdout and stderr pipes;
  buffer access is synchronized on a serial queue; timeout/cancellation
  terminates the process and still drains/cleans up; the continuation resumes
  exactly once (guarded).
- Preserve unsaved drafts when the settings window closes and reopens (review
  3-3): keep the draft instead of reloading in `onAppear`.

**Tests** (`Tests/OctMenubarTests`)

- Runner: both pipes emitting beyond pipe-buffer capacity **simultaneously**
  → completes, full data received from both; output emitted right before
  exit → not truncated; timeout → process terminated, continuation fires
  exactly once; launch failure (missing executable) → error propagates.
- Settings: slow command while window "open" → UI state still updates (no
  main-thread block); late load result does not overwrite a newer draft;
  exit-code/stderr propagation (`nonZeroExit`) contract unchanged.

**Commits**: `fix(macos): async settings I/O and non-blocking process pipes`

## 6. usage provider registry + netclient unification (structure)

**Fix** (`internal/usage`)

- New registry: `type Provider struct { Name, Label, CompactLabel, ColorCode
  string; Aliases []string; Fetch func(ctx) UsageResult }` +
  `var providers = []Provider{...}`. The five scattered sites (fetchers map
  usage.go:68, default order :58, color switch :126-149, display labels
  :175-192, compact.go:35-57) all derive from it. A new provider = one file +
  one registry line.
- Keep the substring fallback for unknown providers (behavior compat — the
  existing colorize/label/compact tests pin it).
- Unify the five percent implementations: export
  `InvertUsedPercent(string) (string, bool)` and `ClampPercent(float64)` from
  the usage package; replace usage.go:355, compact.go:112, gemini.go:163,
  commandcode.go:443, **and cmd/monitor.go:356**. Inputs differ in meaning
  (used vs remaining) — document semantics per call site.
- netclient gains `Client.GetJSON(ctx, url, headers, &v)`; absorb the inline
  clients at commandcode.go:239 / opencode.go:165 (retry policy restored).
- Apply `osDebugEnabled` (gemini.go:174) to the 13 raw `os.Getenv` checks
  (absorbs the TrimSpace semantic difference).
- **JSON output shape stays byte-identical** (Swift contract; PrintJSON schema
  test guards it).

**Tests**: registry unit tests (alias→fetcher resolution, order, unknown
fallback); percent helper table tests (boundaries: negative, >100, parse
failure); existing colorize/label/compact/PrintJSON tests unchanged and green;
commandcode/opencode tests green through netclient (reuse the
`rewriteHostTransport` pattern).

**Commits**: `refactor(usage): provider registry` /
`refactor(usage): unify percent math` /
`refactor(usage): route all HTTP through netclient`

## 7. Swift app honors refresh interval; drop per-minute config re-fetch (UI)

**Fix**

- Go: add `menubar_refresh_interval` to the `config list --json` payload
  (cmd/config_api.go:22-31), emitted as the stored string (e.g. `"1m"`).
  Backward compatible (old Swift ignores unknown fields).
- Swift: decode `refreshInterval` in `ConfigurationSnapshot` + a Go-duration
  parser (`"90s"`, `"1m30s"`), default 60s.
- **Cache ownership and save notification, stated precisely**: today
  `SettingsView` creates its own `OctCLIService` while `UsageViewModel` holds
  its own `let service`, so a save in one cannot reach the other's timer.
  Introduce a single app-level configuration store (one `OctCLIService` +
  cached snapshot, injected into both consumers). Settings save updates the
  store, the store notifies observers, and `UsageViewModel` reschedules its
  `Timer` from the new interval.
- **External-change policy** (CLI edits `config.yaml` directly): explicit
  re-read when the popover or settings window opens; no file watching (stated
  as the chosen policy).
- Remove the per-refresh `config list --json` call (OctCLIService.swift:34);
  drop the `try?` swallowing — on read failure keep the last known/default
  configuration, surface the error, and retry on the next open/save.

**Tests**: Go — `config_api_test.go` asserts the new key. Swift — legacy JSON
without the field decodes (default 60s); duration parser table including
`0`, negative, and invalid strings → fallback 60s; save through the shared
store reschedules the timer observed by `UsageViewModel`; initial read
failure → default used + retry succeeds on next open.

**Commits**: `feat(config): expose menubar_refresh_interval in config list` /
`fix(macos): honor refresh interval and cache configuration`

## 8. Resolve the `config update --json` collision (CLI UX)

**Fix**

- Go: add `--payload <json>` to `config update` (config_api_command.go:49) as
  the input flag; `--json <payload>` keeps working but prints a one-time
  deprecation warning to stderr; `--payload -` reads stdin (for scripts).
  Specifying both `--payload` and `--json` is an error ("use one of
  --payload or --json").
- Swift: `saveConfiguration` (:60) switches to
  `["config", "update", "--payload", payload]`, with a fallback retry via
  `--json` **only when the failure is positively identified as the
  old-binary flag-parse error** (stderr contains `unknown flag: --payload`).
  Every other failure — validation, file write, timeout — surfaces directly
  with no retry: retrying them would mask the real error and repeat an
  already-applied change.
- Remove the `--json` input meaning two releases later (track as a follow-up).

**Tests**: Go — `--payload` round trip (config_api_test pattern), deprecation
warning emission, stdin mode, both-flags → error. Swift — unknown-flag
failure → one retry via `--json` succeeds; **generic failure → no retry**
(error propagates as-is).

**Commits**: `feat(config): add --payload flag for config update` /
`fix(macos): use --payload for config updates`

## 9. monitor TTY detection; command Examples (CLI UX)

**Fix**

- `cmd/monitor.go:83,132`: reuse usage.go:129-138's TTY detection — non-TTY
  runs (`--once` piped) get plain output without clear-screen ANSI codes and
  the "Ctrl+C to stop" line. Extract detection into a test seam
  (`var monitorIsTTY`).
- Add `Example:` fields: `schedule` (enable/disable/config), `alert`
  (config set-threshold/snooze), `monitor` (default/`--once`) — flag-heavy
  commands first; add missing Long descriptions.

**Tests**: monitor_test.go seam-driven two-mode tests (no `\x1b[2J`-style codes
and no "Ctrl+C" in non-TTY; present in TTY mode); root_test.go asserts Examples
appear in help.

**Commits**: `fix(monitor): plain output when non-TTY` /
`docs(cli): add examples to schedule/alert/monitor`

## 10. Dead code & placebo test cleanup (verified findings) — split 10-a / 10-b

Pure deletions and behavior changes are **separate commits**; each behavior
change lands with its regression test confirmed first.

**10-a — deletions + real test (no behavior change)**

- `cmd/config.go:420 visibleLen`, `internal/ui/image.go:81 PrintIcon`,
  `internal/schedule/linux.go:100 filepathJoin`, the `Config` struct at
  `internal/config/config.go:13`, **only** the two dead functions
  `commandEnv`/`withPathEnv` in `internal/update/command_env.go` (the file
  itself is live — review correction #1), the no-op `if` at
  cmd/alert.go:112-114, and the vestigial `brewInstallMu`
  (internal/update/update.go:20; the install loop is sequential).
- Rewrite `internal/config/config_test.go:11-51` (named
  `TestMigrateLegacyConfig` but never calls it): a real-call test with
  `t.TempDir` + HOME override asserting `enabled_tools` migration, `.bak`
  rename, 0600 permissions, and — as explicitly pinned current behavior — that
  other legacy keys (`schedule_enabled`) are dropped.

**10-b — behavior changes (each its own commit, test-first)**

- `internal/schedule/macos.go:5` plist template: **not a plain
  html→text/template swap** — that would drop XML escaping and break plists
  when `BinaryPath`/`LogPath` contain `&` or `<`. Keep escaping semantics via
  explicit XML escaping (or `encoding/xml` serialization) for the injected
  path values; regression test round-trips a special-char path through an XML
  parser and matches the intent. Commit: `fix(schedule): escape XML in
  launchd plist template`.
- `internal/execenv/execenv.go:104` lookup fallback: **GOOS-split
  executability rule** — Unix requires the exec bit
  (`info.Mode().Perm()&0111 != 0`); Windows does **not** get the Unix bit
  check and instead requires an executable extension per the
  standard `exec.LookPath`/PATHEXT rules (shared code must not apply Unix
  semantics to Windows). Regression tests: Unix non-executable file → not
  found; Windows `.exe` discovery in the fallback path (GOOS-gated test).
  Commit: `fix(execenv): require executability in lookup fallback`.
- `internal/netclient/client.go:57`: drop deprecated `net.Error.Temporary()`
  → `errors.As` + `Timeout()` (semantic change: timeouts only are retried —
  note in comment); update the retry-condition test table. Commit:
  `refactor(netclient): drop deprecated Temporary()`.
- Plus `docs: correct code review dead-code findings` (review-doc correction).

**Tests**: `go build ./...` + full suite after 10-a; new MigrateLegacyConfig
coverage; per-change regression tests above, written before each change.

---

## Execution order (dependency-aware)

1. **Batch 1** — item 1 → item 3 → **item 10-a** (pure deletions + placebo
   test) → item 2 (full cancellation propagation) → **item 10-b** (plist
   XML, exec-bit, retry semantics — each test-first). Item 10-b is separated
   from 10-a because these are behavior changes, not cleanups; item 2 is
   similarly more than a small fix. Regression tests for each change are
   settled before the change lands.
2. **Batch 2** — item 6 (registry builds on item 2's ctx-aware fetcher
   signatures, avoiding double churn), item 9.
3. **Batch 3** — item 7-Go → 8-Go → 5, 7-Swift, 8-Swift (Go JSON exposure
   precedes Swift consumption).
4. **Batch 4** — items 4-A, 4-B (helper distribution); 4-C after its
   completion criteria (above) are met.

Each batch completes only when the full suite is green **and** the batch's
manual verification gates pass (Swift UI responsiveness for batch 3; real
helper launch/discovery for batch 4; smoke pass per
macbook_air_smoke_test.md). Commits stay local; a push happens only after
explicit publish approval, with the auto-release workflow state checked
immediately beforehand.

## Related docs

- [code_review_2026-09-10.md](code_review_2026-09-10.md): the review backlog
  this plan executes; section 5 holds the original Top 10 table
- [local_test.md](local_test.md): everyday build/test workflow used per item
- [macbook_air_smoke_test.md](macbook_air_smoke_test.md): post-change smoke
  pass for menubar/UI batches
- [menubar_helper_operations.md](menubar_helper_operations.md): helper
  build/install flows referenced by item 4
- [todo.md](todo.md): living future-work list (follow-ups land here)
