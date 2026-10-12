//go:build !windows

package update

import (
	"os"
	"testing"
)

func writeStubExecutable(t *testing.T, path, script string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("write stub %s: %v", path, err)
	}
}
