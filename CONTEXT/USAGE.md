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
- **Cursor** (`cursor-agent`)

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
- `OCT_CURSOR_USAGE_URL` — Cursor remote usage endpoint (optional; falls back to local workspaceStorage count)
- `CURSOR_API_KEY` — Bearer token sent to `OCT_CURSOR_USAGE_URL` if set

### GitHub Copilot Endpoint Resolution
If `OCT_COPILOT_USAGE_ENDPOINT` is not set, `oct` resolves it based on:
1. **Enterprise**: `OCT_GITHUB_ENTERPRISE` (or `GITHUB_ENTERPRISE`)
2. **Organization**: `OCT_GITHUB_ORG` (or `GITHUB_ORG`)
3. **User**: `OCT_GITHUB_USER` (or `GITHUB_USER`)
4. **Auto-lookup**: If all are unset, it uses `gh auth token` and `GET /user` to find the current user.

When `GITHUB_API_TOKEN`/`GITHUB_TOKEN` is not configured, `oct` also attempts `gh auth token` as a token-less CLI fallback before using local session logs.

### Copilot Usage Filtering
- `OCT_COPILOT_USAGE_YEAR`
- `OCT_COPILOT_USAGE_MONTH`
- `OCT_COPILOT_USAGE_DAY`
- `OCT_COPILOT_USAGE_MODEL`
- `OCT_COPILOT_USAGE_PRODUCT`

---

### Cursor Usage Notes

Fetch priority order:
1. **Custom endpoint** (`OCT_CURSOR_USAGE_URL`): any JSON endpoint, optionally authenticated via `CURSOR_API_KEY`
2. **Local auth token** (`~/.config/cursor/auth.json`): automatically reads `accessToken` and calls `https://api2.cursor.sh/auth/usage`; returns per-model monthly `numRequestsTotal` with `source: local-auth`
3. **Workspace storage count** (fallback): counts workspace directories as a proxy for session count, `source: local`

Override the known API URL for testing:
- `OCT_CURSOR_API_USAGE_URL` — replaces `https://api2.cursor.sh/auth/usage`

Auth token paths:
- Linux: `~/.config/cursor/auth.json`
- macOS: `~/.config/cursor/auth.json` or `~/Library/Application Support/cursor/auth.json`
- Windows: `%APPDATA%\cursor\auth.json`

Set `OCT_USAGE_DEBUG=1` to expose per-model breakdown in `source_detail`.

---

## 4. Experimental Features

### OAuth-based Usage Tracking
Experimental mode inspired by `codex-opero` to use local OAuth/session state:

```bash
oct usage --experimental-oauth-usage
oct usage --experimental-oauth-usage --json
```

Gemini token-less fallback order:
1. `~/.gemini/oauth_creds.json`
2. `gcloud auth print-access-token`
3. Local session summary from `~/.gemini/antigravity/conversations`

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
