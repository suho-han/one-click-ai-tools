# Changelog

All notable changes to this project will be documented in this file.

## 0.4.2 (2026-05-09)

### Bug Fixes
- recover from npm install conflicts during `oct update`

### Documentation
- reorganize localized README and `CONTEXT/` docs

## 0.4.0 (2026-05-03)

### Features
- Synchronized versioning with npm repository (re-basing from v0.3.7).
- Baseline update for production release.

## 0.3.7 (2026-04-30)

### Chore
- release: 0.3.7

## 0.3.6 (2026-04-28)

### Chore
- release: 0.3.6
- revert npm package name to one-click-tools

## 0.3.5 (2026-04-25)

### Features
- align agent-update icon layout with config
- improve interactive selection UX and summary output
- integrate @lobehub/icons metadata

## 0.3.4 (2026-04-20)

### Features
- add icons to agent-update and refine braille alignment
- implement high-density Braille icon renderer
- implement terminal image rendering for @lobehub/icons

## 0.3.1 (2026-04-10)

### Features
- complete v0.3.0 TODOs (Usage APIs, Package Manager detection, CI/CD, Tests)
- migrate to Go architecture (v0.3.0)

## 0.2.5 (2026-04-01)

### Features
- add config and schedule commands
- improve update UI and summary

## 0.2.2 (2026-03-25)

### Fixes
- fallback dotenv lookup to ~/.env for global oct
- resolve symlink for lib path when installed via npm global

## 0.2.0 (2026-03-15)

### Features
- copilot usage api rewrite
- support copilot billing usage query filters

## 0.1.1 (2026-03-05)

### Features
- initial release improvements
- add self-update command with beta channel

## 0.1.0 (2026-05-02)

### Features
- Initial reset release of one-click-tools (oct).
- Support for Claude, Gemini, Codex, and Copilot usage reporting.
- Automated update scheduling for macOS, Linux, and Windows.
- Interactive configuration system.
- Standardized authentication flow for all AI providers.
