# Platform E2E Checklist (Linux / Windows)

## Common

- `oct` binary is on PATH
- `oct schedule` prints status
- log/state paths are writable

## Linux (cron)

1. `oct schedule enable --interval daily --hour 3`
2. confirm a single `oct agent-update` entry in `crontab -l`
3. confirm `oct schedule` reports `enabled`
4. `oct schedule disable`
5. confirm entry is removed from `crontab -l`

## Windows (Task Scheduler)

1. `oct schedule enable --interval daily --hour 3`
2. `schtasks /Query /TN OneClickToolsUpdate` succeeds
3. task action is `oct agent-update`
4. `oct schedule` reports `enabled`
5. `oct schedule disable`
6. `schtasks /Query /TN OneClickToolsUpdate` fails (not found)

## CI smoke

- OS: `ubuntu-latest`, `windows-latest`
- Checks: `go test ./...`, `go build ./...`
