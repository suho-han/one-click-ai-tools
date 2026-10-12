//go:build windows

package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Windows cannot execute the shell scripts used by the Unix fixtures. Reuse
// the test executable as a real .exe and intercept its startup in TestMain.
func TestMain(m *testing.M) {
	if os.Getenv("OCT_UPDATE_TEST_PROCESS") == "1" {
		os.Exit(runStubProcess())
	}
	os.Exit(m.Run())
}

func writeStubExecutable(t *testing.T, path, _ string) {
	t.Helper()
	source, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Windows keeps the running test image locked; a hard link would make
	// TempDir cleanup unable to remove any fixture pointing to that image.
	in, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy executable stub: %v; close: %v", copyErr, closeErr)
	}
}

func runStubProcess() int {
	name := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
	args := os.Args[1:]
	switch name {
	case "npm":
		if len(args) == 0 {
			return 1
		}
		switch args[0] {
		case "list":
			version := os.Getenv("OCT_UPDATE_TEST_NPM_BEFORE")
			if state := os.Getenv("OCT_UPDATE_TEST_NPM_STATE"); state != "" {
				if _, err := os.Stat(state); err == nil {
					version = os.Getenv("OCT_UPDATE_TEST_NPM_AFTER")
				}
			}
			if version != "" {
				fmt.Println("mmx-cli@" + version)
			}
			return 0
		case "outdated":
			switch os.Getenv("OCT_UPDATE_TEST_OUTDATED") {
			case "json1":
				fmt.Println(`{"mmx-cli":{"current":"1.0.0","latest":"2.0.0"}}`)
				return 1
			case "json0":
				fmt.Println(`{}`)
				return 0
			default:
				return 1
			}
		case "view":
			if version := os.Getenv("OCT_UPDATE_TEST_VIEW"); version != "" {
				fmt.Println(version)
			}
			return 0
		case "install":
			if message := os.Getenv("OCT_UPDATE_TEST_INSTALL_ERROR"); message != "" {
				fmt.Println(message)
				return 1
			}
			if state := os.Getenv("OCT_UPDATE_TEST_NPM_STATE"); state != "" {
				if err := os.WriteFile(state, nil, 0o600); err != nil {
					return 1
				}
			}
			return 0
		}
	case "agy":
		if len(args) > 0 && args[0] == "update" {
			fmt.Println("agy updated")
			return 0
		}
	}
	if version := os.Getenv("OCT_UPDATE_TEST_VERSION"); version != "" {
		fmt.Println(version)
		return 0
	}
	return 1
}
