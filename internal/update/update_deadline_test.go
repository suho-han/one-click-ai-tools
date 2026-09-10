package update

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// TestMatchesInstalledPackageAbortsOnDeadline drives a hung package-list
// command (the slowest probe path) through the ctx-aware commandOutput seam:
// the deadline must kill the subprocess instead of blocking the run.
func TestMatchesInstalledPackageAbortsOnDeadline(t *testing.T) {
	orig := commandOutput
	commandOutput = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// sleep only ends when ctx kills it at the deadline
		return exec.CommandContext(ctx, "sleep", "30").CombinedOutput()
	}
	t.Cleanup(func() { commandOutput = orig })

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	matches := matchesInstalledPackage(ctx, Npm, Tool{Package: "some-package"})
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("matchesInstalledPackage took %v with a hung probe, want bounded by the deadline", elapsed)
	}
	if matches {
		t.Fatal("matchesInstalledPackage = true, want false when the probe never completes")
	}
}

// TestInstallTimeoutCeilingSanity guards the shipped default so a silent
// change cannot make agent-update unbounded (or uselessly short).
func TestInstallTimeoutCeilingSanity(t *testing.T) {
	if installTimeout <= 0 || installTimeout > 30*time.Minute {
		t.Fatalf("installTimeout = %v, want a positive ceiling of at most 30m", installTimeout)
	}
}
