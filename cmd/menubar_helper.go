package cmd

import (
	"os"
	"path/filepath"
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

	return candidates
}
