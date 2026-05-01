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

func DetectManager(t Tool) Manager {
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
