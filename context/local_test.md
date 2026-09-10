# Local Test Guide

## Quick run

Currently recommended local Go toolchain: `go1.26.4` (`~/.local/go`)

```bash
go version
go run main.go help
go run main.go usage --json
```

Caution:
- `go run main.go agent-update` executes real package updates.

## Build & run

```bash
go build -o oct main.go
./oct help
./oct usage
```

## Tests

```bash
GOTOOLCHAIN=auto go test ./...
GOTOOLCHAIN=auto go test -cover ./...
```

## npm wrapper check

```bash
go build -o oct main.go
npm link
oct help
oct usage
npm unlink -g one-click-tools
```

## API mock / endpoint testing

```bash
OCT_CLAUDE_USAGE_ENDPOINT="http://localhost:8080/usage" go run main.go usage
```

## Windows validation essentials

```powershell
go build -o oct.exe main.go
.\oct.exe help
.\oct.exe usage --json
```

Checkpoints:
- Commands run and output correctly
- Works when executed from paths containing spaces
- `schedule enable/disable` and Task Scheduler registration verified

## Related docs

- [platform_e2e_checklist.md](platform_e2e_checklist.md): full per-platform schedule/install E2E verification checklist
- [macbook_air_smoke_test.md](macbook_air_smoke_test.md): quick smoke test on the primary macOS host
