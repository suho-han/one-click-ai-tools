package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestReleaseDoctorJSONUsesGitHubReleaseFields(t *testing.T) {
	report := releaseDoctorReport{LatestRelease: "v1.2.3", UpdateAvailable: true}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	jsonText := string(data)
	if !strings.Contains(jsonText, `"latest_release":"v1.2.3"`) {
		t.Fatalf("expected latest_release field, got %s", jsonText)
	}
	if strings.Contains(jsonText, "npm_") || strings.Contains(jsonText, "repo_npmrc") {
		t.Fatalf("unexpected npm release doctor field in %s", jsonText)
	}
}

func TestCollectReleaseDoctorReportChecksGitHubLatestRelease(t *testing.T) {
	origCommand := releaseDoctorCommand
	releaseDoctorCommand = fakeReleaseDoctorCommand
	origLatest := releaseDoctorLatestRelease
	releaseDoctorLatestRelease = func(ctx context.Context, repo string) (string, error) {
		if repo != selfUpdateRepo {
			t.Fatalf("repo = %q, want %q", repo, selfUpdateRepo)
		}
		return "v1.2.3", nil
	}
	t.Cleanup(func() {
		releaseDoctorCommand = origCommand
		releaseDoctorLatestRelease = origLatest
	})

	report := collectReleaseDoctorReport(context.Background())
	if report.LatestRelease != "v1.2.3" {
		t.Fatalf("expected GitHub latest release, got %q", report.LatestRelease)
	}
	if !report.UpdateAvailable {
		t.Fatal("expected update_available for newer latest release")
	}
}

func fakeReleaseDoctorCommand(name string, args ...string) *exec.Cmd {
	cmdArgs := []string{"-test.run=TestReleaseDoctorCommandHelper", "--", name}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "OCT_RELEASE_DOCTOR_HELPER=1")
	return cmd
}

func TestReleaseDoctorCommandHelper(t *testing.T) {
	if os.Getenv("OCT_RELEASE_DOCTOR_HELPER") != "1" {
		return
	}
	args := os.Args
	sep := -1
	for i, arg := range args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep < 0 || sep+1 >= len(args) {
		fmt.Fprintln(os.Stderr, "missing helper command")
		os.Exit(2)
	}

	name := args[sep+1]
	cmdArgs := args[sep+2:]
	switch name {
	case "git":
		handleFakeGit(cmdArgs)
	default:
		fmt.Fprintf(os.Stderr, "unexpected command: %s\n", name)
		os.Exit(2)
	}
	os.Exit(0)
}

func handleFakeGit(args []string) {
	switch strings.Join(args, " ") {
	case "status --short":
		return
	case "branch --show-current":
		fmt.Println("main")
	case "remote get-url origin":
		fmt.Println("git@github.com:suho-han/one-click-ai-tools.git")
	default:
		fmt.Fprintf(os.Stderr, "unexpected git args: %s\n", strings.Join(args, " "))
		os.Exit(1)
	}
}
