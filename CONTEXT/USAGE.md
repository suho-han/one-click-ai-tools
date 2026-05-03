# Detailed Usage Guide (oct)

This document provides in-depth information about the commands and configuration options available in `one-click-tools`.

## 1. `oct update`

Updates `oct` itself to the latest version.

- **Stable Version**: `oct update`
- **Beta Version**: `oct update --beta`

## 2. `oct agent-update`

Checks and updates all supported AI CLI tools.

### Behavior by OS
- **macOS**:
  - Runs `brew update` and `brew upgrade` first.
  - Updates/installs agent CLIs via `npm`.
  - If `npm` fails, it retries via Homebrew formula/cask upgrade when available.
  - If a tool exists but is not npm-managed, it attempts a non-npm update path (e.g., `brew upgrade` or tool-specific self-update).
- **Ubuntu**:
  - Updates/installs agent CLIs via `npm`.
  - Automatically attempts to use `sudo` if permission errors are detected.

### Logging
Execution logs are saved to `~/.oct/logs/agent-update-YYYYMMDD-HHMMSS.log`. The final log path is printed upon completion.

---

## 3. `oct usage`

Collects and displays usage statistics for AI tools.

### Supported Models
- **Claude Code** (`claude-code`)
- **OpenAI Codex** (`codex`)
- **Gemini CLI** (`gemini`)
- **GitHub Copilot** (`copilot`)

### Machine-Readable Output
```bash
oct usage --json
```

### Environment Variables for Custom Endpoints
You can override the default API endpoints using the following environment variables:

- `OCT_CODEX_USAGE_ENDPOINT`
- `OCT_CLAUDE_USAGE_ENDPOINT`
- `OCT_GEMINI_USAGE_ENDPOINT`
- `OCT_COPILOT_USAGE_ENDPOINT`
- `OCT_GEMINI_API_ENDPOINT` (Experimental Gemini path)

### GitHub Copilot Endpoint Resolution
If `OCT_COPILOT_USAGE_ENDPOINT` is not set, `oct` resolves it based on:
1. **Enterprise**: `OCT_GITHUB_ENTERPRISE` (or `GITHUB_ENTERPRISE`)
2. **Organization**: `OCT_GITHUB_ORG` (or `GITHUB_ORG`)
3. **User**: `OCT_GITHUB_USER` (or `GITHUB_USER`)
4. **Auto-lookup**: If all are unset, it uses `gh auth token` and `GET /user` to find the current user.

### Copilot Usage Filtering
- `OCT_COPILOT_USAGE_YEAR`
- `OCT_COPILOT_USAGE_MONTH`
- `OCT_COPILOT_USAGE_DAY`
- `OCT_COPILOT_USAGE_MODEL`
- `OCT_COPILOT_USAGE_PRODUCT`

---

## 4. Experimental Features

### OAuth-based Usage Tracking
Experimental mode inspired by `codex-opero` to use local OAuth/session state:

```bash
oct usage --experimental-oauth-usage
oct usage --experimental-oauth-usage --json
```

---

## 5. UI and Icon Rendering

`oct` detects terminal capabilities and saves them to `~/.oct/terminal-capabilities.json`.

### Icon Renderer Fallback Order
`native_image` -> `ansi_asset` -> `text`

### Override Renderer
Set `OCT_ICON_RENDERER` to one of:
- `native_image`
- `ansi_asset`
- `text`
