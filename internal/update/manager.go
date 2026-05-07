package update

import (
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
	Unknown     Manager = "unknown"
)

func DetectManager(t Tool) Manager {
	if t.BinaryName == "cursor-agent" {
		return CursorAgent
	}

	// 1. Check brew
	brewTarget := t.BinaryName
	if t.BrewPackage != "" {
		brewTarget = t.BrewPackage
	}
	if out, err := exec.Command("brew", "list", brewTarget).CombinedOutput(); err == nil && len(out) > 0 {
		return Brew
	}

	// 2. Check pnpm
	if out, err := exec.Command("pnpm", "list", "-g", t.Package).CombinedOutput(); err == nil && strings.Contains(string(out), t.Package) {
		return Pnpm
	}

	// 3. Check yarn
	if out, err := exec.Command("yarn", "global", "list", t.Package).CombinedOutput(); err == nil && strings.Contains(string(out), t.Package) {
		return Yarn
	}

	// 4. Check npm
	if out, err := exec.Command("npm", "list", "-g", t.Package).CombinedOutput(); err == nil && strings.Contains(string(out), t.Package) {
		return Npm
	}

	// 5. Default to npm
	return Npm
}

func (m Manager) InstallCommand(t Tool) *exec.Cmd {
	switch m {
	case CursorAgent:
		return exec.Command(t.BinaryName, "update")
	case Brew:
		brewTarget := t.BinaryName
		if t.BrewPackage != "" {
			brewTarget = t.BrewPackage
		}
		return exec.Command("brew", "upgrade", brewTarget)
	case Pnpm:
		return exec.Command("pnpm", "add", "-g", t.Package)
	case Yarn:
		return exec.Command("yarn", "global", "add", t.Package)
	default:
		return exec.Command("npm", "install", "-g", t.Package)
	}
}

func (m Manager) GetInstalledVersion(t Tool) string {
	switch m {
	case CursorAgent:
		out, _ := exec.Command(t.BinaryName, "--version").Output()
		return strings.TrimSpace(string(out))
	case Brew:
		brewTarget := t.BinaryName
		if t.BrewPackage != "" {
			brewTarget = t.BrewPackage
		}
		// Ignore exit code: brew list exits non-zero when package is absent
		out, _ := exec.Command("brew", "list", "--versions", brewTarget).Output()
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
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, t.Package+"@") {
				idx := strings.LastIndex(line, "@")
				if idx >= 0 {
					return strings.TrimSpace(line[idx+1:])
				}
			}
		}
		return ""
	default: // npm
		// Ignore exit code: npm list exits non-zero on peer-dep issues even when package is present
		out, _ := exec.Command("npm", "list", "-g", t.Package, "--depth=0").Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, t.Package+"@") {
				idx := strings.LastIndex(line, "@")
				if idx >= 0 {
					return strings.TrimSpace(line[idx+1:])
				}
			}
		}
		return ""
	}
}

// IsNoChangeOutput returns true when the install command's output indicates
// the package was already at the latest version (used as a fallback when
// version detection is unavailable, and to handle brew which exits non-zero
// when nothing to upgrade).
func (m Manager) IsNoChangeOutput(output string) bool {
	out := strings.ToLower(output)
	switch m {
	case CursorAgent:
		return strings.Contains(out, "already up to date") ||
			strings.Contains(out, "already on the latest version") ||
			strings.Contains(out, "latest version")
	case Npm:
		return strings.Contains(out, "up to date")
	case Pnpm:
		return strings.Contains(out, "already up to date")
	case Yarn:
		return strings.Contains(out, "already up to date")
	case Brew:
		return strings.Contains(out, "already installed") || strings.Contains(out, "up-to-date")
	}
	return false
}
