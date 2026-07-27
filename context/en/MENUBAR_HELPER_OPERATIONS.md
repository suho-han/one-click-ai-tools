# Menubar Helper Operations Guide

This document records the operational workflow for building, installing, diagnosing, and remotely validating the macOS menubar helper (`OctMenubarApp`).

## Scope

Use this guide when you need to:
- build and install the Swift helper on a macOS development machine
- verify helper resolution priority for repo-local builds vs standalone `oct` binaries
- validate the helper flow over SSH on a remote macOS host

## Core commands

```bash
oct menubar doctor
oct menubar build-helper
oct menubar install-helper
```

What each command does:
- `doctor`: reports launch mode, resolved helper path, and helper project location
- `build-helper`: builds the Swift helper under `macos/OctMenubar`
- `install-helper`: installs the built helper into the user-local executable path

## Expected behavior

### 1. Before helper installation

Example `oct menubar doctor` result:
- `launch mode: swift-package`
- helper not found
- helper project detected at `macos/OctMenubar`

Meaning:
- the Swift helper has not been built/installed yet, but the command can run the Swift package source directly and open the new popover UI

If the Swift toolchain or helper project is unavailable, `launch mode: legacy-fallback` still uses the legacy NSMenu path.

### 2. Immediately after repo-local build

```bash
oct menubar build-helper
oct menubar doctor
```

Expected result:
- helper build succeeds
- build artifact example: `macos/OctMenubar/.build/debug/OctMenubarApp`
- `doctor` reports `launch mode: swift-helper`
- helper path resolves to the worktree build artifact

Meaning:
- during development, the repo-local build artifact should be preferred even before installation

### 3. After install

```bash
oct menubar install-helper
oct menubar doctor
```

Expected result:
- install path: `~/.local/bin/OctMenubarApp`
- binary type: `Mach-O 64-bit executable arm64` (for Apple Silicon)
- `doctor` reports `launch mode: swift-helper`

Meaning:
- the installed helper should allow menubar launch outside the development worktree

## Standalone binary validation

Check that a copied binary outside the repo still finds the installed helper.

```bash
cp ./oct /tmp/oct-standalone
/tmp/oct-standalone menubar doctor
```

Expected result:
- `launch mode: swift-helper`
- helper path resolves to `~/.local/bin/OctMenubarApp`

Meaning:
- standalone distributed binaries can still use the installed helper correctly

## Remote macOS validation procedure

Example remote host:
- host: `100.114.89.25`
- user: `suhohan`

Example fresh workspace:
- `/tmp/oct-remote-validate`

Recommended sequence:
1. upload and unpack the current worktree into a remote temp directory
2. `go build -o oct .`
3. `./oct menubar doctor`
4. `./oct menubar build-helper`
5. `./oct menubar install-helper`
6. `./oct menubar doctor`
7. `/tmp/oct-standalone menubar doctor`

## Verified results on 2026-06-14

Confirmed on the remote macOS host:
- `build-helper` succeeded
- `install-helper` succeeded
- installed helper path: `/Users/suhohan/.local/bin/OctMenubarApp`
- installed helper type: `Mach-O 64-bit executable arm64`
- same-worktree `doctor` chose the build artifact as helper path
- standalone `doctor` chose the installed helper as helper path

This confirms both cases:
- dev-worktree-first resolution
- installed-helper fallback resolution

## Operational notes

### non-interactive SSH PATH

On the remote macOS host, plain non-interactive SSH may omit Homebrew bins from PATH.

Recommended bootstrap:

```bash
export PATH=/opt/homebrew/bin:$PATH
```

Without that bootstrap, remote automation may fail to find `oct`, `node`, or `npm`.

### build prerequisites

`build-helper` requires macOS plus a working Swift/Xcode toolchain.
You cannot validate helper builds on this Linux host.

## Troubleshooting order

1. `oct menubar doctor`
2. confirm `macos/OctMenubar` exists
3. `oct menubar build-helper`
4. confirm `~/.local/bin/OctMenubarApp` exists and is executable
5. rerun standalone `menubar doctor`

## Related docs

- `README.md`: user-facing quick start and command overview
- `PROJECT_CONTEXT/remote-macos-validation-status.md`: detailed remote validation record
- `CONTEXT/en/LOCAL_TEST.md`: general local build/test guide
- `CONTEXT/en/MACBOOK_AIR_SMOKE_TEST.md`: quick smoke-test flow for the primary macOS host
