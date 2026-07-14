package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "git"), `#!/bin/sh
case "$1 $2" in
"status --short")
	exit 0
	;;
"branch --show-current")
	echo main
	exit 0
	;;
"remote get-url")
	echo https://example.test/suho-han/one-click-ai-tools.git
	exit 0
	;;
*)
	echo "unexpected git args: $*" >&2
	exit 1
	;;
esac
`)

	writeExecutable(t, filepath.Join(binDir, "npm"), `#!/bin/sh
case "$1" in
view)
	if [ "$2" != "one-click-ai-tools" ]; then
		echo "unexpected package: $2" >&2
		exit 2
	fi
	if [ "$3" != "version" ]; then
		echo "unexpected npm view field: $3" >&2
		exit 3
	fi
	echo 1.2.3
	exit 0
	;;
config)
	if [ "$2" = "get" ] && [ "$3" = "userconfig" ]; then
		echo /tmp/npmrc
		exit 0
	fi
	echo "unexpected npm config args: $*" >&2
	exit 4
	;;
whoami)
	echo publisher
	exit 0
	;;
*)
	echo "unexpected npm args: $*" >&2
	exit 5
	;;
esac
`)

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	report := collectReleaseDoctorReport()
	if report.RegistryLatest != "1.2.3" {
		t.Fatalf("expected registry version from one-click-ai-tools lookup, got %q", report.RegistryLatest)
	}
	if report.NPMWhoami != "publisher" {
		t.Fatalf("expected npm whoami from fake npm, got %q", report.NPMWhoami)
	}
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable failed: %v", err)
	}
}
