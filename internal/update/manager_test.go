package update

import (
	"path/filepath"
	"testing"
)

func TestDetectManagerCursorAgent(t *testing.T) {
	manager := DetectManager(Tool{
		Name:          "Cursor",
		Package:       "cursor-agent",
		BinaryName:    "cursor-agent",
		BinaryAliases: []string{"cursor", "agent"},
	})

	if manager != CursorAgent {
		t.Fatalf("DetectManager(cursor-agent) = %q, want %q", manager, CursorAgent)
	}
}

func TestCursorAgentInstallCommand(t *testing.T) {
	cmd := CursorAgent.InstallCommand(Tool{BinaryName: "cursor-agent", BinaryAliases: []string{"cursor", "agent"}})
	if len(cmd.Args) != 3 {
		t.Fatalf("unexpected args length: %v", cmd.Args)
	}
	if cmd.Args[0] != "bash" || cmd.Args[1] != "-lc" || cmd.Args[2] != "curl https://cursor.com/install -fsS | bash" {
		t.Fatalf("CursorAgent.InstallCommand args = %v, want [bash -lc 'curl https://cursor.com/install -fsS | bash']", cmd.Args)
	}
}

func TestCursorAgentNoChangeOutput(t *testing.T) {
	if !CursorAgent.IsNoChangeOutput("Already on the latest version") {
		t.Fatal("expected latest-version message to be treated as no change")
	}
}

func TestDetectManagerByPackagePrefix(t *testing.T) {
	tests := []struct {
		pkg  string
		want Manager
	}{
		{pkg: "cargo:uv", want: Cargo},
		{pkg: "go:github.com/foo/bar/cmd/baz", want: GoInstall},
		{pkg: "pip:llm", want: Pip},
	}

	for _, tc := range tests {
		got := DetectManager(Tool{Package: tc.pkg, BinaryName: "dummy"})
		if got != tc.want {
			t.Fatalf("DetectManager(%q) = %q, want %q", tc.pkg, got, tc.want)
		}
	}
}

func TestInstallCommandForExpandedManagers(t *testing.T) {
	cargoCmd := Cargo.InstallCommand(Tool{Package: "cargo:uv"})
	if got := cargoCmd.Args; len(got) != 4 || got[0] != "cargo" || got[1] != "install" || got[2] != "uv" || got[3] != "--locked" {
		t.Fatalf("unexpected cargo command: %v", got)
	}

	goCmd := GoInstall.InstallCommand(Tool{Package: "go:github.com/some/tool/cmd/tool"})
	if got := goCmd.Args; len(got) != 3 || got[0] != "go" || got[1] != "install" || got[2] != "github.com/some/tool/cmd/tool@latest" {
		t.Fatalf("unexpected go install command: %v", got)
	}

	pipCmd := Pip.InstallCommand(Tool{Package: "pip:llm"})
	if got := pipCmd.Args; len(got) != 6 || got[0] != "python3" || got[1] != "-m" || got[2] != "pip" || got[3] != "install" || got[4] != "--upgrade" || got[5] != "llm" {
		t.Fatalf("unexpected pip command: %v", got)
	}
}

func TestPreferredBinariesIncludesAliases(t *testing.T) {
	got := preferredBinaries(Tool{BinaryName: "cursor-agent", BinaryAliases: []string{"cursor", "agent", "cursor"}})
	want := []string{"cursor-agent", "cursor", "agent"}
	if len(got) != len(want) {
		t.Fatalf("preferredBinaries len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("preferredBinaries[%d] = %q, want %q (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestDetectManagerPrefersBinaryProvenanceOverPackageLists(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "codex" {
			return "/opt/homebrew/bin/codex", nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		switch name {
		case "brew":
			if len(args) >= 1 && args[0] == "--prefix" {
				return []byte("/opt/homebrew\n"), nil
			}
		case "npm":
			if len(args) >= 2 && args[0] == "prefix" && args[1] == "-g" {
				return []byte("/Users/test/.npm-global\n"), nil
			}
			if len(args) >= 3 && args[0] == "list" {
				return []byte("@openai/codex@1.2.3\n"), nil
			}
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "@openai/codex", BinaryName: "codex", BrewPackage: "codex"})
	if got != Brew {
		t.Fatalf("DetectManager() = %q, want %q", got, Brew)
	}
}

func TestDetectManagerUsesPnpmPrefixOwnership(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "claude" {
			return "/Users/test/Library/pnpm/claude", nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		switch name {
		case "pnpm":
			if len(args) >= 1 && args[0] == "bin" {
				return []byte("/Users/test/Library/pnpm\n"), nil
			}
		case "npm":
			if len(args) >= 2 && args[0] == "prefix" && args[1] == "-g" {
				return []byte("/Users/test/.npm-global\n"), nil
			}
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "@anthropic-ai/claude-code", BinaryName: "claude"})
	if got != Pnpm {
		t.Fatalf("DetectManager() = %q, want %q", got, Pnpm)
	}
}

func TestDetectManagerUsesGoInstallBinaryOwnership(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "mytool" {
			return filepath.Clean("/home/test/go/bin/mytool"), nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		if name == "go" && len(args) >= 2 && args[0] == "env" && args[1] == "GOPATH" {
			return []byte("/home/test/go\n"), nil
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "go:github.com/example/mytool", BinaryName: "mytool"})
	if got != GoInstall {
		t.Fatalf("DetectManager() = %q, want %q", got, GoInstall)
	}
}

func TestDetectManagerReturnsUnknownWhenPackageAndProvenanceAreAmbiguous(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "mystery" {
			return "/tmp/mystery", nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "mystery-tool", BinaryName: "mystery"})
	if got != Unknown {
		t.Fatalf("DetectManager() = %q, want %q", got, Unknown)
	}
}

func TestResolveManagerForInstallFallsBackToDefaultManager(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	got := ResolveManagerForInstall(Tool{Package: "mystery-tool", BinaryName: "mystery"})
	if got != Npm {
		t.Fatalf("ResolveManagerForInstall() = %q, want %q", got, Npm)
	}
}

func stubManagerDetection(t *testing.T) func() {
	t.Helper()
	origLookup := binaryLookup
	origOutput := commandOutput
	binaryLookup = func(string) (string, error) { return "", errExecutableNotFound }
	commandOutput = func(string, ...string) ([]byte, error) { return nil, errExecutableNotFound }
	return func() {
		binaryLookup = origLookup
		commandOutput = origOutput
	}
}
