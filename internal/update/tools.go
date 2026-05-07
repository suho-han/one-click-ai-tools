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

func (t Tool) BrewTarget() string {
	if t.BrewPackage != "" {
		return t.BrewPackage
	}
	return t.BinaryName
}

func (t Tool) hexRGB() (uint8, uint8, uint8, bool) {
	if len(t.HexColor) != 7 || t.HexColor[0] != '#' {
		return 0, 0, 0, false
	}
	r, err1 := strconv.ParseUint(t.HexColor[1:3], 16, 8)
	g, err2 := strconv.ParseUint(t.HexColor[3:5], 16, 8)
	b, err3 := strconv.ParseUint(t.HexColor[5:7], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return uint8(r), uint8(g), uint8(b), true
}

func (t Tool) Colorize(text string) string {
	r, g, b, ok := t.hexRGB()
	if !ok {
		return text
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", r, g, b, text)
}

func (t Tool) ColorizeWithBackgroundBlackText(text string) string {
	r, g, b, ok := t.hexRGB()
	if !ok {
		return text
	}
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

	for _, t := range Tools {
		if _, ok := toolMap[strings.ToLower(t.BinaryName)]; ok {
			ordered = append(ordered, t)
		}
	}

	return ordered
}

func GetFilteredTools(enabled []string, ordered []Tool) []Tool {
	if len(enabled) == 0 {
		return ordered
	}
	var result []Tool
	for _, et := range enabled {
		for _, t := range ordered {
			if strings.EqualFold(et, t.BinaryName) {
				result = append(result, t)
				break
			}
		}
	}
	return result
}
