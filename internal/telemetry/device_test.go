package telemetry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMacFamilyUsesProductNameNotModelIdentifier(t *testing.T) {
	for _, tc := range []struct{ machine, want string }{
		{"Mac mini", "Mac mini"},
		{"Mac mini (2023)", "Mac mini"},
		{"MacBook Air", "MacBook Air"},
		{"MacBook Pro", "MacBook Pro"},
		{"Mac Studio", "Mac Studio"},
		{"iMac", "iMac"},
		{"Mac Pro", "Mac Pro"},
		{"Mac14,3", "Mac"},
		{"secret-hostname", "Mac"},
		{"", "Mac"},
	} {
		if got := macFamily(tc.machine); got != tc.want {
			t.Errorf("macFamily(%q) = %q, want %q", tc.machine, got, tc.want)
		}
	}
	machine, err := decodeMachineName([]byte(`{"SPHardwareDataType":[{"machine_name":"Mac mini","machine_model":"Mac14,3","serial_number":"secret-serial"}]}`))
	if err != nil || macFamily(machine) != "Mac mini" {
		t.Fatalf("decoded machine family = %q (%v)", machine, err)
	}
	if _, err := decodeMachineName([]byte(`{"SPHardwareDataType":[{"serial_number":"secret-serial"}]}`)); err == nil {
		t.Fatal("accepted hardware report without machine_name")
	}
	var limited boundedOutput
	if _, err := limited.Write(make([]byte, maxProfilerOutput+1)); err == nil || limited.Len() != 0 {
		t.Fatalf("profiler output limit failed: len=%d, err=%v", limited.Len(), err)
	}
}

func TestDeviceFamilyCacheStoresOnlySanitizedFamily(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".oct", "telemetry")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	calls := 0
	probe := func(context.Context) (string, error) {
		calls++
		return "Mac mini (private-serial)", nil
	}
	if got := deviceFamily("darwin", home, probe); got != "Mac mini" {
		t.Fatalf("family = %q", got)
	}
	if got := deviceFamily("darwin", home, probe); got != "Mac mini" || calls != 1 {
		t.Fatalf("cached family = %q, probe count = %d", got, calls)
	}
	path := filepath.Join(dir, "device-family")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "Mac mini\n" || strings.Contains(string(data), "private") {
		t.Fatalf("unsafe cache = %q (%v)", data, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Errorf("device cache permissions = %v (%v)", info, err)
	}
}

func TestDeviceFamilyRejectsUnsafeCacheAndNeverProbesOtherSystems(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".oct", "telemetry")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "device-family")
	outside := filepath.Join(home, "outside")
	if err := os.WriteFile(outside, []byte("Mac mini\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err := readCachedFamily(path); err == nil {
		t.Fatal("followed symlink cache")
	}
	calls := 0
	probe := func(context.Context) (string, error) { calls++; return "MacBook Air", nil }
	if got := deviceFamily("darwin", home, probe); got != "MacBook Air" {
		t.Fatalf("family = %q", got)
	}
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("replaced unsafe cache: %v (%v)", info, err)
	}
	for goos, want := range map[string]string{"linux": "Linux PC", "windows": "Windows PC", "plan9": "Other"} {
		if got := deviceFamily(goos, home, probe); got != want {
			t.Errorf("%s family = %q, want %q", goos, got, want)
		}
	}
	if calls != 1 {
		t.Errorf("unexpected probe count: %d", calls)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("Mac mini\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := readCachedFamily(path); err == nil {
		t.Fatal("accepted broadly readable cache")
	}
}

func TestUnknownMacDeviceCacheAvoidsRepeatProbe(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".oct", "telemetry"), 0700); err != nil {
		t.Fatal(err)
	}
	calls := 0
	probe := func(context.Context) (string, error) { calls++; return "Mac14,3", nil }
	for range 2 {
		if got := deviceFamily("darwin", home, probe); got != "Mac" {
			t.Fatalf("fallback family = %q", got)
		}
	}
	if calls != 1 {
		t.Errorf("probe invoked %d times", calls)
	}
	failureHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(failureHome, ".oct", "telemetry"), 0700); err != nil {
		t.Fatal(err)
	}
	failures := 0
	failed := func(context.Context) (string, error) { failures++; return "", errors.New("unavailable") }
	for range 2 {
		if got := deviceFamily("darwin", failureHome, failed); got != "Mac" {
			t.Errorf("failed probe family = %q", got)
		}
	}
	if failures != 2 {
		t.Errorf("failed probe was cached: %d invocations", failures)
	}
}
