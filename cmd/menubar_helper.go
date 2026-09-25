package cmd

import (
	"bytes"
	"context"
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
			// Release-config builds (used by the release workflow) land in
			// .build/release; checked after debug so dev worktrees keep
			// preferring the freshly-built debug helper.
			appendCandidate(filepath.Join(cursor, "macos", "OctMenubar", ".build", "release", "OctMenubarApp"))
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

	home := strings.TrimSpace(env["HOME"])
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = strings.TrimSpace(h)
		}
	}
	if home != "" {
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
	// Building the SwiftUI helper needs the macro plugins that full Xcode
	// toolchains ship; the standalone CLT swift often lacks them (fails with
	// "SwiftUIMacros ... not found"), so prefer discovered Xcode toolchains
	// over PATH.
	if devDir := strings.TrimSpace(env["DEVELOPER_DIR"]); devDir != "" {
		appendCandidate(filepath.Join(devDir, "usr", "bin", "swift"))
	}
	for _, candidate := range xcodeToolchainSwiftCandidates(env) {
		appendCandidate(candidate)
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

// xcodeAppSearchRoots lists the directories scanned for Xcode.app bundles.
// Package-level seam: tests override it so candidate order is isolated from
// whatever Xcode installs the host happens to have.
var xcodeAppSearchRoots = func(env map[string]string) []string {
	roots := []string{"/Applications"}
	if home := strings.TrimSpace(env["HOME"]); home != "" {
		roots = append(roots, filepath.Join(home, "Applications"), filepath.Join(home, "Downloads"))
	}
	return roots
}

// xcodeToolchainSwiftCandidates finds swift front-ends inside installed
// Xcode.app bundles: standard Applications locations plus the user's
// Downloads folder, where beta releases commonly sit. Two layouts are probed
// (Xcode ≤15 puts swift directly under Developer/usr/bin; newer ones nest it
// in Toolchains/XcodeDefault.xctoolchain).
func xcodeToolchainSwiftCandidates(env map[string]string) []string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	roots := xcodeAppSearchRoots(env)

	var candidates []string
	for _, root := range roots {
		// Enumeration works where TCC allows directory reads.
		for _, pattern := range []string{
			filepath.Join(root, "Xcode*.app", "Contents", "Developer", "usr", "bin", "swift"),
			filepath.Join(root, "Xcode*.app", "Contents", "Developer", "Toolchains", "*.xctoolchain", "usr", "bin", "swift"),
		} {
			if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
				candidates = append(candidates, matches...)
			}
		}
	}
	// ~/Downloads denies directory enumeration behind TCC, but a fully
	// specified path can still be probed — try well-known bundle names.
	for _, root := range roots {
		for _, name := range []string{"Xcode.app", "Xcode-beta.app"} {
			dev := filepath.Join(root, name, "Contents", "Developer")
			candidates = append(candidates,
				filepath.Join(dev, "usr", "bin", "swift"),
				filepath.Join(dev, "Toolchains", "XcodeDefault.xctoolchain", "usr", "bin", "swift"),
			)
		}
	}
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
		if strings.Contains(field, "one-click-tools") {
			return true
		}
	}
	return false
}

func buildMenubarHelper(ctx context.Context, projectDir string, stdout, stderr io.Writer) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("menubar helper build is supported only on macOS")
	}
	env := map[string]string{}
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok {
			env[key] = value
		}
	}
	swiftPath, searched := resolveSwiftExecutablePath(env)
	if swiftPath == "" {
		return fmt.Errorf("swift not found (searched: %s)", strings.Join(searched, ", "))
	}
	cmd := exec.CommandContext(ctx, swiftPath, "build")
	cmd.Dir = projectDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// A toolchain swift invoked directly still picks its SDK via
	// xcode-select (often the CLT SDK, whose SwiftUI lacks the macro
	// plugins). Point DEVELOPER_DIR at the discovered Xcode so the driver
	// uses that toolchain's SDK too.
	if developerDir := xcodeDeveloperDirForSwift(swiftPath); developerDir != "" {
		cmd.Env = append(os.Environ(), "DEVELOPER_DIR="+developerDir)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("swift build failed (%s): %w", swiftPath, err)
	}
	return nil
}

func xcodeDeveloperDirForSwift(swiftPath string) string {
	normalized := filepath.Clean(swiftPath)
	if idx := strings.Index(normalized, "/Contents/Developer/"); idx >= 0 {
		return normalized[:idx+len("/Contents/Developer")]
	}
	return ""
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

// menubarHelperVersionMarker is the literal BuildVersion.swift embeds in the
// helper binary; doctor scans for it instead of executing the helper.
const menubarHelperVersionMarker = "oct-menubar-helper-version="

// probeMenubarHelperVersion extracts the helper's stamped build version by
// scanning the binary for menubarHelperVersionMarker. It deliberately never
// executes the helper: running an older helper with --version would launch
// its status item (pre-version helpers do not parse arguments) and hang
// until a probe timeout, flashing a menubar icon at the user. Returns ""
// when the file is unreadable or predates version stamping.
// Package var so tests can stub the scan.
var probeMenubarHelperVersion = func(helperPath string) string {
	data, err := os.ReadFile(helperPath)
	if err != nil {
		return ""
	}
	idx := bytes.Index(data, []byte(menubarHelperVersionMarker))
	if idx < 0 {
		return ""
	}
	start := idx + len(menubarHelperVersionMarker)
	end := start
	for end < len(data) && data[end] >= 0x21 && data[end] <= 0x7e { // printable ASCII run
		end++
	}
	version := string(data[start:end])
	if version == "" {
		return ""
	}
	return strings.TrimPrefix(version, "v")
}

func copyExecutableFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Write to a temp file and rename: replacing the destination in place
	// (O_TRUNC) can crash a helper process that is already running from it,
	// while rename leaves the old inode intact for the running process.
	tmpDst := dst + ".new"
	out, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmpDst)
		return err
	}
	if err := out.Chmod(0o755); err != nil {
		out.Close()
		os.Remove(tmpDst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpDst)
		return err
	}
	return os.Rename(tmpDst, dst)
}
