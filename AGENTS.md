# Repository Guidelines

## Project Structure & Module Organization

`one-click-tools` is a Go-first CLI distributed through GitHub Releases.

- `main.go`: entrypoint.
- `cmd/`: Cobra commands (`agent-update`, `usage`, `config`, `schedule`, `update`).
- `internal/`: core logic (`update/`, `usage/`, `config/`, `schedule/`, `ui/`).
- `scripts/`: release and installer helpers (`install.sh`, `release-package.sh`, `verify-release-integrity.sh`).
- `context/`: project notes and local testing guides.
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
- `bash scripts/release-package.sh auto`: same, but computes the version from Conventional Commits since the last release tag (see "Automatic version bump" below).

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
- For usage/API flows, prefer mock endpoints via env vars (see `context/LOCAL_TEST.md`).

Run `go test ./...` before committing.

## Commit & Pull Request Guidelines

History follows Conventional Commits (examples: `feat(ui): ...`, `fix: ...`, `docs: ...`, `chore(release): ...`).

- Format: `type(scope): short imperative summary`.
- Keep commits focused by concern (UI, usage, update logic, docs).
- PRs should include: purpose, key changes, test evidence (`go test ./...` output), and screenshots/log snippets for terminal UI changes.
- Link related issues and note any behavior that triggers system package updates.
- Optional advisory check: `git config core.hooksPath .githooks` enables a `commit-msg` hook that warns (never blocks) when a subject doesn't match the Conventional Commits format above.

## Automatic version bump

`scripts/next-version.sh` computes the next release tag from commit subjects since the last release tag (baseline = most recently *created* `vX.Y.Z` tag, not the semver-highest one -- this repo's version reset at v0.1.x makes those different). Rule:

- `major == 0` (current state): `feat` bumps PATCH, a `!`/`BREAKING CHANGE` commit bumps MINOR. Nothing pre-1.0 is a stable public API yet, so breaking changes don't get a MAJOR bump.
- `major >= 1`: standard SemVer -- `feat` bumps MINOR, breaking bumps MAJOR, `fix`/`perf` bumps PATCH.
- If every commit since the baseline is something else (`chore`/`docs`/`style`/`refactor`/`test`/`ci`/`build`/...), the script refuses to suggest a version -- there's nothing release-worthy. This is what makes it safe to run unattended (see below): a docs typo fix doesn't cut a public release.

Run `bash scripts/next-version.sh` to preview the computed version and the feat/fix/breaking breakdown (printed to stderr; stdout is just the version string), or `bash scripts/release-package.sh auto` to use it directly for a release. The script refuses to suggest a tag that already exists locally or as a published GitHub Release.

### CI: automatic release on every push to main

**Currently disabled** (`gh workflow disable auto-release.yml`) after it fired on its own first commit and had to be rolled back (deleted release/tag, force-pushed the version-bump commit back off `main`). Re-enable with `gh workflow enable auto-release.yml` once you're ready to rely on it again; until then, cut releases manually with `bash scripts/release-package.sh auto` (or `vX.Y.Z`).

`.github/workflows/auto-release.yml` runs the above on every push to `main`: it computes the next version, and if there's anything release-worthy it runs `scripts/release-package.sh` end to end (commit, tag, `go test ./...`, push, publish) with no human step. If nothing release-worthy landed, the job exits cleanly without releasing.

This means **every push to main that contains a `feat`/`fix`/`perf`/breaking commit becomes a public GitHub Release automatically** -- this repo has no PR gate, so there is no review point before that happens. Before pushing a commit you don't want released immediately, consider whether it belongs in a topic branch instead.

Mechanism note: a tag pushed by the workflow's `GITHUB_TOKEN` does not trigger the `goreleaser` workflow's normal `push: tags` event (GitHub suppresses further workflow runs triggered by `GITHUB_TOKEN`, to prevent infinite loops). `release-package.sh` detects `CI=true` and instead dispatches `goreleaser` explicitly via `gh workflow run ... -f release_mode=release -f git_ref=vX.Y.Z`, which *is* exempt from that restriction.

### Batch related changes into one push, don't push after every commit

Because auto-release fires on every push, pushing after each individual commit turns each one into its own release (`v0.1.4`, `v0.1.5`, `v0.1.6`, ... for what should have been one coherent change). **Commit as many times as you like locally, but hold the push until the batch of related `feat`/`fix`/`perf` work is actually ready to ship, then push once.** One push should read as one meaningful release, not one commit's worth of diff.

(Reference: [gajae-code](https://github.com/Yeachan-Heo/gajae-code) achieves the same outcome differently -- it decouples "land on main" from "cut a release" with a cron-scheduled nightly build plus a separate human-triggered stable-tag release. We didn't adopt that scheduling infrastructure here since it's sized for a multi-package monorepo; the equivalent for this repo's single-binary CLI is simply this push-batching discipline.)
