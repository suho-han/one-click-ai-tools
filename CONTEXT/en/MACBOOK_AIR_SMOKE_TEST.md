# MacBook Air Smoke Test Guide

This is the minimal smoke-test procedure for quickly validating `one-click-tools` on the primary macOS work host (`100.73.225.85`).

## Target environment

- host: `100.73.225.85`
- user: `suhohan`
- repo: `/Users/suhohan/Projects/one-click-tools`
- purpose: quick post-change sanity validation

## Goals

Use this document to quickly confirm:
- the worktree is on the expected `main`/HEAD
- the Go CLI still builds
- the baseline Go test suite still passes
- the core Swift build/test path for the menubar helper still works

For full validation or remote operations, see:
- `CONTEXT/en/LOCAL_TEST.md`
- `CONTEXT/en/MENUBAR_HELPER_OPERATIONS.md`
- `PROJECT_CONTEXT/remote-macos-validation-status.md`

## Quick start

```bash
ssh -o IdentitiesOnly=yes -i ~/.ssh/hermes_kbo_live_ed25519 suhohan@100.73.225.85
export PATH=/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin:$PATH
cd /Users/suhohan/Projects/one-click-tools
```

## Recommended smoke-test sequence

### 1. Confirm worktree state

```bash
git rev-parse --short HEAD
git status --short
git branch -vv | sed -n '1,5p'
```

Checklist:
- confirm the expected branch/commit
- confirm there are no unintended local changes

### 2. Build the Go CLI

```bash
go build -o oct main.go
./oct --version
./oct help >/dev/null
```

Checklist:
- `go build` succeeds
- the built `./oct` runs correctly

### 3. Run Go tests

```bash
GOTOOLCHAIN=auto go test ./...
```

Checklist:
- full Go test suite passes
- no regressions in CLI command / usage / schedule behavior

### 4. Build the Swift helper

```bash
./oct menubar build-helper
```

Checklist:
- `macos/OctMenubar/.build/debug/OctMenubarApp` is produced
- no Xcode/Swift toolchain issues

### 5. Run the menubar snapshot test

```bash
swift test --package-path macos/OctMenubar --filter UsageSnapshotTests
```

Checklist:
- no regression in recent menubar usage snapshot expectations
- timezone/date-formatting changes do not break the snapshot test

## Optional additional smoke tests

Add these depending on the change scope.

### helper diagnosis / install validation

```bash
./oct menubar doctor
./oct menubar install-helper
./oct menubar doctor
```

Confirm:
- `launch mode: swift-helper`
- installed helper path resolves to `~/.local/bin/OctMenubarApp` or the repo-local build artifact

### standalone helper resolution

```bash
cp ./oct /tmp/oct-standalone
/tmp/oct-standalone menubar doctor
```

Confirm:
- a binary outside the repo still finds the installed helper

### usage JSON smoke

```bash
./oct usage --json | head
```

Confirm:
- non-TTY JSON output still renders correctly

## Recommended cleanup

If the generated artifacts are not needed after validation:

```bash
rm -f ./oct
rm -rf macos/OctMenubar/.build
```

Note:
- do not blindly remove `node_modules` or `dist`; they may still be needed for ongoing work.

## Verified baseline as of 2026-06-15

The following combination was verified successfully:

```bash
go test ./...
go build ./...
swift build --package-path macos/OctMenubar
swift test --package-path macos/OctMenubar --filter UsageSnapshotTests
```

Related commit at that time:
- `4581aae` `test(menubar): fix timezone expectations in usage snapshot test`

## First things to check on failure

1. `xcode-select -p`
2. `swift --version`
3. `go version`
4. `./oct menubar doctor`
5. `git status --short`

On remote SSH validation, the first root cause is often Xcode selection, Swift toolchain state, or a dirty worktree rather than the CLI logic itself.
