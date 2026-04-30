package update

type Tool struct {
	Name       string
	Package    string
	BinaryName string
}

var Tools = []Tool{
	{Name: "Claude Code", Package: "@anthropic-ai/claude-code", BinaryName: "claude"},
	{Name: "OpenAI Codex", Package: "@openai/codex", BinaryName: "codex"},
	{Name: "Gemini CLI", Package: "@google/gemini-cli", BinaryName: "gemini"},
	{Name: "GitHub Copilot", Package: "@github/copilot", BinaryName: "copilot"},
}
