package execenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func Environ() []string {
	return WithPathEnv(os.Environ(), BuildPATH(os.Getenv("PATH")))
}

func WithPathEnv(env []string, pathValue string) []string {
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "PATH=") {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "PATH="+pathValue)
}

func BuildPATH(base string) string {
	sep := string(os.PathListSeparator)
	seen := map[string]bool{}
	parts := make([]string, 0, 16)

	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if path == "." || seen[path] {
			return
		}
		seen[path] = true
		parts = append(parts, path)
	}

	for _, part := range strings.Split(base, sep) {
		add(part)
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		home = filepath.Clean(home)
		add(filepath.Join(home, ".local", "bin"))
		add(filepath.Join(home, ".npm-global", "bin"))
		add(filepath.Join(home, ".opencode", "bin"))
		add(filepath.Join(home, ".cargo", "bin"))
		add(filepath.Join(home, "go", "bin"))
		if runtime.GOOS == "darwin" {
			add(filepath.Join(home, "Library", "pnpm"))
		}
	}

	if runtime.GOOS == "darwin" {
		add("/opt/homebrew/bin")
		add("/usr/local/bin")
	}

	return strings.Join(parts, sep)
}

func Command(name string, args ...string) *exec.Cmd {
	resolved := ResolveExecutable(name)
	cmd := exec.Command(resolved, args...)
	cmd.Env = Environ()
	return cmd
}

func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	resolved := ResolveExecutable(name)
	cmd := exec.CommandContext(ctx, resolved, args...)
	cmd.Env = Environ()
	return cmd
}

func ResolveExecutable(name string) string {
	if strings.ContainsRune(name, os.PathSeparator) {
		return name
	}
	if resolved, err := LookPath(name); err == nil && strings.TrimSpace(resolved) != "" {
		return resolved
	}
	return name
}

func LookPath(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	pathValue := BuildPATH(os.Getenv("PATH"))
	for _, dir := range strings.Split(pathValue, string(os.PathListSeparator)) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", exec.ErrNotFound
}
