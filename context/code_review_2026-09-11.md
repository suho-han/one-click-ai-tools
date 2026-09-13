# Full Code Review — 2026-09-11

Second full audit pass (after the 2026-09-10 top-10 audit, which is fully
implemented and pushed as of `32b259f`). Scope: `main.go`, `cmd/` (25 files),
`internal/` (10 packages), `scripts/*.sh`, and the Swift menubar helper.
Focus: unhandled errors and inefficiencies missed by the first pass.

All fixes below landed as 12 local commits on `main` (2026-09-11). Status
update 2026-09-12: pushed — `origin/dev` was created at the tip (`a75108f`)
and `dev` is the ongoing working branch. Verification: `GOTOOLCHAIN=auto go
test ./...` green, `gofmt` clean, Swift build + 26/26 tests green under the
Xcode beta toolchain.

## Related docs

- [code_review_2026-09-10.md](code_review_2026-09-10.md): first audit pass
  (top-10 backlog; all items now implemented)
- [improvement_plan_2026-09-10.md](improvement_plan_2026-09-10.md): execution
  plan for the first pass, including the still-open 4-B / 4-C release items
- [readme.md](readme.md): context index

## Fixed

### High severity

1. **sessionrefresh probes could hang forever** — `claude auth status`,
   `opencode providers list`, `gh auth status`, and `codex login status` ran
   with no deadline; a hung CLI stalled `oct session-refresh` and any
   scheduled task running it. Probes now derive a 5s per-command timeout from
   the refresh context (`internal/sessionrefresh/sessionrefresh.go`).
2. **Copilot plan detection doubled the HTTP cost for nothing** —
   `detectCopilotBillingPlanSource` made `/user` + billing calls on every
   refresh and every return path reported "no plan found" (the billing API
   never exposes a plan). Removed; plan comes solely from the quota API
   payload (`internal/usage/plan.go`, `copilot.go`). A quota→billing fallback
   run now issues 2 requests instead of 5.
3. **Manager detection fanned out dozens of duplicate subprocesses** —
   `ExplainPlans` and `anyBrewManaged` → `ResolveManagerForInstall` re-ran the
   same brew/pnpm/yarn/npm/go/python3 probes per tool per call. Detection
   probes now share a per-phase memo carried in the context; the install loop
   keeps a memo-free context so post-install version checks still re-probe
   (`internal/update/probe_memo.go`).

### Medium severity

4. **Alert-state read-modify-write race** — monitor, the menubar's
   `usage --notify`, and the scheduled task could clobber each other's
   `LastSent` bookkeeping. The update is now serialized by an advisory flock
   (best-effort no-op on Windows) and both the alert state and the usage
   snapshot are written via temp file + rename
   (`internal/notify/`, `internal/usage/state_snapshot.go`).
5. **Config prompts lost input and hid save failures** — a fresh
   `bufio.Reader` per prompt could swallow subsequent answers; prompts now
   share one reader, non-EOF read errors fail the command, a failed token
   save returns an error instead of printing to stdout with exit 0, and
   legacy-config migration checks `scanner.Err()` before renaming the
   original to `.bak` (`cmd/config.go`, `internal/config/config.go`).
