# Platform E2E Checklist (Linux / macOS / Windows)

## Purpose

Validate that `schedule enable/disable` and install-time `session-refresh` wiring use the correct binary, correct task name, and correct log path on real target platforms.

## Common preflight

- Confirm `oct` binary location
  - `command -v oct` (Linux/macOS)
  - `where oct` (Windows)
- Confirm baseline command execution
  - `oct --version`
  - `oct schedule --task agent-update`
  - `oct schedule --task session-refresh`
- Confirm write access for home/log paths
  - `~/.oct/`
  - `~/.oct/logs/`
- Confirm old scheduler entries are cleaned up
  - `agent-update`
  - `session-refresh`

## Install-time (postinstall) validation

### Common

1. Start from a clean HOME or test account
2. Run `npm install -g one-click-tools` or equivalent install flow
3. Confirm `~/.oct/config.yaml` is created
4. Confirm defaults:
   - `session_refresh_enabled: false`
   - `session_refresh_interval: daily`
   - `session_refresh_hour: 9`

### Non-interactive

1. Run install in a non-TTY / CI-like environment
2. Confirm `Enable periodic token-free session refresh?` is not shown
3. Confirm `session_refresh_enabled: false` remains in config
4. Confirm no scheduler entry is auto-created

### Forced enable

1. Run install with `OCT_INSTALL_ENABLE_SESSION_REFRESH=yes`
2. Confirm `session_refresh_enabled: true` is saved
3. Confirm the `session-refresh` schedule is auto-created
4. Confirm the created entry points to the **currently installed oct binary**

### Interactive

1. Run install in a TTY environment
2. Confirm prompt: `Enable periodic token-free session refresh? [y/N]:`
3. Answer `y` and confirm `session_refresh_enabled: true` is saved
4. Confirm the created scheduler entry points to `session-refresh`
5. Confirm the entry points to the **just-installed oct binary**, not some other `oct` on PATH

## Linux (cron)

### agent-update

1. `oct schedule enable --task agent-update --interval daily --hour 3`
2. Confirm a single `# oct-managed:agent-update` entry in `crontab -l`
3. Confirm the command is `oct agent-update`
4. Confirm the log path is `~/.oct/logs/agent-update.log`
5. Confirm `oct schedule --task agent-update` reports `enabled`
6. Re-run the same enable command and confirm no duplicate entry is added
7. `oct schedule disable --task agent-update`
8. Confirm the `agent-update` entry is removed from `crontab -l`

### session-refresh

1. `oct schedule enable --task session-refresh --interval daily --hour 9`
2. Confirm a single `# oct-managed:session-refresh` entry in `crontab -l`
3. Confirm the command is `oct session-refresh`
4. Confirm the log path is `~/.oct/logs/session-refresh.log`
5. Confirm the stored binary path matches the currently running `oct`
6. Confirm `oct schedule --task session-refresh` reports `enabled`
7. Re-run the same enable command and confirm no duplicate entry is added
8. `oct schedule disable --task session-refresh`
9. Confirm the `session-refresh` entry is removed from `crontab -l`

## macOS (launchctl)

### agent-update

1. `oct schedule enable --task agent-update --interval daily --hour 3`
2. Confirm `~/Library/LaunchAgents/com.oct.agent-update.plist` exists
3. Confirm `ProgramArguments` is `[<oct binary>, "agent-update"]`
4. Confirm `StandardOutPath` / `StandardErrorPath` is `~/.oct/logs/agent-update.log`
5. Confirm the job is loaded via `launchctl list | grep com.oct.agent-update` or equivalent
6. Confirm `oct schedule --task agent-update` reports `enabled`
7. `oct schedule disable --task agent-update`
8. Confirm the plist is removed/unloaded

### session-refresh

1. `oct schedule enable --task session-refresh --interval daily --hour 9`
2. Confirm `~/Library/LaunchAgents/com.oct.session-refresh.plist` exists
3. Confirm `ProgramArguments` is `[<oct binary>, "session-refresh"]`
4. Confirm the plist points to the current binary, not some other `oct` on PATH
5. Confirm `StandardOutPath` / `StandardErrorPath` is `~/.oct/logs/session-refresh.log`
6. Confirm the job is loaded via `launchctl list | grep com.oct.session-refresh` or equivalent
7. Confirm `oct schedule --task session-refresh` reports `enabled`
8. `oct schedule disable --task session-refresh`
9. Confirm the plist is removed/unloaded

## Windows (Task Scheduler)

### agent-update

1. `oct schedule enable --task agent-update --interval daily --hour 3`
2. Confirm `schtasks /Query /TN OneClickToolsUpdate` succeeds
3. Confirm the action is `<oct binary> agent-update`
4. Confirm logging/output policy matches the docs
5. Confirm `oct schedule --task agent-update` reports `enabled`
6. `oct schedule disable --task agent-update`
7. Confirm `schtasks /Query /TN OneClickToolsUpdate` fails (not found)

### session-refresh

1. `oct schedule enable --task session-refresh --interval daily --hour 9`
2. Confirm `schtasks /Query /TN OneClickToolsSessionRefresh` succeeds
3. Confirm the action is `<oct binary> session-refresh`
4. Confirm binary path quoting is safe for paths with spaces
5. Confirm the task points to the current binary, not some other `oct` on PATH
6. Confirm `oct schedule --task session-refresh` reports `enabled`
7. `oct schedule disable --task session-refresh`
8. Confirm `schtasks /Query /TN OneClickToolsSessionRefresh` fails (not found)

## Minimum regression checks

- `GOTOOLCHAIN=auto go test ./internal/schedule -v`
- `GOTOOLCHAIN=auto go test ./...`
- `go build -o oct main.go`

## Exit criteria for manual validation

- install-time prompt / forced env enable / non-interactive defaults all behave as expected
- `agent-update` and `session-refresh` coexist safely with distinct scheduler entries/task names per platform
- created entries always point to the **current oct binary**
- repeated enable commands do not create duplicate entries
- disable removes the entry completely
