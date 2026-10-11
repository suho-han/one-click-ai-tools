package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// stubMenubarTrack pins the menubar track for a test. The test binary's
// basename is never oct-beta, so the version string decides the track.
func stubMenubarTrack(t *testing.T, version string) {
	t.Helper()
	orig := rootCmd.Version
	rootCmd.Version = version
	t.Cleanup(func() { rootCmd.Version = orig })
}

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

func TestMenubarProcessTrackRecognizesBetaHelper(t *testing.T) {
	if got := menubarProcessTrack("/Users/me/.local/bin/OctMenubarApp-beta"); got != menubarTrackBeta {
		t.Fatalf("menubarProcessTrack(beta helper) = %q, want %q", got, menubarTrackBeta)
	}
}

func TestIsMenubarStopTargetIncludesBetaHelper(t *testing.T) {
	if !isMenubarStopTarget(200, 42, "/Users/me/.local/bin/OctMenubarApp-beta") {
		t.Fatal("isMenubarStopTarget(beta helper) = false, want true")
	}
}

func TestMenubarHelperCandidatesBetaTrackUseOwnHelperName(t *testing.T) {
	stubMenubarTrack(t, "0.1.6-beta.1")

	home := t.TempDir()
	candidates := menubarHelperCandidates(map[string]string{"HOME": home}, filepath.Join(home, ".local", "bin", "oct-beta"), "")
	if !candidateListContains(candidates, filepath.Join(home, ".local", "bin", betaHelperName)) {
		t.Fatalf("beta candidates missing the beta helper install location:\n%s", strings.Join(candidates, "\n"))
	}
	for _, candidate := range candidates {
		if filepath.Base(candidate) == stableHelperName {
			t.Fatalf("beta candidates must not include the stable helper name:\n%s", strings.Join(candidates, "\n"))
		}
	}
}

// candidateListContains matches whole candidate paths, not substrings —
// OctMenubarApp-beta contains OctMenubarApp as a substring.
func candidateListContains(candidates []string, want string) bool {
	for _, candidate := range candidates {
		if candidate == want {
			return true
		}
	}
	return false
}

func TestMenubarHelperCandidatesStableTrackKeepSourceTreePaths(t *testing.T) {
	stubMenubarTrack(t, "0.1.6")

	home := t.TempDir()
	workDir := filepath.Join(home, "repo", "sub")
	candidates := menubarHelperCandidates(map[string]string{"HOME": home}, filepath.Join(home, "repo", "oct"), workDir)
	if !candidateListContains(candidates, filepath.Join(home, "repo", "macos", "OctMenubar", ".build", "debug", stableHelperName)) {
		t.Fatalf("stable candidates missing the source-tree build path:\n%s", strings.Join(candidates, "\n"))
	}
	for _, candidate := range candidates {
		if filepath.Base(candidate) == betaHelperName {
			t.Fatalf("stable candidates must not include the beta helper name:\n%s", strings.Join(candidates, "\n"))
		}
	}
}
