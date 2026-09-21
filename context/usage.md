# Detailed Usage Guide (oct)

This page covers core commands and operational configuration for `one-click-tools`.

## 1) `oct update`

Updates `oct` itself.

```bash
oct update           # update to the latest GitHub Release
oct update --check   # check for a newer release without installing
```

## 2) `oct agent-update`

Updates all supported agents.

Supported agents:
- Claude Code (`@anthropic-ai/claude-code`)
- Command Code (`command-code`, binary: `commandcode` / `cmd`)
- OpenAI Codex (`@openai/codex`)
- Antigravity CLI (official installer, binary: `agy`)
- GitHub Copilot (`@github/copilot`)
- Cursor (`cursor-agent`)
- OpenCode (`opencode-ai`)
- Kimi Code (`@moonshot-ai/kimi-code`, binary: `kimi`) (experimental)
- Qwen Code (`@qwen-code/qwen-code`, binary: `qwen`) (experimental)
- MiniMax (`mmx-cli`, binary: `mmx`) (experimental)

Default behavior:
- macOS: `brew update/upgrade`, npm-based updates, then fallback paths when needed
- Linux: npm-based updates, with `sudo` path on permission failures

Logs:
- `~/.oct/logs/agent-update-YYYYMMDD-HHMMSS.log`

## 3) `oct usage`

Collects and prints usage from configured providers.

```bash
oct usage
oct usage --compact
oct usage --json
oct usage --notify
```

Notes:
- In non-TTY environments (CI/pipes), output auto-switches to JSON.
- `--compact` prints remaining percent in one line, for example `C-88% X-45% G-72%` (`Claude=C`, `Codex=X`, `Gemini/Antigravity=G`).
- Alert logic applies when `--notify` is set or `usage_alert_enabled=true`.
- Selected providers are filtered by `enabled_tools`, and output order follows `agent_order`.
- Claude Code falls back to parsing `claude --print /usage` for 5h/weekly quota when the OAuth API reports no utilization.
- Antigravity parses quota from `agy --print /usage` without reading tokens or keychain data directly.
- Command Code reads 5h/7d/monthly buckets from the billing API using `COMMAND_CODE_API_KEY` or `~/.commandcode/auth.json`.
- Kimi Code reads weekly + 5-hour request windows from `api.kimi.com/coding/v1/usages` (`KIMI_CODE_API_KEY`, `~/.kimi-code/credentials/kimi-code.json`, or the rotated `kimi-code-env-*.json` files the current CLI writes; access tokens expire after ~15 minutes and are only refreshed by the kimi CLI itself).
- Qwen Code has no public usage API; oct counts today's local token-usage records (`~/.qwen/**/usage/token-usage-*.jsonl`) against `qwen_daily_limit` (default 100, this machine only).
- MiniMax plan quota (5h + weekly) comes from `POST /v1/coding_plan/remains` (`MINIMAX_CODING_API_KEY` or `MINIMAX_API_KEY`; `MINIMAX_REGION=cn` switches to the mainland host).
- Standalone providers (`zai`, `deepseek`, `openrouter`, `grok`) have no CLI behind them and are fetched only when listed in `agent_order` or `enabled_tools`.
- `(experimental)` marks providers whose response fields were written from community sources and not yet confirmed against a live subscription (2026-09-20: live-verified = codex, claude, commandcode, opencode, antigravity, openrouter; kimi auth path verified, schema pending quota reset).
- Legacy config values `gemini` and `gemini-cli` are still accepted, but they normalize internally to `agy`.

### Key environment variables

- Common endpoint overrides (all verified against the code):
  - `OCT_CODEX_USAGE_ENDPOINT`
  - `OCT_COMMANDCODE_API_BASE_URL`
  - `OCT_OPENCODE_USAGE_ENDPOINT`
  - `OCT_KIMI_USAGE_ENDPOINT`
  - `OCT_ZAI_USAGE_ENDPOINT`
  - `OCT_DEEPSEEK_BALANCE_ENDPOINT`
  - `OCT_OPENROUTER_USAGE_ENDPOINT`
  - `OCT_MINIMAX_USAGE_ENDPOINT`
  - `OCT_GROK_USAGE_ENDPOINT`, `OCT_GROK_SETTINGS_ENDPOINT`
- Cursor:
  - `OCT_CURSOR_USAGE_URL` (custom remote endpoint)
  - `CURSOR_API_KEY` (Bearer token used with `OCT_CURSOR_USAGE_URL`)
  - `OCT_CURSOR_API_USAGE_URL` (override for `https://api2.cursor.sh/auth/usage`)
- Copilot:
  - `OCT_COPILOT_USER_ENDPOINT` (override for the quota API endpoint)
- Debug:
  - `OCT_USAGE_DEBUG=1` (shows richer provider source details)

Note: earlier revisions of this file listed `OCT_CLAUDE_USAGE_ENDPOINT`, `OCT_COPILOT_USAGE_ENDPOINT`, and `OCT_COPILOT_USAGE_*` filter variables; none of them exist in the code (Claude's endpoint is hardcoded, Copilot's override is `OCT_COPILOT_USER_ENDPOINT`).

### Cursor usage fetch priority

1. `OCT_CURSOR_USAGE_URL` (custom remote)
2. Local auth token (`~/.config/cursor/auth.json`) + Cursor API
3. Local workspaceStorage fallback

### OpenCode usage source

OpenCode usage is read from local session logs first.

Primary paths:
- `~/.opencode/sessions`
- `~/.config/opencode/sessions`
- `~/.local/share/opencode/sessions`

### Statusbar output modes (`--format`)

`oct usage --format waybar|polybar|swiftbar` renders the same results for
Linux/macOS statusbars (roadmap P2 cross-platform parity with the macOS-only
menubar):

- `waybar`: one-line JSON `{"text","tooltip","class"}` for a waybar
  custom/script module; `class` is `ok|warn|error` using the shared severity
  thresholds (percent used >= 85 warn, >= 95 crit).
- `polybar`: one line for a polybar custom script; literal `%` is doubled and
  per-provider tokens carry `%{F#...}` severity colors.
- `swiftbar`: SwiftBar plugin protocol — severity-colored title line, `---`,
  then one dropdown line per provider.

All three honor `usage_display_mode` (normalized used/remaining) like the
menubar title; `--compact` remains pinned to "remaining". Providers without a
usable value (unconfigured tools, empty billing windows, local-only estimates
— the "?" tokens) are hidden from statusline output, while real failures
(e.g. HTTP 401s) stay visible; the waybar tooltip / swiftbar dropdown end
with a count of the hidden providers. Pair with `--from-snapshot` to render
`~/.oct/state/usage-latest.json` (written by `oct monitor`) instead of a live
fetch, so short statusbar poll intervals do not trigger a provider fan-out. Renderers live in
`internal/usage/statusline.go`; `UsageSeverity` there is the shared
classification also used by `oct monitor`.

## 4) Related docs

- [monitoring.md](monitoring.md): continuous live view instead of one-shot printing (`oct monitor`)
- [usage_alerts.md](usage_alerts.md): threshold-based OS alert configuration behind `oct usage --notify`
