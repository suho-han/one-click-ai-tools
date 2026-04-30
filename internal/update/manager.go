package update

import (
	"os/exec"
	"strings"
)

type Manager string

const (
	Npm   Manager = "npm"
	Brew  Manager = "brew"
	Pnpm  Manager = "pnpm"
	Yarn  Manager = "yarn"
	Unknown Manager = "unknown"
)

func DetectManager(pkg string, binName string) Manager {
	// 1. Check brew
	if out, err := exec.Command("brew", "list", binName).CombinedOutput(); err == nil && len(out) > 0 {
		return Brew
	}

	// 2. Check pnpm
	if out, err := exec.Command("pnpm", "list", "-g", pkg).CombinedOutput(); err == nil && !strings.Contains(string(out), "empty") {
		return Pnpm
	}

	// 3. Check yarn
	if out, err := exec.Command("yarn", "global", "list", pkg).CombinedOutput(); err == nil && strings.Contains(string(out), pkg) {
		return Yarn
	}

	// 4. Default to npm
	return Npm
}

func (m Manager) InstallCommand(pkg string) *exec.Cmd {
	switch m {
	case Brew:
		return exec.Command("brew", "upgrade", pkg) // Assuming formula name matches pkg or handled elsewhere
	case Pnpm:
		return exec.Command("pnpm", "add", "-g", pkg)
	case Yarn:
		return exec.Command("yarn", "global", "add", pkg)
	default:
		return exec.Command("npm", "install", "-g", pkg)
	}
}
