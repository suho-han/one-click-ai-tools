package sessionrefresh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	Confidence string `json:"confidence,omitempty"`
	Message    string `json:"message"`
	SourcePath string `json:"source_path,omitempty"`
}

type refresher func(RefreshOptions, update.Tool) RefreshResult

var refreshers = map[string]refresher{
	"agy":          probeAntigravitySession,
	"claude":       probeClaudeSession,
	"codex":        probeCodexSession,
	"cursor-agent": probeCursorSession,
	"copilot":      probeCopilotSession,
	"opencode":     probeOpenCodeSession,
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

const (
	confidenceVerified = "verified"
	confidencePartial  = "partial"
)

func unsupportedProbe(mode, message string) refresher {
	return func(opts RefreshOptions, tool update.Tool) RefreshResult {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: mode, Status: "unsupported", Message: message}
	}
}

func probeClaudeSession(opts RefreshOptions, tool update.Tool) RefreshResult {
	if opts.DryRun {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "skipped", Confidence: confidenceVerified, Message: "would run 'claude auth status --json'"}
	}
	if _, err := refreshLookPath("claude"); err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: "auth-status", Status: "unsupported", Message: "Claude CLI binary not installed (expected 'claude')"}
	}
	cmd := refreshCommand("claude", "auth", "status", "--json")
	out, err := cmd.CombinedOutput()

	var data struct {
		LoggedIn   bool   `json:"loggedIn"`
		AuthMethod string `json:"authMethod"`
	}
	if json.Unmarshal(bytes.TrimSpace(out), &data) == nil {
		if data.LoggedIn {
			msg := "Claude auth status confirmed"
			if strings.TrimSpace(data.AuthMethod) != "" {
				msg = fmt.Sprintf("Claude auth status confirmed (%s)", data.AuthMethod)
			}
			return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "ok", Confidence: confidenceVerified, Message: msg}
		}
		msg := "Claude auth not logged in"
		if strings.TrimSpace(data.AuthMethod) != "" && !strings.EqualFold(strings.TrimSpace(data.AuthMethod), "none") {
			msg = fmt.Sprintf("Claude auth not logged in (%s)", data.AuthMethod)
		}
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "skipped", Confidence: confidenceVerified, Message: msg}
	}
	if err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "error", Confidence: confidenceVerified, Message: trimCommandOutput(out, err)}
	}
	msg := firstNonEmptyLine(string(out))
	if msg == "" {
		msg = "Claude auth status command returned no parseable output"
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "error", Confidence: confidenceVerified, Message: msg}
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "ok", Confidence: confidenceVerified, Message: msg}
}

func probeOpenCodeSession(opts RefreshOptions, tool update.Tool) RefreshResult {
	if opts.DryRun {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "providers-list", Status: "skipped", Confidence: confidenceVerified, Message: "would run 'opencode providers list'"}
	}
	if _, err := refreshLookPath("opencode"); err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: "providers-list", Status: "unsupported", Message: "OpenCode CLI binary not installed (expected 'opencode')"}
	}
	cmd := refreshCommand("opencode", "providers", "list")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "providers-list", Status: "error", Confidence: confidenceVerified, Message: trimCommandOutput(out, err)}
	}
	credentialCount := parseCredentialCount(string(out))
	if credentialCount > 0 {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "providers-list", Status: "ok", Confidence: confidenceVerified, Message: fmt.Sprintf("OpenCode credential inventory detected (%d credential(s))", credentialCount)}
	}
	if strings.Contains(strings.ToLower(string(out)), "environment") {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "providers-list", Status: "skipped", Confidence: confidencePartial, Message: "OpenCode environment credential hints detected, but no stored credentials listed"}
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "providers-list", Status: "skipped", Confidence: confidenceVerified, Message: "OpenCode providers list reported no configured credentials"}
}

