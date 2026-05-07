package update

import (
	"context"
	"os/exec"
	"strings"
)

type Manager string

const (
	Npm         Manager = "npm"
	Brew        Manager = "brew"
	Pnpm        Manager = "pnpm"
	Yarn        Manager = "yarn"
	CursorAgent Manager = "cursor-agent"
)

var noChangePatterns = map[Manager][]string{
	Npm:         {"up to date"},
	Pnpm:        {"already up to date"},
	Yarn:        {"already up to date"},
	Brew:        {"already installed", "up-to-date"},
	CursorAgent: {"already up to date", "already on the latest version", "latest version"},
}

func DetectManager(t Tool) Manager {
	if t.BinaryName == "cursor-agent" {
		return CursorAgent
	}

	if out, err := exec.Command("brew", "list", t.BrewTarget()).CombinedOutput(); err == nil && len(out) > 0 {
		return Brew
	}

	if out, err := exec.Command("pnpm", "list", "-g", t.Package).CombinedOutput(); err == nil && strings.Contains(string(out), t.Package) {
		return Pnpm
	}

	if out, err := exec.Command("yarn", "global", "list", t.Package).CombinedOutput(); err == nil && strings.Contains(string(out), t.Package) {
		return Yarn
	}

	if out, err := exec.Command("npm", "list", "-g", t.Package).CombinedOutput(); err == nil && strings.Contains(string(out), t.Package) {
		return Npm
	}

	return Npm
}

func (m Manager) InstallCommand(t Tool) *exec.Cmd {
	return m.InstallCommandCtx(context.Background(), t)
}

func (m Manager) InstallCommandCtx(ctx context.Context, t Tool) *exec.Cmd {
	switch m {
	case CursorAgent:
		return exec.Command(t.BinaryName, "update")
	case Brew:
		return exec.CommandContext(ctx, "brew", "upgrade", t.BrewTarget())
	case Pnpm:
		return exec.CommandContext(ctx, "pnpm", "add", "-g", t.Package)
	case Yarn:
		return exec.CommandContext(ctx, "yarn", "global", "add", t.Package)
	default:
		return exec.CommandContext(ctx, "npm", "install", "-g", t.Package)
	}
}

func (m Manager) GetInstalledVersion(t Tool) string {
	switch m {
	case CursorAgent:
		out, _ := exec.Command(t.BinaryName, "--version").Output()
		return strings.TrimSpace(string(out))
	case Brew:
		// Ignore exit code: brew list exits non-zero when package is absent
		out, _ := exec.Command("brew", "list", "--versions", t.BrewTarget()).Output()
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) >= 2 {
			return parts[1]
		}
		return ""
	case Pnpm:
		// Ignore exit code: pnpm list may exit non-zero due to unrelated warnings
		out, _ := exec.Command("pnpm", "list", "-g", t.Package, "--depth=0").Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, t.Package) {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[len(parts)-1]
				}
			}
		}
		return ""
	case Yarn:
		// Ignore exit code: yarn list may exit non-zero for missing packages
		out, _ := exec.Command("yarn", "global", "list", "--pattern", t.Package).Output()
		return parseVersionFromAtSuffix(string(out), t.Package)
	default: // npm
		// Ignore exit code: npm list exits non-zero on peer-dep issues even when package is present
		out, _ := exec.Command("npm", "list", "-g", t.Package, "--depth=0").Output()
		return parseVersionFromAtSuffix(string(out), t.Package)
	}
}

func parseVersionFromAtSuffix(output, pkg string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, pkg+"@") {
			if idx := strings.LastIndex(line, "@"); idx >= 0 {
				return strings.TrimSpace(line[idx+1:])
			}
		}
	}
	return ""
}

// IsNoChangeOutput returns true when the install command's output indicates
// the package was already at the latest version (used as a fallback when
// version detection is unavailable, and to handle brew which exits non-zero
// when nothing to upgrade).
func (m Manager) IsNoChangeOutput(output string) bool {
	patterns, ok := noChangePatterns[m]
	if !ok {
		return false
	}
	out := strings.ToLower(output)
	for _, p := range patterns {
		if strings.Contains(out, p) {
			return true
		}
	}
	return false
}
