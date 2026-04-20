# awesome-dev-tools

Update and bootstrap popular AI CLI tools with a single command.

## What it does

- Detects OS (`macOS` or `Ubuntu`)
- For each tool, installs if missing and updates if present
- Supports:
  - Claude Code
  - OpenAI Codex CLI
  - Gemini CLI
  - GitHub Copilot CLI

## Install

```bash
npm install -g awesome-dev-tools
```

## Usage

```bash
awesome-dev-tools
```

## Notes

- On Ubuntu, global npm operations may require `sudo`.
- If required package manager is missing (`brew` on macOS, `npm` on Ubuntu), the script prints a warning and exits.
