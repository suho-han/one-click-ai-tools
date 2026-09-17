# Contributing to one-click-ai-tools (oct)

Thanks for your interest in contributing! `AGENTS.md` in the repo root carries the full working agreement for repository agents; this document covers the essentials for human contributors.

## Project overview

`oct` is a single-binary Go CLI distributed through GitHub Releases. See [README.md](README.md) (Korean) or [README.en.md](README.en.md) (English) for what it does.

- `main.go` — entrypoint
- `cmd/` — Cobra commands
- `internal/` — core logic (`update/`, `usage/`, `config/`, `schedule/`, `ui/`)
- `context/` — internal developer notes and testing guides
- `scripts/` — install / release / verification helpers
- `macos/OctMenubar/` — the Swift macOS menubar companion

## Getting started

Prerequisites: Go (the toolchain is auto-selected via `GOTOOLCHAIN=auto`), bash, python3 (used by release scripts).

```bash
go run main.go help                 # run the CLI without building
go build -o oct main.go             # build a local binary
./oct usage --json                  # smoke-test the built binary
GOTOOLCHAIN=auto go test ./...      # run all tests
```

> **Caution:** `go run main.go agent-update` executes real `brew`/`npm` updates on your machine.

The menubar app needs a full Xcode toolchain; the default Command Line Tools toolchain lacks the SwiftUI macro plugin. `oct menubar build-helper` auto-discovers a full Xcode installation for you.

## Branching & commits

- Development lands on `dev`; `main` is the release branch. Open PRs against `dev` unless you are coordinating a release batch.
- Follow Conventional Commits: `type(scope): short imperative summary` (e.g. `feat(usage): ...`, `fix(menubar): ...`, `docs: ...`).
- Run `gofmt` on changed Go files and keep commits focused by concern.
- Optional advisory guard: `git config core.hooksPath .githooks` enables a `commit-msg` hook that warns when a subject doesn't match the format.

## Testing

- Add unit tests next to the changed code (`internal/.../*_test.go`); add command-level tests in `cmd/*_test.go` when CLI behavior changes.
- `GOTOOLCHAIN=auto go test ./...` must pass before committing code changes.
- For usage/API flows, prefer mock endpoints via environment variables — see `context/local_test.md`.
- CI runs a smoke matrix (macOS / Linux / Windows) on pull requests.

## Pull requests

Include: purpose, key changes, test evidence (`go test ./...` output), and screenshots or log snippets for terminal-UI changes. Link related issues and note any behavior that can trigger system package updates.

## Releases (maintainers)

Releases are cut from `main` with:

```bash
bash scripts/release-package.sh vX.Y.Z   # explicit version
bash scripts/release-package.sh auto     # computed from Conventional Commits
```

Tags with a prerelease suffix (e.g. `-beta.N`) publish as GitHub prereleases. Please don't run release scripts unless you're coordinating with a maintainer.
