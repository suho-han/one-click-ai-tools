# Local Test Guide

## Quick run

```bash
go run main.go help
go run main.go usage --json
```

Caution:
- `go run main.go agent-update` performs real package updates.

## Build and run

```bash
go build -o oct main.go
./oct help
./oct usage
```

## Tests

```bash
go test ./...
go test -cover ./...
```

## npm wrapper validation

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

Checklist:
- commands execute and render correctly
- runs correctly from paths containing spaces
- `schedule enable/disable` and Task Scheduler registration are valid
