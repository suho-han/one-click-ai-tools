package cmd

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// createUpdateArchive writes a tar.gz with the given name->content entries at
// the archive root, mirroring the release tarball layout (oct + OctMenubarApp).
func createUpdateArchive(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// withDarwinSelfUpdate forces the darwin-only helper install path so the
// test runs on every CI host.
func withDarwinSelfUpdate(t *testing.T) {
	t.Helper()
	old := selfUpdateGOOS
	selfUpdateGOOS = "darwin"
	t.Cleanup(func() { selfUpdateGOOS = old })
}

func TestInstallMenubarHelperFromArchive(t *testing.T) {
	withDarwinSelfUpdate(t)
	// The source default version is a beta, which would route the install to
	// OctMenubarApp-beta; this test pins the stable helper name.
	stubMenubarTrack(t, "0.1.6")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "one-click-ai-tools_darwin_arm64.tar.gz")
	createUpdateArchive(t, archivePath, map[string]string{
		"oct":           "#!/bin/sh\n",
		"OctMenubarApp": "FAKE-HELPER-BINARY",
	})

	extractDir := filepath.Join(dir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installMenubarHelperFromArchive(archivePath, extractDir, filepath.Join(home, ".local", "bin", "oct")); err != nil {
		t.Fatalf("installMenubarHelperFromArchive: %v", err)
	}

	dst := filepath.Join(home, ".local", "bin", "OctMenubarApp")
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("helper not installed at %s: %v", dst, err)
	}
	// Windows has no executable bit: the 0755 chmod only clears the
	// read-only attribute, so the mode check would never pass there.
	if info.IsDir() || (runtime.GOOS != "windows" && info.Mode()&0o111 == 0) {
		t.Fatalf("installed helper mode = %s, want executable file", info.Mode())
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "FAKE-HELPER-BINARY" {
		t.Fatalf("helper content = %q err=%v, want copied payload", data, err)
	}
}

func TestInstallMenubarHelperFollowsCustomBetaExecutable(t *testing.T) {
	withDarwinSelfUpdate(t)
	stubMenubarTrack(t, "0.1.6-beta.1")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	customDir := filepath.Join(t.TempDir(), "custom-bin")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatal(err)
	}
	octPath := filepath.Join(customDir, "oct-beta")
	defaultHelper := filepath.Join(home, ".local", "bin", betaHelperName)
	if err := os.MkdirAll(filepath.Dir(defaultHelper), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultHelper, []byte("OLD-HELPER"), 0o755); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "asset.tar.gz")
	createUpdateArchive(t, archivePath, map[string]string{
		"oct":           "#!/bin/sh\n",
		"OctMenubarApp": "NEW-HELPER",
	})
	extractDir := t.TempDir()
	if err := installMenubarHelperFromArchive(archivePath, extractDir, octPath); err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(customDir, betaHelperName)
	if content, err := os.ReadFile(installed); err != nil || string(content) != "NEW-HELPER" {
		t.Fatalf("custom helper = %q, %v", content, err)
	}
	resolved, _ := resolveMenubarHelperPath(map[string]string{"HOME": home}, octPath, t.TempDir())
	if resolved != installed {
		t.Fatalf("resolved helper = %q, want custom installation %q", resolved, installed)
	}
	if content, err := os.ReadFile(defaultHelper); err != nil || string(content) != "OLD-HELPER" {
		t.Fatalf("unrelated default helper changed: %q, %v", content, err)
	}
}

func TestInstallMenubarHelperFromArchiveMissingHelper(t *testing.T) {
	withDarwinSelfUpdate(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.tar.gz")
	createUpdateArchive(t, archivePath, map[string]string{"oct": "#!/bin/sh\n"})

	extractDir := filepath.Join(dir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}
	err := installMenubarHelperFromArchive(archivePath, extractDir, filepath.Join(home, ".local", "bin", "oct"))
	if err == nil || !strings.Contains(err.Error(), "not bundled") {
		t.Fatalf("err = %v, want not-bundled error (older release archives)", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".local", "bin", "OctMenubarApp")); statErr == nil {
		t.Fatalf("helper must not be installed when the archive lacks it")
	}
}

func TestInstallMenubarHelperSkipsNonDarwin(t *testing.T) {
	old := selfUpdateGOOS
	selfUpdateGOOS = "linux"
	t.Cleanup(func() { selfUpdateGOOS = old })

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "asset.tar.gz")
	createUpdateArchive(t, archivePath, map[string]string{"oct": "x", "OctMenubarApp": "x"})
	extractDir := filepath.Join(dir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := installMenubarHelperFromArchive(archivePath, extractDir, filepath.Join(dir, "oct")); err != nil {
		t.Fatalf("non-darwin install must be a silent no-op, got %v", err)
	}
}
