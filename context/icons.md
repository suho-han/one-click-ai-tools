# Icon Guide

Everything about provider icon mapping and rendering in one place.

`one-click-tools` uses Lobe Icons metadata to map provider icons.

Current mappings (source of truth: `LobeIcon` fields in `internal/update/tools.go`):
- Claude Code -> `ClaudeCode`
- Command Code -> `CommandCode`
- OpenAI Codex -> `Codex`
- Gemini CLI / Antigravity -> `GeminiCLI`
- GitHub Copilot -> `GithubCopilot`
- Cursor -> `Cursor`
- OpenCode -> `OpenCode`
- Kimi Code -> `Moonshot`
- Qwen Code -> `QwenCode`
- MiniMax -> `MiniMax`

Standalone usage providers (`zai`, `deepseek`, `openrouter`, `grok`) have no
update tool entry, so they have no icon mapping.

## Rendering

Renderer fallback order depends on terminal capability.

- Order: `native_image` -> `ansi_asset` -> `text`
- Override: `OCT_ICON_RENDERER=native_image|ansi_asset|text`

Code locations:
- mapping definition: `internal/update/tools.go`
- rendering/fallback: `internal/ui/`