func probeCopilotSession(opts RefreshOptions, tool update.Tool) RefreshResult {
	if opts.DryRun {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "partial-auth", Status: "skipped", Confidence: confidencePartial, Message: "would inspect GitHub auth and local Copilot auth artifacts"}
	}
	if _, err := refreshLookPath("copilot"); err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: "partial-auth", Status: "unsupported", Message: "Copilot CLI binary not installed (expected 'copilot')"}
	}
	if copilotTokenEnvExists() {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "partial-auth", Status: "skipped", Confidence: confidencePartial, Message: "GitHub auth detected via token environment, but Copilot exposes no dedicated token-free status probe"}
	}
	if _, err := refreshLookPath("gh"); err == nil {
		cmd := refreshCommand("gh", "auth", "status")
		out, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(strings.ToLower(string(out)), "logged in to") {
			return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "partial-auth", Status: "skipped", Confidence: confidencePartial, Message: "GitHub auth detected, but Copilot exposes no dedicated token-free status probe"}
		}
	}
	if copilotAuthEvidenceExists() {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "partial-auth", Status: "skipped", Confidence: confidencePartial, Message: "Local Copilot auth evidence detected, but Copilot exposes no dedicated token-free status probe"}
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: "partial-auth", Status: "unsupported", Message: "No verified token-free Copilot status probe beyond partial auth evidence"}
}

func probeCodexSession(opts RefreshOptions, tool update.Tool) RefreshResult {
	if opts.DryRun {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "skipped", Confidence: confidenceVerified, Message: "would run 'codex login status'"}
	}
	if _, err := refreshLookPath("codex"); err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: false, Mode: "auth-status", Status: "unsupported", Message: "codex binary not installed"}
	}
	cmd := refreshCommand("codex", "login", "status")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "error", Confidence: confidenceVerified, Message: trimCommandOutput(out, err)}
	}
	msg := firstNonEmptyLine(string(out))
	if msg == "" {
		msg = "login status confirmed"
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "auth-status", Status: "ok", Confidence: confidenceVerified, Message: msg}
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
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-auth", Status: "skipped", Confidence: confidenceVerified, Message: msg, SourcePath: path}
	}
	if path == "" {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-auth", Status: "skipped", Confidence: confidenceVerified, Message: fmt.Sprintf("%s found, but no local Cursor auth.json detected", binary)}
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-auth", Status: "ok", Confidence: confidenceVerified, Message: fmt.Sprintf("Cursor auth.json detected via %s", binary), SourcePath: path}
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
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-session", Status: "skipped", Confidence: confidenceVerified, Message: msg, SourcePath: path}
	}
	if path == "" {
		return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-session", Status: "skipped", Confidence: confidenceVerified, Message: fmt.Sprintf("%s found, but no local Antigravity session artifacts detected", binary)}
	}
	return RefreshResult{Provider: tool.BinaryName, Supported: true, Mode: "local-session", Status: "ok", Confidence: confidenceVerified, Message: fmt.Sprintf("Local Antigravity session artifacts detected via %s", binary), SourcePath: path}
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

func parseCredentialCount(text string) int {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(strings.ToLower(line), "credential") {
			continue
		}
		fields := strings.Fields(line)
		for _, field := range fields {
			field = strings.TrimSpace(field)
			n, err := strconv.Atoi(field)
			if err == nil {
				return n
			}
		}
	}
	return 0
}

func copilotAuthEvidenceExists() bool {
	if copilotTokenEnvExists() {
		return true
	}
	home, _ := os.UserHomeDir()
	if strings.TrimSpace(home) == "" {
		return false
	}
	paths := []string{
		filepath.Join(home, ".copilot"),
		filepath.Join(home, ".config", "copilot"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func copilotTokenEnvExists() bool {
	return strings.TrimSpace(os.Getenv("COPILOT_GITHUB_TOKEN")) != "" || strings.TrimSpace(os.Getenv("GH_TOKEN")) != "" || strings.TrimSpace(os.Getenv("GITHUB_TOKEN")) != ""
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
