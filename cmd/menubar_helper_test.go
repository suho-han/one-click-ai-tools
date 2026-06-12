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
