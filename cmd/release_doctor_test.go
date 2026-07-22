package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestReleaseDoctorJSONTagForNPMUserConfig(t *testing.T) {
	report := releaseDoctorReport{NPMUserConfig: "/tmp/.npmrc"}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	jsonText := string(data)
	if !contains(jsonText, `"npm_userconfig":"/tmp/.npmrc"`) {
		t.Fatalf("expected npm_userconfig field, got %s", jsonText)
	}
	if contains(jsonText, `"***"`) {
		t.Fatalf("unexpected legacy redacted field key in %s", jsonText)
	}
}

func TestCollectReleaseDoctorReportChecksRenamedNPMPackage(t *testing.T) {
	origCommand := releaseDoctorCommand
	releaseDoctorCommand = fakeReleaseDoctorCommand
	t.Cleanup(func() { releaseDoctorCommand = origCommand })

	report := collectReleaseDoctorReport()
	if report.RegistryLatest != "1.2.3" {
		t.Fatalf("expected registry version from one-click-ai-tools lookup, got %q", report.RegistryLatest)
	}
	if report.NPMWhoami != "publisher" {
		t.Fatalf("expected npm whoami from fake npm, got %q", report.NPMWhoami)
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
	case "npm":
		handleFakeNPM(cmdArgs)
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
		fmt.Println("https://example.test/suho-han/one-click-ai-tools.git")
	default:
		fmt.Fprintf(os.Stderr, "unexpected git args: %s\n", strings.Join(args, " "))
		os.Exit(1)
	}
}

func handleFakeNPM(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "missing npm args")
		os.Exit(1)
	}
	switch args[0] {
	case "view":
		if len(args) < 3 || args[1] != "one-click-ai-tools" || args[2] != "version" {
			fmt.Fprintf(os.Stderr, "unexpected npm view args: %s\n", strings.Join(args, " "))
			os.Exit(1)
		}
		fmt.Println("1.2.3")
	case "config":
		if len(args) == 3 && args[1] == "get" && args[2] == "userconfig" {
			fmt.Println("/tmp/npmrc")
			return
		}
		fmt.Fprintf(os.Stderr, "unexpected npm config args: %s\n", strings.Join(args, " "))
		os.Exit(1)
	case "whoami":
		fmt.Println("publisher")
	default:
		fmt.Fprintf(os.Stderr, "unexpected npm args: %s\n", strings.Join(args, " "))
		os.Exit(1)
	}
}
