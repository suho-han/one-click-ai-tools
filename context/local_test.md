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

## API mock / endpoint testing

Each provider accepts a full-URL endpoint override (`oct usage` fans out to
every enabled provider, so pick the one under test):

```bash
# Claude has no override (endpoint is hardcoded) -- test it via unit tests instead.
OCT_CODEX_USAGE_ENDPOINT="http://localhost:8080/usage" go run main.go usage
OCT_KIMI_USAGE_ENDPOINT="http://localhost:8080/coding/v1/usages" go run main.go usage
OCT_ZAI_USAGE_ENDPOINT="http://localhost:8080/api/monitor/usage/quota/limit" ZAI_API_KEY=tok OCT_ENABLED_TOOLS=zai go run main.go usage
OCT_DEEPSEEK_BALANCE_ENDPOINT="http://localhost:8080/user/balance" DEEPSEEK_API_KEY=sk go run main.go usage
OCT_OPENROUTER_USAGE_ENDPOINT="http://localhost:8080/api/v1/key" OPENROUTER_API_KEY=sk-or go run main.go usage
OCT_MINIMAX_USAGE_ENDPOINT="http://localhost:8080/v1/coding_plan/remains" MINIMAX_CODING_API_KEY=sk-cp go run main.go usage
OCT_GROK_USAGE_ENDPOINT="http://localhost:8080/v1/billing?format=credits" GROK_OAUTH_TOKEN=tok go run main.go usage
```

Standalone providers (`zai`, `deepseek`, `openrouter`, `grok`) must also be
listed in `agent_order`/`enabled_tools` (or pass the tools as shown for zai).

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
