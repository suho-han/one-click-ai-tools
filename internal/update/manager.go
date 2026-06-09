package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Manager string

const (
	Npm         Manager = "npm"
	Brew        Manager = "brew"
	Pnpm        Manager = "pnpm"
	Yarn        Manager = "yarn"
	Cargo       Manager = "cargo"
	GoInstall   Manager = "go-install"
	Pip         Manager = "pip"
	CursorAgent Manager = "cursor-agent"
	Unknown     Manager = "unknown"
)

var (
	binaryLookup          = exec.LookPath
	commandOutput         = defaultCommandOutput
	errExecutableNotFound = exec.ErrNotFound
)

var noChangePatterns = map[Manager][]string{
	Npm:         {"up to date"},
	Pnpm:        {"already up to date"},
	Yarn:        {"already up to date"},
	Brew:        {"already installed", "up-to-date"},
	CursorAgent: {"already up to date", "already on the latest version", "latest version"},
	Cargo:       {"is already installed", "use --force to override"},
	Pip:         {"requirement already satisfied"},
}

func DetectManager(t Tool) Manager {
	if m, ok := managerFromPackagePrefix(t.Package); ok {
		return m
	}

	if isCursorTool(t) {
		return CursorAgent
	}

	if m, ok := detectManagerFromBinaryPath(t); ok {
		return m
	}

	if matchesInstalledPackage(Brew, t) {
		return Brew
	}
	if matchesInstalledPackage(Pnpm, t) {
		return Pnpm
	}
	if matchesInstalledPackage(Yarn, t) {
		return Yarn
	}
	if matchesInstalledPackage(Npm, t) {
		return Npm
	}

	return Unknown
}

func ResolveManagerForInstall(t Tool) Manager {
	if detected := DetectManager(t); detected != Unknown {
		return detected
	}
	if preferred, ok := defaultManagerForTool(t); ok {
		return preferred
	}
	return Unknown
}

func (m Manager) InstallCommand(t Tool) *exec.Cmd {
	return m.InstallCommandCtx(context.Background(), t)
}

func (m Manager) InstallCommandCtx(ctx context.Context, t Tool) *exec.Cmd {
	switch m {
	case CursorAgent:
		return exec.CommandContext(ctx, "bash", "-lc", "curl https://cursor.com/install -fsS | bash")
	case Brew:
		return exec.CommandContext(ctx, "brew", "upgrade", t.BrewTarget())
	case Pnpm:
		return exec.CommandContext(ctx, "pnpm", "add", "-g", t.Package)
	case Yarn:
		return exec.CommandContext(ctx, "yarn", "global", "add", t.Package)
	case Cargo:
		return exec.CommandContext(ctx, "cargo", "install", packageWithoutManagerPrefix(t.Package), "--locked")
	case GoInstall:
		return exec.CommandContext(ctx, "go", "install", packageWithoutManagerPrefix(t.Package)+"@latest")
	case Pip:
		return exec.CommandContext(ctx, "python3", "-m", "pip", "install", "--upgrade", packageWithoutManagerPrefix(t.Package))
	default:
		return exec.CommandContext(ctx, "npm", "install", "-g", t.Package)
	}
}

func (m Manager) GetInstalledVersion(t Tool) string {
	switch m {
	case CursorAgent:
		for _, binary := range preferredBinaries(t) {
			out, err := exec.Command(binary, "--version").Output()
			if err == nil {
				return strings.TrimSpace(string(out))
			}
		}
		return ""
	case Brew:
		out, _ := exec.Command("brew", "list", "--versions", t.BrewTarget()).Output()
		parts := strings.Fields(strings.TrimSpace(string(out)))
		if len(parts) >= 2 {
			return parts[1]
		}
		return ""
	case Pnpm:
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
		out, _ := exec.Command("yarn", "global", "list", "--pattern", t.Package).Output()
		return parseVersionFromAtSuffix(string(out), t.Package)
	case Cargo:
		out, _ := exec.Command("cargo", "install", "--list").Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), packageWithoutManagerPrefix(t.Package)+" ") {
				fields := strings.Fields(strings.TrimSuffix(strings.TrimSpace(line), ":"))
				if len(fields) >= 2 {
					return strings.TrimPrefix(fields[1], "v")
				}
			}
		}
		return ""
	case GoInstall:
		out, _ := exec.Command(t.BinaryName, "--version").Output()
		return strings.TrimSpace(string(out))
	case Pip:
		out, _ := exec.Command("python3", "-m", "pip", "show", packageWithoutManagerPrefix(t.Package)).Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(strings.ToLower(line), "version:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			}
		}
		return ""
	default:
		out, _ := exec.Command("npm", "list", "-g", t.Package, "--depth=0").Output()
		return parseVersionFromAtSuffix(string(out), t.Package)
	}
}