6. **Swallowed failures across cmd/** — `schedule status` printed an empty
   status on scheduler errors; `release-doctor` reported git failures as a
   clean tree (new `git_error` field); menubar menu clicks discarded
   `runInTerminal` errors and blocked the event loop (now async, surfaced in
   the tooltip, 30s osascript timeout); `usage --notify` alert failures now
   warn on stderr; `config list --json` encode errors now return non-zero
   (`configListCmd` converted to `RunE`).
7. **Self-update trust path** — `downloadReleaseFile` /
   `writeExtractedBinary` dropped close-time flush errors (truncated archive
   risk); the brew upgrade subprocess runs under the command context;
   `installedViaBrew` stats the Cellar formula directory instead of spawning
   `brew list` on every `oct update` (`cmd/update.go`).
8. **HTTP body handling in usage providers** — 8 `io.ReadAll` sites ignored
   read errors (misreported as JSON parse failures); all now surface errors
   and cap reads at 2 MiB (`internal/usage/http_body.go`).
9. **Schedule setup errors** — linux `Enable` ignored home-dir and log-dir
   errors (cron entry logging to a relative path under the cron daemon's
   cwd); macOS `Enable` writes the plist atomically so launchctl can never
   load a half-written file (`internal/schedule/`).
10. **Release scripts** — `release-package.sh` committed and tagged before
    running tests; tests + integrity check now gate the commit/tag step.
    `install.sh`: a failing `sha256sum` piped into `awk` produced an empty
    digest reported as "checksum mismatch"; the hash command's exit status is
    checked directly, and the wget path gained timeout/tries parity with
    curl (`scripts/`).

### Low severity (also fixed)

11. Context threading: usage TUI fetch, menubar refresh, osascript and
    notification subprocesses (timeouts), `swift build` in
    `menubar build-helper` (plus cobra stream routing), release-doctor git
    probes.
12. `truncateText` / `truncateBody` cut by bytes and could split multi-byte
    runes; both cut on rune boundaries now.
13. `cli_fallback` includes the command's output in errors (matches plan.go).
14. `fetchOpenCodeGoUsage` uses `netclient` instead of a per-call
    `http.Client`.
15. Codex session-log collection drops a redundant re-sort; embedded icons
    are decoded once and cached; `netclient.DoWithRetry` fails loudly on a
    non-seekable retry body; `monitor` uses a guarded type assertion for the
    final TUI model; `alert --quiet-now` no longer mutates global viper state.
16. Swift helper: per-action `.command` launcher files are removed after a
    60s grace period; ready-file write failures are logged to stderr instead
    of `try?`.

## Deferred (documented, not fixed)

- **crontab read-modify-write locking** (`internal/schedule/linux.go`): two
  concurrent `oct schedule enable` runs for different tasks could drop an
  entry. Needs a lockfile design decision; same flock pattern as item 4 is
  the obvious candidate.
- **`launchctl` Status mapping** (`internal/schedule/macos.go`): any launchctl
  failure maps to "disabled", which can misreport a real failure as disabled.
  Needs distinguishing "not loaded" from load errors.
- **AppleScript string quoting** in `sendOSNotification`: Go `%q` is not
  exactly AppleScript quoting; messages are internally generated today, so
  exposure is theoretical.
- **Copilot local session walk** (`FetchCopilotLocalUsage`): unbounded walk
  is inherent to the cumulative metric; per-file errors are intentionally
  best-effort.
- **`monitor` terminal width** falls back to `$COLUMNS`/100 on real TTYs;
  fixing needs `golang.org/x/term`, a new direct dependency — not worth it
  for compact-mode cosmetics.
- **UsageViewModel `Task.detached` captures `self` strongly** (Swift):
  benign given the `isRefreshing` guard; informational.

## Verification

- `GOTOOLCHAIN=auto go test ./...` — all packages pass (includes new tests:
  probe timeout, probe memo, concurrent alert writers, prompt reader sharing,
  brew Cellar detection, rune truncation, doctor git-error reporting).
- `gofmt` clean on all changed files.
- Swift: `DEVELOPER_DIR=~/Downloads/Xcode-beta.app/Contents/Developer swift
  build/test --package-path macos/OctMenubar` — build clean, 26/26 tests
  pass. Note: on Xcode 27, plain `swift test` reports "0 tests" (the
  swift-testing runner no longer bridges XCTest automatically); use
  `swift test --filter "OctMenubarTests."`.
- Scripts: `bash -n` / `sh -n` plus `verify-release-integrity.sh` (no release
  was cut).

## Out of scope (unchanged from the first pass)

Remaining work from [improvement_plan_2026-09-10.md](improvement_plan_2026-09-10.md):
item 4-B (helper distribution in the release pipeline), item 4-C (legacy Go
menubar removal, criteria-gated), the `config update --json` input removal
(release-count-gated), and the manual UI gates.
