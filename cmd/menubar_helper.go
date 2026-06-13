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

func resolveMenubarHelperPath(env map[string]string, execPath string, workingDir string) (string, []string) {
	candidates := menubarHelperCandidates(env, execPath, workingDir)
	searched := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		searched = append(searched, cleaned)
		if info, err := os.Stat(cleaned); err == nil && !info.IsDir() {
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
