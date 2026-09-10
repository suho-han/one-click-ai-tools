# Icon Guide

Everything about provider icon mapping and rendering in one place.

`one-click-tools` uses Lobe Icons metadata to map provider icons.

Current mappings:
- Claude Code -> `ClaudeCode`
- OpenAI Codex -> `Codex`
- Gemini CLI -> `GeminiCLI`
- GitHub Copilot -> `GithubCopilot`
- Cursor -> text fallback (no Lobe icon)
- OpenCode -> text fallback (no Lobe icon)

## Rendering

Renderer fallback order depends on terminal capability.

- Order: `native_image` -> `ansi_asset` -> `text`
- Override: `OCT_ICON_RENDERER=native_image|ansi_asset|text`

Code locations:
- mapping definition: `internal/update/tools.go`
- rendering/fallback: `internal/ui/`
