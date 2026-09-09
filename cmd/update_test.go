package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

// serveChecksums points checksumBaseURL at an httptest server serving the
// given body (or HTTP error when status is non-200) and restores the
// original value on cleanup.
func serveChecksums(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	orig := checksumBaseURL
	checksumBaseURL = srv.URL
	t.Cleanup(func() { checksumBaseURL = orig })
}

func TestVerifyReleaseAssetChecksum(t *testing.T) {
	asset := releaseAsset{
		Name: "one-click-ai-tools_darwin_arm64.tar.gz",
		URL:  "https://github.com/suho-han/one-click-ai-tools/releases/download/v1.2.3/one-click-ai-tools_darwin_arm64.tar.gz",
	}
	archiveContent := []byte("archive-bytes")
	sum := sha256.Sum256(archiveContent)
	validHash := hex.EncodeToString(sum[:])

	tests := []struct {
		name        string
		status      int
		checksums   string
		archivePath string
		wantErr     string
	}{
		{
			name:        "matching checksum passes",
			status:      http.StatusOK,
			checksums:   fmt.Sprintf("%s  %s\n", validHash, asset.Name),
			archivePath: "unused", // set below
		},
		{
			name:        "mismatch fails",
			status:      http.StatusOK,
			checksums:   fmt.Sprintf("deadbeef  %s\n", asset.Name),
			archivePath: "unused",
			wantErr:     "checksum mismatch",
		},
		{
			name:        "checksums.txt download failure fails closed",
			status:      http.StatusNotFound,
			checksums:   "",
			archivePath: "unused",
			wantErr:     "checksums.txt unavailable",
		},
		{
			name:        "missing entry fails closed",
			status:      http.StatusOK,
			checksums:   "deadbeef  one-click-ai-tools_linux_amd64.tar.gz\n",
			archivePath: "unused",
			wantErr:     "checksum entry not found",
		},
		{
			name:        "unreadable archive fails",
			status:      http.StatusOK,
			checksums:   fmt.Sprintf("%s  %s\n", validHash, asset.Name),
			archivePath: "does-not-exist",
			wantErr:     "checksum computation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serveChecksums(t, tt.status, tt.checksums)
			archivePath := tt.archivePath
			if archivePath == "unused" {
				archivePath = filepath.Join(t.TempDir(), asset.Name)
				if err := os.WriteFile(archivePath, archiveContent, 0o644); err != nil {
					t.Fatalf("WriteFile(archive) error = %v", err)
				}
			}

			err := verifyReleaseAssetChecksum(t.Context(), selfUpdateRepo, asset, archivePath)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("verifyReleaseAssetChecksum() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("verifyReleaseAssetChecksum() = nil, want error containing %q", tt.wantErr)
			}
			if got := err.Error(); !strings.Contains(got, tt.wantErr) {
				t.Fatalf("error = %q, want containing %q", got, tt.wantErr)
			}
		})
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
