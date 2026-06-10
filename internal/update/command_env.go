package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func commandEnv() []string {
	return withPathEnv(os.Environ(), bootstrapPATH(os.Getenv("PATH")))
}

func withPathEnv(env []string, pathValue string) []string {
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "PATH=") {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "PATH="+pathValue)
}

func bootstrapPATH(base string) string {
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

func commandWithEnv(name string, args ...string) *exec.Cmd {
	resolved := resolveExecutable(name)
	cmd := exec.Command(resolved, args...)
	cmd.Env = commandEnv()
	return cmd
}

func commandContextWithEnv(ctx context.Context, name string, args ...string) *exec.Cmd {
	resolved := resolveExecutable(name)
	cmd := exec.CommandContext(ctx, resolved, args...)
	cmd.Env = commandEnv()
	return cmd
}

func resolveExecutable(name string) string {
	if strings.ContainsRune(name, os.PathSeparator) {
		return name
	}
	if resolved, err := lookPathWithBootstrap(name); err == nil && strings.TrimSpace(resolved) != "" {
		return resolved
	}
	return name
}

func lookPathWithBootstrap(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	pathValue := bootstrapPATH(os.Getenv("PATH"))
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
