package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMenubarHelperPathPrefersExplicitOverride(t *testing.T) {
	temp := t.TempDir()
	helper := filepath.Join(temp, "custom", "OctMenubarApp")
	if err := os.MkdirAll(filepath.Dir(helper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	resolved, searched := resolveMenubarHelperPath(
		map[string]string{"OCT_MENUBAR_HELPER_PATH": helper},
		filepath.Join(temp, "oct"),
		temp,
	)
	if resolved != helper {
		t.Fatalf("resolved helper = %q, want %q", resolved, helper)
	}
	if len(searched) == 0 || searched[0] != helper {
		t.Fatalf("searched[0] = %v, want explicit helper first", searched)
	}
}

func TestResolveMenubarHelperPathFindsRepoBuildFromWorkingDir(t *testing.T) {
	temp := t.TempDir()
	repo := filepath.Join(temp, "repo")
	workingDir := filepath.Join(repo, "subdir")
	helper := filepath.Join(repo, "macos", "OctMenubar", ".build", "debug", "OctMenubarApp")
	if err := os.MkdirAll(filepath.Dir(helper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	resolved, searched := resolveMenubarHelperPath(nil, filepath.Join(repo, "oct"), workingDir)
	if resolved != helper {
		t.Fatalf("resolved helper = %q, want %q", resolved, helper)
	}
	found := false
	for _, candidate := range searched {
		if candidate == helper {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("searched paths missing helper: %v", searched)
	}
}

func TestResolveMenubarHelperLaunchRunsSwiftPackageWhenHelperBinaryIsMissing(t *testing.T) {
	temp := t.TempDir()
	repo := filepath.Join(temp, "repo")
	workingDir := filepath.Join(repo, "subdir")
	project := filepath.Join(repo, "macos", "OctMenubar")
	swift := filepath.Join(temp, "bin", "swift")
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(swift), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "Package.swift"), []byte("// swift-tools-version: 6.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(swift, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	isolateXcodeSearch(t)

	launch, searched := resolveMenubarHelperLaunch(
		map[string]string{"PATH": filepath.Dir(swift), "HOME": temp},
		filepath.Join(repo, "oct"),
		workingDir,
	)
	if launch.Mode != "swift-package" {
		t.Fatalf("launch mode = %q, want swift-package (searched %v)", launch.Mode, searched)
	}
	if launch.Executable != swift {
		t.Fatalf("launch executable = %q, want %q", launch.Executable, swift)
	}
	if len(launch.Args) < 4 {
		t.Fatalf("launch args = %v, want swift run package args", launch.Args)
	}
	if launch.Args[0] != "run" || launch.Args[1] != "--package-path" || launch.Args[2] != project {
		t.Fatalf("launch args = %v, want swift run package args for %s", launch.Args, project)
	}
	if launch.Args[len(launch.Args)-1] != "OctMenubarApp" {
		t.Fatalf("launch args = %v, want OctMenubarApp executable", launch.Args)
	}
}

func TestResolveMenubarHelperLaunchPrefersBuiltHelperOverSwiftPackage(t *testing.T) {
	temp := t.TempDir()
	repo := filepath.Join(temp, "repo")
	workingDir := filepath.Join(repo, "subdir")
	helper := filepath.Join(repo, "macos", "OctMenubar", ".build", "debug", "OctMenubarApp")
	swift := filepath.Join(temp, "bin", "swift")
	if err := os.MkdirAll(filepath.Dir(helper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(swift), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "macos", "OctMenubar", "Package.swift"), []byte("// swift-tools-version: 6.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(swift, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	launch, _ := resolveMenubarHelperLaunch(
		map[string]string{"PATH": filepath.Dir(swift)},
		filepath.Join(repo, "oct"),
		workingDir,
	)
	if launch.Mode != "swift-helper" {
		t.Fatalf("launch mode = %q, want swift-helper", launch.Mode)
	}
	if launch.Executable != helper {
		t.Fatalf("launch executable = %q, want %q", launch.Executable, helper)
	}
	if len(launch.Args) != 0 {
		t.Fatalf("launch args = %v, want none", launch.Args)
	}
}

func TestIsMenubarStopTarget(t *testing.T) {
	currentPID := 42
	tests := []struct {
		name    string
		pid     int
		command string
		want    bool
	}{
		{
			name:    "Swift helper executable",
			pid:     100,
			command: "/Users/me/.local/bin/OctMenubarApp",
			want:    true,
		},
		{
			name:    "Swift package fallback",
			pid:     101,
			command: "/usr/bin/swift run --package-path /repo/macos/OctMenubar --scratch-path /tmp/oct OctMenubarApp",
			want:    true,
		},
		{
			name:    "Legacy oct menubar",
			pid:     102,
			command: "/Users/me/bin/oct menubar --legacy",
			want:    true,
		},
		{
			name:    "Current stop command",
			pid:     currentPID,
			command: "/Users/me/bin/oct menubar stop",
			want:    false,
		},
		{
			name:    "Other stop command process",
			pid:     104,
			command: "/Users/me/bin/oct menubar stop",
			want:    false,
		},
		{
			name:    "Swift build is not a running helper",
			pid:     105,
			command: "/usr/bin/swift build --package-path /repo/macos/OctMenubar",
			want:    false,
		},
		{
			name:    "Unrelated command mentioning menubar docs",
			pid:     106,
			command: "vim CONTEXT/menubar_helper_operations.md",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMenubarStopTarget(tt.pid, currentPID, tt.command)
			if got != tt.want {
				t.Fatalf("isMenubarStopTarget(%d, %d, %q) = %v, want %v", tt.pid, currentPID, tt.command, got, tt.want)
			}
		})
	}
}

func makeFakeSwift(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// isolateXcodeSearch pins the Xcode scan to the given roots so candidate
// order does not depend on whatever Xcode the host has in /Applications —
// CI's macOS runners ship a real Xcode there, which otherwise shadows the
// fixtures these tests build.
func isolateXcodeSearch(t *testing.T, roots ...string) {
	t.Helper()
	orig := xcodeAppSearchRoots
	xcodeAppSearchRoots = func(env map[string]string) []string { return roots }
	t.Cleanup(func() { xcodeAppSearchRoots = orig })
}

// TestSwiftExecutableCandidatesPrefersXcodeOverPath pins the fix for hosts
// whose PATH swift is the standalone CLT (no SwiftUI macro plugins): a full
// Xcode toolchain — DEVELOPER_DIR, /Applications, ~/Downloads, ~/Applications
// — must win over PATH.
func TestSwiftExecutableCandidatesPrefersXcodeOverPath(t *testing.T) {
	temp := t.TempDir()
	devDirSwift := filepath.Join(temp, "Developer", "usr", "bin", "swift")
	makeFakeSwift(t, devDirSwift)
	downloadsSwift := filepath.Join(temp, "Downloads", "Xcode-beta.app", "Contents", "Developer", "usr", "bin", "swift")
	makeFakeSwift(t, downloadsSwift)
	isolateXcodeSearch(t, filepath.Join(temp, "Downloads"), filepath.Join(temp, "Applications"))

	candidates := swiftExecutableCandidates(map[string]string{
		"HOME":          temp,
		"DEVELOPER_DIR": filepath.Join(temp, "Developer"),
		"PATH":          "/usr/bin:/bin:/usr/local/bin",
	})

	if candidates[0] != devDirSwift {
		t.Fatalf("candidates[0] = %q, want DEVELOPER_DIR swift %q", candidates[0], devDirSwift)
	}
	if candidates[1] != downloadsSwift {
		t.Fatalf("candidates[1] = %q, want discovered Downloads Xcode swift %q", candidates[1], downloadsSwift)
	}
	for _, c := range candidates {
		if c == "/usr/bin/swift" {
			return // PATH candidates still present as fallback
		}
	}
	t.Fatalf("PATH swift missing from candidates: %v", candidates)
}

func TestSwiftExecutableCandidatesDiscoverDownloadsXcodeWithoutDeveloperDir(t *testing.T) {
	temp := t.TempDir()
	downloadsSwift := filepath.Join(temp, "Downloads", "Xcode-beta.app", "Contents", "Developer", "usr", "bin", "swift")
	makeFakeSwift(t, downloadsSwift)
	isolateXcodeSearch(t, filepath.Join(temp, "Downloads"))

	resolved, searched := resolveSwiftExecutablePath(map[string]string{
		"HOME": temp,
		"PATH": "/usr/bin:/bin",
	})
	if resolved != downloadsSwift {
		t.Fatalf("resolved = %q, want Downloads Xcode swift %q", resolved, downloadsSwift)
	}
	if len(searched) == 0 || searched[0] != downloadsSwift {
		t.Fatalf("searched[0] = %v, want Xcode swift first", searched)
	}
}

// TestCopyExecutableFileReplacesAtomically verifies install over a live
// helper: the destination is replaced via rename, so a concurrently running
// process keeps its old inode and the ".new" temp never survives.
func TestCopyExecutableFileReplacesAtomically(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := os.WriteFile(src, []byte("new-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := copyExecutableFile(src, dst); err != nil {
		t.Fatalf("copyExecutableFile() error = %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "new-bytes" {
		t.Fatalf("dst = %q (err=%v), want new-bytes", data, err)
	}
	if _, err := os.Stat(dst + ".new"); !os.IsNotExist(err) {
		t.Fatalf("temp file survived: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Fatalf("dst perms = %o, want 755", perm)
	}
}

func TestXcodeDeveloperDirForSwift(t *testing.T) {
	tests := []struct {
		swiftPath string
		want      string
	}{
		{swiftPath: "/Applications/Xcode.app/Contents/Developer/usr/bin/swift", want: "/Applications/Xcode.app/Contents/Developer"},
		{swiftPath: "/Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/bin/swift", want: "/Applications/Xcode.app/Contents/Developer"},
		{swiftPath: "/usr/bin/swift", want: ""},
		{swiftPath: "/opt/homebrew/bin/swift", want: ""},
	}
	for _, tt := range tests {
		if got := xcodeDeveloperDirForSwift(tt.swiftPath); got != tt.want {
			t.Errorf("xcodeDeveloperDirForSwift(%q) = %q, want %q", tt.swiftPath, got, tt.want)
		}
	}
}
