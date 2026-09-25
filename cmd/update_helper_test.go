package cmd

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
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
	if err := installMenubarHelperFromArchive(archivePath, extractDir); err != nil {
		t.Fatalf("installMenubarHelperFromArchive: %v", err)
	}

	dst := filepath.Join(home, ".local", "bin", "OctMenubarApp")
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("helper not installed at %s: %v", dst, err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		t.Fatalf("installed helper mode = %s, want executable file", info.Mode())
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "FAKE-HELPER-BINARY" {
		t.Fatalf("helper content = %q err=%v, want copied payload", data, err)
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
	err := installMenubarHelperFromArchive(archivePath, extractDir)
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
	if err := installMenubarHelperFromArchive(archivePath, extractDir); err != nil {
		t.Fatalf("non-darwin install must be a silent no-op, got %v", err)
	}
}
