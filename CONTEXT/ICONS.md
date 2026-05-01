# AI Tools Icons (Powered by Lobe Icons)

This project uses icon metadata from [@lobehub/icons](https://icons.lobehub.com) to represent AI tools.

## Supported Tools

| Tool | Icon (Static SVG) | Lobe Icon Name |
| :--- | :---: | :--- |
| **Claude Code** | <img src="https://registry.npmmirror.com/@lobehub/icons-static-svg/latest/files/icons/claude-code.svg" height="32" /> | `ClaudeCode` |
| **OpenAI Codex** | <img src="https://registry.npmmirror.com/@lobehub/icons-static-svg/latest/files/icons/codex.svg" height="32" /> | `Codex` |
| **Gemini CLI** | <img src="https://registry.npmmirror.com/@lobehub/icons-static-svg/latest/files/icons/gemini-cli.svg" height="32" /> | `GeminiCLI` |
| **GitHub Copilot** | <img src="https://registry.npmmirror.com/@lobehub/icons-static-svg/latest/files/icons/github-copilot.svg" height="32" /> | `GithubCopilot` |

## Integration

The icon metadata is integrated into the core `Tool` struct in `internal/update/tools.go`:

```go
type Tool struct {
    Name        string
    Icon        string // Terminal Emoji
    LobeIcon    string // LobeHub Icon Identifier
    // ...
}
```
