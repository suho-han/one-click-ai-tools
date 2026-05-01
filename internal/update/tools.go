package update

type Tool struct {
	Name        string
	Package     string
	BinaryName  string
	BrewPackage string
	Icon        string
	LobeIcon    string
}

var Tools = []Tool{
	{Name: "Claude Code", Package: "@anthropic-ai/claude-code", BinaryName: "claude", BrewPackage: "claude-code", Icon: "🤖", LobeIcon: "ClaudeCode"},
	{Name: "OpenAI Codex", Package: "@openai/codex", BinaryName: "codex", BrewPackage: "codex", Icon: "⚛️", LobeIcon: "Codex"},
	{Name: "Gemini CLI", Package: "@google/gemini-cli", BinaryName: "gemini", BrewPackage: "gemini-cli", Icon: "✨", LobeIcon: "GeminiCLI"},
	{Name: "GitHub Copilot", Package: "@github/copilot", BinaryName: "copilot", Icon: "🐙", LobeIcon: "GithubCopilot"},
}
