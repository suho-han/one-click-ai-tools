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

func (t Tool) ColorizeWithBackgroundBlackText(text string) string {
	if t.HexColor == "" || !strings.HasPrefix(t.HexColor, "#") || len(t.HexColor) != 7 {
		return text
	}

	r, _ := strconv.ParseUint(t.HexColor[1:3], 16, 8)
	g, _ := strconv.ParseUint(t.HexColor[3:5], 16, 8)
	b, _ := strconv.ParseUint(t.HexColor[5:7], 16, 8)

	// Background uses the tool color, foreground uses black.
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm\x1b[30m%s\x1b[0m", r, g, b, text)
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
		Name:        "Cursor",
		Package:     "cursor-agent",
		BinaryName:  "cursor-agent",
		BrewPackage: "cursor-agent",
		Icon:        "▣",
		LobeIcon:    "",
		HexColor:    "#111111",
	},
	{
		Name:        "OpenCode",
		Package:     "opencode-ai",
		BinaryName:  "opencode",
		BrewPackage: "opencode",
		Icon:        "🧩",
		LobeIcon:    "",
		HexColor:    "#4F46E5",
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
		Name:       "GitHub Copilot",
		Package:    "@github/copilot",
		BinaryName: "copilot",
		Icon:       "🐙",
		LobeIcon:   "GithubCopilot",
		HexColor:   "#BC8CF2",
	},
}

func GetOrderedTools(order []string) []Tool {
	if len(order) == 0 {
		return Tools
	}

	var ordered []Tool
	toolMap := make(map[string]Tool)
	for _, t := range Tools {
		toolMap[strings.ToLower(t.BinaryName)] = t
	}

	for _, name := range order {
		if t, ok := toolMap[strings.ToLower(name)]; ok {
			ordered = append(ordered, t)
			delete(toolMap, strings.ToLower(name))
		}
	}

	// Optionally append remaining tools that were not in the order list
	// For now, we only return what was requested to strictly follow the order,
	// or everything if order is empty.
	// But to be safe, let's append the rest if some were missing from the order list.
	for _, t := range Tools {
		if _, ok := toolMap[strings.ToLower(t.BinaryName)]; ok {
			ordered = append(ordered, t)
		}
	}

	return ordered
}
