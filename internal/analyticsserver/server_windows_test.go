//go:build windows

package analyticsserver

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRefusesWindowsWithoutPrivateFileModes(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "analytics.sqlite"))
	if err == nil || !strings.Contains(err.Error(), "POSIX private file permissions") {
		t.Fatalf("Open() error = %v, want private-permissions refusal", err)
	}
}
