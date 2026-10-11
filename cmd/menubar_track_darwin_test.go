//go:build darwin && cgo

package cmd

import (
	"strings"
	"testing"
)

func TestMenubarStopTargetsFromOutputSeparatesTracks(t *testing.T) {
	psOutput := strings.Join([]string{
		"  100 /Users/me/.local/bin/OctMenubarApp",
		"  101 /Users/me/.local/bin/oct-beta menubar",
		"  102 /Users/me/bin/oct menubar --legacy",
		"  103 /Users/me/bin/oct menubar stop",
		"  104 vim CONTEXT/menubar_helper_operations.md",
		"",
	}, "\n")

	stable := menubarStopTargetsFromOutput(psOutput, 999, menubarTrackStable)
	stablePIDs := targetPIDs(stable)
	if len(stable) != 2 || stablePIDs[0] != 100 || stablePIDs[1] != 102 {
		t.Fatalf("stable targets = %v (pids %v), want the helper and the stable legacy menubar only", stable, stablePIDs)
	}

	beta := menubarStopTargetsFromOutput(psOutput, 999, menubarTrackBeta)
	if len(beta) != 1 || beta[0].pid != 101 {
		t.Fatalf("beta targets = %v, want only the oct-beta menubar (pid 101)", beta)
	}
}

func targetPIDs(targets []menubarTargetProcess) []int {
	pids := make([]int, 0, len(targets))
	for _, target := range targets {
		pids = append(pids, target.pid)
	}
	return pids
}

// TestMenubarStopTargetsFromOutputExcludesCurrentPID guards the existing
// self-exclusion rule survives the track refactor.
func TestMenubarStopTargetsFromOutputExcludesCurrentPID(t *testing.T) {
	psOutput := strings.Join([]string{
		"  42 /Users/me/bin/oct menubar stop",
		"  100 /Users/me/bin/oct menubar --legacy",
		"",
	}, "\n")

	targets := menubarStopTargetsFromOutput(psOutput, 42, menubarTrackStable)
	if len(targets) != 1 || targets[0].pid != 100 {
		t.Fatalf("targets = %v, want only pid 100 (the current pid 42 excluded)", targets)
	}
}
