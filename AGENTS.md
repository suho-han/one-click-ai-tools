# Repository Guidelines

## Project Structure & Module Organization

`one-click-tools` is a Go-first CLI distributed through GitHub Releases.

- `main.go`: entrypoint.
- `cmd/`: Cobra commands (`agent-update`, `usage`, `config`, `schedule`, `update`).
- `internal/`: core logic (`update/`, `usage/`, `config/`, `schedule/`, `ui/`).
- `scripts/`: release and installer helpers (`install.sh`, `release-package.sh`, `verify-release-integrity.sh`).
- `CONTEXT/`: project notes and local testing guides.
- `skills/`: optional skill docs; not runtime-critical.

Use `internal/ui/assets/` for icon/image assets and keep generated artifacts in their existing folders.

## Build, Test, and Development Commands

- `go run main.go help`: run CLI quickly without building.
- `go build -o oct main.go`: build local binary.
- `./oct usage --json`: smoke-test built binary behavior.
- `GOTOOLCHAIN=auto go test ./...`: run all tests.
- `GOTOOLCHAIN=auto go test -cover ./...`: run tests with coverage summary.
- `bash scripts/install.sh`: install the latest GitHub Release binary locally.
- `bash scripts/verify-release-integrity.sh`: validate release version/build integrity.
- `bash scripts/release-package.sh vX.Y.Z`: tag and publish a GitHub Release through CI.

Use caution with `go run main.go agent-update`; it can execute real `brew`/`npm` updates on your machine.

## Coding Style & Naming Conventions

Follow standard Go formatting and idioms:

- Run `gofmt` on changed Go files before opening a PR.
- Keep package names short and lowercase (`internal/update`, `internal/usage`).
- Test files use `_test.go`; test functions use `TestXxx`.

Prefer descriptive flag/command names aligned with existing CLI verbs.

## Testing Guidelines

Primary framework is Go’s built-in `testing` package.

- Add unit tests near changed code (for example `internal/usage/usage_test.go`).
- Add command-level tests in `cmd/*_test.go` when CLI behavior changes.
- For usage/API flows, prefer mock endpoints via env vars (see `CONTEXT/LOCAL_TEST.md`).

Run `go test ./...` before committing.

## Commit & Pull Request Guidelines

History follows Conventional Commits (examples: `feat(ui): ...`, `fix: ...`, `docs: ...`, `chore(release): ...`).

- Format: `type(scope): short imperative summary`.
- Keep commits focused by concern (UI, usage, update logic, docs).
- PRs should include: purpose, key changes, test evidence (`go test ./...` output), and screenshots/log snippets for terminal UI changes.
- Link related issues and note any behavior that triggers system package updates.
