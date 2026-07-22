package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompareReleaseVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "v0.1.1", b: "v0.1.2", want: -1},
		{a: "0.2.0", b: "v0.1.9", want: 1},
		{a: "v1.0.0", b: "1.0.0", want: 0},
		{a: "v1.2.3-beta.1", b: "v1.2.3", want: 0},
	}
	for _, tt := range tests {
		got := compareReleaseVersions(tt.a, tt.b)
		if got != tt.want {
			t.Fatalf("compareReleaseVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestReleaseAssetForDarwinArm64(t *testing.T) {
	asset, err := releaseAssetFor("darwin", "arm64", "v1.2.3")
	if err != nil {
		t.Fatalf("releaseAssetFor() error = %v", err)
	}
	if asset.Name != "one-click-ai-tools_darwin_arm64.tar.gz" {
		t.Fatalf("asset name = %q", asset.Name)
	}
	wantURL := "https://github.com/suho-han/one-click-ai-tools/releases/download/v1.2.3/one-click-ai-tools_darwin_arm64.tar.gz"
	if asset.URL != wantURL {
		t.Fatalf("asset URL = %q, want %q", asset.URL, wantURL)
	}
}

func TestChecksumForAsset(t *testing.T) {
	checksums := "abc123  one-click-ai-tools_linux_amd64.tar.gz\nfeed42  one-click-ai-tools_darwin_arm64.tar.gz\n"
	if got := checksumForAsset(checksums, "one-click-ai-tools_darwin_arm64.tar.gz"); got != "feed42" {
		t.Fatalf("checksumForAsset() = %q", got)
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src-oct")
	target := filepath.Join(dir, "oct")
	if err := os.WriteFile(src, []byte("new"), 0o755); err != nil {
		t.Fatalf("WriteFile(src) error = %v", err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := replaceExecutable(src, target); err != nil {
		t.Fatalf("replaceExecutable() error = %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile(target) error = %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("target content = %q", data)
	}
}
