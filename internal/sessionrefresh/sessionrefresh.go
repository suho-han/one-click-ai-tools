package sessionrefresh

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/suho-han/one-click-tools/internal/execenv"
	"github.com/suho-han/one-click-tools/internal/update"
)

type RefreshOptions struct {
	Providers []string
	DryRun    bool
}

type RefreshResult struct {
	Provider   string `json:"provider"`
	Supported  bool   `json:"supported"`
	Mode       string `json:"mode"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	SourcePath string `json:"source_path,omitempty"`
}

type refresher func(RefreshOptions, update.Tool) RefreshResult

var refreshers = map[string]refresher{
	"agy":          probeAntigravitySession,
	"claude":       unsupportedProbe("probe", "no verified token-free auth/session probe yet"),
	"codex":        probeCodexSession,
	"cursor-agent": probeCursorSession,
	"copilot":      unsupportedProbe("probe", "no verified token-free auth/session probe yet"),
	"opencode":     unsupportedProbe("probe", "no verified token-free auth/session probe yet"),
}

var (
	refreshLookPath = execenv.LookPath
	refreshCommand  = execenv.Command
)

func Refresh(opts RefreshOptions) []RefreshResult {
	providers := opts.Providers
	if len(providers) == 0 {
		for _, tool := range update.Tools {
			providers = append(providers, tool.BinaryName)
		}
	}

	results := make([]RefreshResult, 0, len(providers))
	seen := map[string]bool{}
	for _, provider := range providers {
		normalized := update.NormalizeToolName(provider)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		tool, ok := resolveTool(normalized)
		if !ok {
			results = append(results, RefreshResult{Provider: provider, Supported: false, Mode: "probe", Status: "unsupported", Message: "provider not recognized"})
			continue
		}
		refresh := refreshers[normalized]
		if refresh == nil {
			refresh = unsupportedProbe("probe", "no token-free refresh strategy registered")
		}
		results = append(results, refresh(opts, tool))
	}
	return results
}

func resolveTool(name string) (update.Tool, bool) {
	for _, tool := range update.Tools {
		if tool.MatchesName(name) {
			return tool, true
		}
	}
	return update.Tool{}, false
}

func unsupportedProbe(mode, message string) refresher {
	return func(opts RefreshOptions, tool update.Tool) RefreshResult {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: mode, Status: "unsupported", Message: message}
	}
}

func probeCodexSession(opts RefreshOptions, tool update.Tool) RefreshResult {
	if opts.DryRun {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "skipped", Message: "would run 'codex login status'"}
	}
	if _, err := refreshLookPath("codex"); err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: "auth-status", Status: "unsupported", Message: "codex binary not installed"}
	}
	cmd := refreshCommand("codex", "login", "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "error", Message: trimCommandOutput(out, err)}
	}
	msg := firstNonEmptyLine(string(out))
	if msg == "" {
		msg = "login status confirmed"
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "ok", Message: msg}
}

func probeCursorSession(opts RefreshOptions, tool update.Tool) RefreshResult {
	binary, err := findFirstBinary("agent", "cursor-agent")
	if err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: "local-auth", Status: "unsupported", Message: "Cursor CLI binary not installed (expected 'agent')"}
	}
	home, _ := os.UserHomeDir()
	paths := cursorAuthPaths(home)
	path := firstExistingPath(paths...)
	if opts.DryRun {
		msg := fmt.Sprintf("would inspect Cursor auth files via %s", binary)
		if path != "" {
			msg += " and local auth.json"
		}
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-auth", Status: "skipped", Message: msg, SourcePath: path}
	}
	if path == "" {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-auth", Status: "skipped", Message: fmt.Sprintf("%s found, but no local Cursor auth.json detected", binary)}
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-auth", Status: "ok", Message: fmt.Sprintf("Cursor auth.json detected via %s", binary), SourcePath: path}
}

func probeAntigravitySession(opts RefreshOptions, tool update.Tool) RefreshResult {
	binary, err := findFirstBinary("agy", "antigravity", "gemini")
	if err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: "local-session", Status: "unsupported", Message: "Antigravity binary not installed (expected 'agy')"}
	}
	home, _ := os.UserHomeDir()
	path := firstExistingDir(
		filepath.Join(home, ".gemini", "antigravity", "conversations"),
		filepath.Join(home, ".gemini", "antigravity-cli", "cache"),
		filepath.Join(home, ".gemini", "antigravity-cli", "projects"),
	)
	if opts.DryRun {
		msg := fmt.Sprintf("would inspect local Antigravity session artifacts via %s", binary)
		if path != "" {
			msg += " and existing session storage"
		}
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-session", Status: "skipped", Message: msg, SourcePath: path}
	}
	if path == "" {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-session", Status: "skipped", Message: fmt.Sprintf("%s found, but no local Antigravity session artifacts detected", binary)}
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-session", Status: "ok", Message: fmt.Sprintf("Local Antigravity session artifacts detected via %s", binary), SourcePath: path}
}

func cursorAuthPaths(home string) []string {
	if strings.TrimSpace(home) == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".config", "cursor", "auth.json"),
		filepath.Join(home, "Library", "Application Support", "cursor", "auth.json"),
	}
}

func findFirstBinary(names ...string) (string, error) {
	for _, name := range names {
		if path, err := refreshLookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("binary not found")
}

func firstExistingDir(paths ...string) string {
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
	return ""
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func trimCommandOutput(out []byte, err error) string {
	joined := strings.TrimSpace(string(bytes.TrimSpace(out)))
	if joined == "" {
		return err.Error()
	}
	return joined
}
