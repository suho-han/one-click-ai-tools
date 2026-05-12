# 아이콘 가이드

`one-click-tools`는 Lobe Icons 메타데이터를 사용해 에이전트 아이콘을 매핑합니다.

현재 지원/매핑:
- Claude Code -> `ClaudeCode`
- OpenAI Codex -> `Codex`
- Gemini CLI -> `GeminiCLI`
- GitHub Copilot -> `GithubCopilot`
- Cursor -> `Cursor` (내장 아이콘)
- OpenCode -> `OpenCode` (내장 아이콘)

코드 위치:
- 아이콘 매핑 정의: `internal/update/tools.go`
- 렌더링/fallback: `internal/ui/`
