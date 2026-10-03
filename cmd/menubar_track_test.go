package cmd

import (
	"strings"
	"testing"
)

func TestMenubarProcessTrack(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{
			name:    "beta command line",
			command: "/Users/me/.local/bin/oct-beta menubar",
			want:    menubarTrackBeta,
		},
		{
			name:    "beta basename anywhere in argv",
			command: "env /opt/tools/oct-beta menubar --legacy",
			want:    menubarTrackBeta,
		},
		{
			name:    "a path merely containing the substring is not beta",
			command: "/Users/me/bin/oct-beta-tools/oct menubar",
			want:    menubarTrackStable,
		},
		{
			name:    "stable legacy menubar",
			command: "/Users/me/bin/oct menubar --legacy",
			want:    menubarTrackStable,
		},
		{
			name:    "shared OctMenubarApp helper counts as stable track",
			command: "/Users/me/.local/bin/OctMenubarApp",
			want:    menubarTrackStable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := menubarProcessTrack(tt.command); got != tt.want {
				t.Fatalf("menubarProcessTrack(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

// TestCurrentMenubarTrackFollowsVersion pins the dev-binary fallback: a
// basename that is not oct-beta is classified by its version string.
func TestCurrentMenubarTrackFollowsVersion(t *testing.T) {
	origVersion := rootCmd.Version
	t.Cleanup(func() { rootCmd.Version = origVersion })

	rootCmd.Version = "0.1.6-beta.1"
	if got := currentMenubarTrack(); got != menubarTrackBeta {
		t.Fatalf("currentMenubarTrack() with beta version = %q, want %q", got, menubarTrackBeta)
	}

	rootCmd.Version = "0.1.6"
	if got := currentMenubarTrack(); got != menubarTrackStable {
		t.Fatalf("currentMenubarTrack() with stable version = %q, want %q", got, menubarTrackStable)
	}
}

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
