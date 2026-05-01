package update

import (
	"fmt"
	"strconv"
	"strings"
)

type Tool struct {
	Name        string
	Package     string
	BinaryName  string
	BrewPackage string
	Icon        string
	LobeIcon    string
	HexColor    string
}

func (t Tool) Colorize(text string) string {
	if t.HexColor == "" || !strings.HasPrefix(t.HexColor, "#") || len(t.HexColor) != 7 {
		return text
	}

	r, _ := strconv.ParseUint(t.HexColor[1:3], 16, 8)
	g, _ := strconv.ParseUint(t.HexColor[3:5], 16, 8)
	b, _ := strconv.ParseUint(t.HexColor[5:7], 16, 8)

	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", r, g, b, text)
}

var Tools = []Tool{
	{
		Name:        "Claude Code",
		Package:     "@anthropic-ai/claude-code",
		BinaryName:  "claude",
		BrewPackage: "claude-code",
		Icon:        "🤖",
		LobeIcon:    "ClaudeCode",
		HexColor:    "#D97757",
	},
	{
		Name:        "OpenAI Codex",
		Package:     "@openai/codex",
		BinaryName:  "codex",
		BrewPackage: "codex",
		Icon:        "⚛️",
		LobeIcon:    "Codex",
		HexColor:    "#00A67E",
	},
	{
		Name:        "Gemini CLI",
		Package:     "@google/gemini-cli",
		BinaryName:  "gemini",
		BrewPackage: "gemini-cli",
		Icon:        "✨",
		LobeIcon:    "GeminiCLI",
		HexColor:    "#4285F4",
	},
	{
		Name:        "GitHub Copilot",
		Package:     "@github/copilot",
		BinaryName:  "copilot",
		Icon:        "🐙",
		LobeIcon:    "GithubCopilot",
		HexColor:    "#BC8CF2",
	},
}