func defaultCommandOutput(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

func detectManagerFromBinaryPath(t Tool) (Manager, bool) {
	for _, binary := range preferredBinaries(t) {
		path, err := binaryLookup(binary)
		if err != nil || strings.TrimSpace(path) == "" {
			continue
		}
		if manager, ok := classifyManagerFromBinaryPath(path); ok {
			return manager, true
		}
	}
	return Unknown, false
}

func classifyManagerFromBinaryPath(path string) (Manager, bool) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return Unknown, false
	}

	checks := []struct {
		manager  Manager
		prefixes []string
		contains []string
	}{
		{manager: Brew, prefixes: commandPrefixes("brew", "--prefix", "bin"), contains: []string{filepath.Clean("/Cellar/")}},
		{manager: Pnpm, prefixes: commandPrefixes("pnpm", "bin", ""), contains: nil},
		{manager: Yarn, prefixes: commandPrefixes("yarn", "global", "bin"), contains: nil},
		{manager: Npm, prefixes: npmGlobalBinaryPrefixes(), contains: nil},
		{manager: Cargo, prefixes: cargoBinaryPrefixes(), contains: nil},
		{manager: GoInstall, prefixes: goInstallBinaryPrefixes(), contains: nil},
		{manager: Pip, prefixes: pipBinaryPrefixes(), contains: nil},
	}

	for _, check := range checks {
		for _, prefix := range check.prefixes {
			if hasPathPrefix(path, prefix) {
				return check.manager, true
			}
		}
		for _, marker := range check.contains {
			if marker != "" && strings.Contains(path, marker) {
				return check.manager, true
			}
		}
	}

	return Unknown, false
}

func matchesInstalledPackage(m Manager, t Tool) bool {
	if strings.TrimSpace(t.Package) == "" {
		return false
	}
	out, err := packageListCommand(m, t)
	return err == nil && strings.Contains(string(out), t.Package)
}

func packageListCommand(m Manager, t Tool) ([]byte, error) {
	switch m {
	case Brew:
		return commandOutput("brew", "list", t.BrewTarget())
	case Pnpm:
		return commandOutput("pnpm", "list", "-g", t.Package)
	case Yarn:
		return commandOutput("yarn", "global", "list", t.Package)
	case Npm:
		return commandOutput("npm", "list", "-g", t.Package)
	default:
		return nil, errExecutableNotFound
	}
}

func defaultManagerForTool(t Tool) (Manager, bool) {
	if m, ok := managerFromPackagePrefix(t.Package); ok {
		return m, true
	}
	if isCursorTool(t) {
		return CursorAgent, true
	}
	if strings.TrimSpace(t.Package) != "" {
		return Npm, true
	}
	if strings.TrimSpace(t.BrewPackage) != "" {
		return Brew, true
	}
	return Unknown, false
}

func commandPrefixes(name string, firstArg string, mode string) []string {
	var args []string
	switch {
	case name == "brew" && firstArg == "--prefix":
		args = []string{"--prefix"}
	case name == "pnpm" && firstArg == "bin":
		args = []string{"bin", "-g"}
	case name == "yarn" && firstArg == "global":
		args = []string{"global", "bin"}
	default:
		args = []string{firstArg}
	}
	out, err := commandOutput(name, args...)
	if err != nil {
		return nil
	}
	prefix := strings.TrimSpace(string(out))
	if prefix == "" {
		return nil
	}
	prefix = filepath.Clean(prefix)
	if mode == "bin" {
		return []string{prefix}
	}
	return []string{prefix, filepath.Join(prefix, "bin")}
}

func npmGlobalBinaryPrefixes() []string {
	out, err := commandOutput("npm", "prefix", "-g")
	if err != nil {
		return nil
	}
	prefix := strings.TrimSpace(string(out))
	if prefix == "" {
		return nil
	}
	prefix = filepath.Clean(prefix)
	return []string{filepath.Join(prefix, "bin"), prefix}
}

func cargoBinaryPrefixes() []string {
	if home := strings.TrimSpace(os.Getenv("CARGO_HOME")); home != "" {
		return []string{filepath.Join(filepath.Clean(home), "bin")}
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return []string{filepath.Join(filepath.Clean(home), ".cargo", "bin")}
	}
	return nil
}

func goInstallBinaryPrefixes() []string {
	out, err := commandOutput("go", "env", "GOPATH")
	if err != nil {
		return nil
	}
	prefix := strings.TrimSpace(string(out))
	if prefix == "" {
		return nil
	}
	return []string{filepath.Join(filepath.Clean(prefix), "bin")}
}

func pipBinaryPrefixes() []string {
	out, err := commandOutput("python3", "-m", "site", "--user-base")
	if err != nil {
		return nil
	}
	base := strings.TrimSpace(string(out))
	if base == "" {
		return nil
	}
	return []string{filepath.Join(filepath.Clean(base), "bin"), filepath.Join(filepath.Clean(base), "Scripts")}
}

func hasPathPrefix(path, prefix string) bool {
	path = filepath.Clean(strings.TrimSpace(path))
	prefix = filepath.Clean(strings.TrimSpace(prefix))
	if path == "" || prefix == "" || path == "." || prefix == "." {
		return false
	}
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+string(os.PathSeparator))
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

func managerFromPackagePrefix(pkg string) (Manager, bool) {
	pkg = strings.TrimSpace(strings.ToLower(pkg))
	switch {
	case strings.HasPrefix(pkg, "cargo:"):
		return Cargo, true
	case strings.HasPrefix(pkg, "go:"):
		return GoInstall, true
	case strings.HasPrefix(pkg, "pip:"):
		return Pip, true
	default:
		return Unknown, false
	}
}

func packageWithoutManagerPrefix(pkg string) string {
	parts := strings.SplitN(pkg, ":", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[1])
	}
	return pkg
}

func isCursorTool(t Tool) bool {
	return t.MatchesName("cursor-agent") || t.MatchesName("cursor") || t.MatchesName("agent")
}

func preferredBinaries(t Tool) []string {
	seen := map[string]bool{}
	candidates := make([]string, 0, 1+len(t.BinaryAliases))
	for _, candidate := range append([]string{t.BinaryName}, t.BinaryAliases...) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	return candidates
}
