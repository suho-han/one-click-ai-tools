package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type menubarHelperLaunch struct {
	Executable string
	Args       []string
	ProjectDir string
	Mode       string
}

type menubarStopResult struct {
	Stopped int
	PIDs    []string
}

func resolveMenubarHelperLaunch(env map[string]string, execPath string, workingDir string) (menubarHelperLaunch, []string) {
	helperPath, searched := resolveMenubarHelperPath(env, execPath, workingDir)
	if helperPath != "" {
		return menubarHelperLaunch{Executable: helperPath, Mode: "swift-helper"}, searched
	}

	projectDir, projectSearched, err := resolveMenubarProjectDir(execPath, workingDir)
	searched = append(searched, projectSearched...)
	if err != nil {
		return menubarHelperLaunch{}, searched
	}
	swiftPath, swiftSearched := resolveSwiftExecutablePath(env)
	searched = append(searched, swiftSearched...)
	if swiftPath == "" {
		return menubarHelperLaunch{}, searched
	}
	return menubarHelperLaunch{
		Executable: swiftPath,
		Args:       menubarSwiftRunArgs(projectDir),
		ProjectDir: projectDir,
		Mode:       "swift-package",
	}, searched
}

func resolveMenubarHelperPath(env map[string]string, execPath string, workingDir string) (string, []string) {
	candidates := menubarHelperCandidates(env, execPath, workingDir)
	searched := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		searched = append(searched, cleaned)
		if info, err := os.Stat(cleaned); err == nil && !info.IsDir() {
			if runtime.GOOS == "windows" {
				return cleaned, searched
			}
			mode := info.Mode()
			if mode&0o111 != 0 {
				return cleaned, searched
			}
		}
	}
	return "", searched
}

func menubarHelperCandidates(env map[string]string, execPath string, workingDir string) []string {
	var candidates []string
	seen := map[string]struct{}{}
	appendCandidate := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	if explicit := strings.TrimSpace(env["OCT_MENUBAR_HELPER_PATH"]); explicit != "" {
		appendCandidate(explicit)
	}

	baseDirs := []string{}
	if workingDir = strings.TrimSpace(workingDir); workingDir != "" {
		baseDirs = append(baseDirs, workingDir)
	}
	if execPath = strings.TrimSpace(execPath); execPath != "" {
		baseDirs = append(baseDirs, filepath.Dir(execPath))
	}

	for _, base := range baseDirs {
		cursor := filepath.Clean(base)
		for i := 0; i < 6; i++ {
			appendCandidate(filepath.Join(cursor, "OctMenubarApp"))
			appendCandidate(filepath.Join(cursor, "macos", "OctMenubar", ".build", "debug", "OctMenubarApp"))
			parent := filepath.Dir(cursor)
			if parent == cursor {
				break
			}
			cursor = parent
		}
	}

	if rawPath := strings.TrimSpace(env["PATH"]); rawPath != "" {
		for _, dir := range filepath.SplitList(rawPath) {
			if strings.TrimSpace(dir) == "" {
				continue
			}
			appendCandidate(filepath.Join(dir, "OctMenubarApp"))
		}
	}

	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		appendCandidate(filepath.Join(home, ".local", "bin", "OctMenubarApp"))
	}

	return candidates
}

func resolveSwiftExecutablePath(env map[string]string) (string, []string) {
	candidates := swiftExecutableCandidates(env)
	searched := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		searched = append(searched, cleaned)
		if info, err := os.Stat(cleaned); err == nil && !info.IsDir() {
			if runtime.GOOS == "windows" || info.Mode()&0o111 != 0 {
				return cleaned, searched
			}
		}
	}
	return "", searched
}

func swiftExecutableCandidates(env map[string]string) []string {
	var candidates []string
	seen := map[string]struct{}{}
	appendCandidate := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		candidates = append(candidates, cleaned)
	}

	if explicit := strings.TrimSpace(env["OCT_MENUBAR_SWIFT_PATH"]); explicit != "" {
		appendCandidate(explicit)
	}
	if rawPath := strings.TrimSpace(env["PATH"]); rawPath != "" {
		for _, dir := range filepath.SplitList(rawPath) {
			if strings.TrimSpace(dir) == "" {
				continue
			}
			appendCandidate(filepath.Join(dir, "swift"))
		}
	}
	appendCandidate("/usr/bin/swift")
	return candidates
}

func menubarSwiftRunArgs(projectDir string) []string {
	args := []string{"run", "--package-path", projectDir}
	if cacheDir, err := os.UserCacheDir(); err == nil && strings.TrimSpace(cacheDir) != "" {
		args = append(args, "--scratch-path", filepath.Join(cacheDir, "one-click-tools", "OctMenubar"))
	}
	return append(args, "OctMenubarApp")
}

func isMenubarStopTarget(pid int, currentPID int, command string) bool {
	if pid == 0 || pid == currentPID {
		return false
	}
	command = strings.TrimSpace(command)
	if command == "" || strings.Contains(command, " menubar stop") {
		return false
	}

	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}

	executableName := filepath.Base(fields[0])
	if executableName == "OctMenubarApp" {
		return true
	}
	if strings.Contains(fields[0], "/OctMenubarApp") {
		return true
	}
	if executableName == "swift" && len(fields) >= 2 && fields[1] == "run" && fields[len(fields)-1] == "OctMenubarApp" {
		return true
	}
	if !containsMenubarArgument(fields) {
		return false
	}
	return commandLooksLikeOctProcess(fields)
}

func containsMenubarArgument(fields []string) bool {
	for _, field := range fields {
		if field == "menubar" {
			return true
		}
	}
	return false
}

func commandLooksLikeOctProcess(fields []string) bool {
	for _, field := range fields {
		base := filepath.Base(field)
		switch base {
		case "oct", "one-click-tools", "main.go":
			return true
		}
		if strings.Contains(field, "one-click-tools") || strings.Contains(field, "oct-wrapper.js") {
			return true
		}
	}
	return false
}

func buildMenubarHelper(projectDir string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("menubar helper build is supported only on macOS")
	}
	cmd := exec.Command("swift", "build")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func installMenubarHelper(projectDir string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("menubar helper install is supported only on macOS")
	}
	src := filepath.Join(projectDir, ".build", "debug", "OctMenubarApp")
	if info, err := os.Stat(src); err != nil || info.IsDir() {
		return "", fmt.Errorf("built helper not found at %s (run 'oct menubar build-helper' first)", src)
	}
	dst, err := defaultMenubarInstallPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	if err := copyExecutableFile(src, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func copyExecutableFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(0o755)
}
