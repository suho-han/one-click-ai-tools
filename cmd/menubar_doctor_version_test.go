package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// stubMenubarHelperVersion pins the helper --version probe so doctor tests do
// not execute real helpers.
func stubMenubarHelperVersion(t *testing.T, version string) {
	t.Helper()
	old := probeMenubarHelperVersion
	probeMenubarHelperVersion = func(helperPath string) string {
		return version
	}
	t.Cleanup(func() { probeMenubarHelperVersion = old })
}

// pointMenubarHelperAtFake installs a fake helper executable and points the
// helper discovery env at it, forcing launch mode "swift-helper".
func pointMenubarHelperAtFake(t *testing.T) string {
	t.Helper()
	helperPath := filepath.Join(t.TempDir(), "OctMenubarApp")
	if err := os.WriteFile(helperPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MENUBAR_HELPER_PATH", helperPath)
	return helperPath
}

func TestMenubarDoctorReportsHelperVersionSkew(t *testing.T) {
	pointMenubarHelperAtFake(t)
	stubMenubarHelperVersion(t, "0.1.5")
	oldVersion := rootCmd.Version
	rootCmd.Version = "0.1.6-beta.4"
	t.Cleanup(func() { rootCmd.Version = oldVersion })

	report, err := collectMenubarDoctorReport()
	if err != nil {
		t.Fatalf("collectMenubarDoctorReport: %v", err)
	}
	if report.OctVersion != "0.1.6-beta.4" {
		t.Fatalf("oct version = %q", report.OctVersion)
	}
	if report.HelperVersion != "0.1.5" {
		t.Fatalf("helper version = %q, want probed value", report.HelperVersion)
	}
	if !report.VersionSkew {
		t.Fatalf("VersionSkew = false, want true for %q vs %q", report.OctVersion, report.HelperVersion)
	}
}

func TestMenubarDoctorMatchingVersionsDoNotReportSkew(t *testing.T) {
	pointMenubarHelperAtFake(t)
	stubMenubarHelperVersion(t, "0.1.6-beta.4")
	oldVersion := rootCmd.Version
	rootCmd.Version = "v0.1.6-beta.4"
	t.Cleanup(func() { rootCmd.Version = oldVersion })

	report, err := collectMenubarDoctorReport()
	if err != nil {
		t.Fatalf("collectMenubarDoctorReport: %v", err)
	}
	if report.VersionSkew {
		t.Fatalf("VersionSkew = true for matching versions (helper normalization missing v?)")
	}
}

func TestMenubarDoctorHelperVersionUnknownWhenHelperAbsent(t *testing.T) {
	// Stub the probe so the test stays independent of whatever helper the
	// host happens to have installed.
	stubMenubarHelperVersion(t, "")
	t.Setenv("OCT_MENUBAR_HELPER_PATH", "")
	report, err := collectMenubarDoctorReport()
	if err != nil {
		t.Fatalf("collectMenubarDoctorReport: %v", err)
	}
	if report.HelperVersion != "" || report.VersionSkew {
		t.Fatalf("helper version/skew = %q/%v, want empty/false without a stamped helper", report.HelperVersion, report.VersionSkew)
	}
}

func TestProbeMenubarHelperVersionScansBinary(t *testing.T) {
	dir := t.TempDir()
	stamped := filepath.Join(dir, "OctMenubarApp-stamped")
	payload := "CFBundleExecutable\x00oct-menubar-helper-version=0.2.0\x00more-bytes"
	if err := os.WriteFile(stamped, []byte(payload), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := probeMenubarHelperVersion(stamped); got != "0.2.0" {
		t.Fatalf("version = %q, want 0.2.0", got)
	}

	vPrefixed := filepath.Join(dir, "OctMenubarApp-v")
	if err := os.WriteFile(vPrefixed, []byte("oct-menubar-helper-version=v1.2.3"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := probeMenubarHelperVersion(vPrefixed); got != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3 (v-prefix stripped)", got)
	}

	// A helper predating version stamping has no marker: "" tells doctor to
	// report the version as unknown rather than executing the old binary.
	old := filepath.Join(dir, "OctMenubarApp-old")
	if err := os.WriteFile(old, []byte("\xCF\xFA\xED\xFEold-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := probeMenubarHelperVersion(old); got != "" {
		t.Fatalf("version = %q, want empty for unstamped helper", got)
	}
}

func TestMenubarHelperCandidatesIncludeReleaseBuild(t *testing.T) {
	candidates := menubarHelperCandidates(map[string]string{"HOME": "/tmp/fake-home"}, "/usr/bin/oct", "/tmp/work")
	want := filepath.Join("/tmp/work", "macos", "OctMenubar", ".build", "release", "OctMenubarApp")
	found := false
	for _, candidate := range candidates {
		if candidate == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates %v missing release build path %s", candidates, want)
	}
}
