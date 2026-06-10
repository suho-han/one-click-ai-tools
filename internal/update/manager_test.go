package update

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func commandBase(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".exe")
}

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

func TestDetectManagerAntigravityInstaller(t *testing.T) {
	manager := DetectManager(Tool{
		Name:          "Antigravity CLI",
		Package:       "github.com/google-antigravity/antigravity-cli",
		BinaryName:    "agy",
		BinaryAliases: []string{"antigravity", "gemini", "gemini-cli"},
	})

	if manager != AntigravityInstaller {
		t.Fatalf("DetectManager(agy) = %q, want %q", manager, AntigravityInstaller)
	}
}

func TestCursorAgentInstallCommand(t *testing.T) {
	cmd := CursorAgent.InstallCommand(Tool{BinaryName: "cursor-agent", BinaryAliases: []string{"cursor", "agent"}})
	if len(cmd.Args) != 3 {
		t.Fatalf("unexpected args length: %v", cmd.Args)
	}
	if commandBase(cmd.Args[0]) != "bash" || cmd.Args[1] != "-lc" || cmd.Args[2] != "curl https://cursor.com/install -fsS | bash" {
		t.Fatalf("CursorAgent.InstallCommand args = %v, want [bash -lc 'curl https://cursor.com/install -fsS | bash']", cmd.Args)
	}
}

func TestAntigravityInstallCommand(t *testing.T) {
	cmd := AntigravityInstaller.InstallCommand(Tool{BinaryName: "agy", BinaryAliases: []string{"antigravity", "gemini", "gemini-cli"}})
	if len(cmd.Args) != 3 {
		t.Fatalf("unexpected args length: %v", cmd.Args)
	}
	if commandBase(cmd.Args[0]) != "bash" || cmd.Args[1] != "-lc" || cmd.Args[2] != "curl -fsSL https://antigravity.google/cli/install.sh | bash" {
		t.Fatalf("AntigravityInstaller.InstallCommand args = %v, want [bash -lc 'curl -fsSL https://antigravity.google/cli/install.sh | bash']", cmd.Args)
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
	if got := cargoCmd.Args; len(got) != 4 || commandBase(got[0]) != "cargo" || got[1] != "install" || got[2] != "uv" || got[3] != "--locked" {
		t.Fatalf("unexpected cargo command: %v", got)
	}

	goCmd := GoInstall.InstallCommand(Tool{Package: "go:github.com/some/tool/cmd/tool"})
	if got := goCmd.Args; len(got) != 3 || commandBase(got[0]) != "go" || got[1] != "install" || got[2] != "github.com/some/tool/cmd/tool@latest" {
		t.Fatalf("unexpected go install command: %v", got)
	}

	pipCmd := Pip.InstallCommand(Tool{Package: "pip:llm"})
	pythonWant := "python3"
	if runtime.GOOS == "windows" {
		pythonWant = "python"
	}
	if got := pipCmd.Args; len(got) != 6 || commandBase(got[0]) != pythonWant || got[1] != "-m" || got[2] != "pip" || got[3] != "install" || got[4] != "--upgrade" || got[5] != "llm" {
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

func TestBuiltInToolManagerSupportMatrix(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	wantByBinary := map[string]Manager{
		"claude":       Npm,
		"cursor-agent": CursorAgent,
		"opencode":     Npm,
		"codex":        Npm,
		"agy":          AntigravityInstaller,
		"copilot":      Npm,
	}

	for _, tool := range Tools {
		want, ok := wantByBinary[tool.BinaryName]
		if !ok {
			t.Fatalf("missing support-matrix expectation for %q", tool.BinaryName)
		}
		if got := ResolveManagerForInstall(tool); got != want {
			t.Fatalf("ResolveManagerForInstall(%s) = %q, want %q", tool.BinaryName, got, want)
		}
	}
}

func TestDetectManagerPrefersNpmWhenHomebrewBinPathIsAmbiguous(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "copilot" {
			return "/opt/homebrew/bin/copilot", nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		switch name {
		case "brew":
			if len(args) >= 1 && args[0] == "--prefix" {
				return []byte("/opt/homebrew\n"), nil
			}
			if len(args) >= 2 && args[0] == "list" && args[1] == "copilot-cli" {
				return []byte("Error: copilot-cli not installed\n"), errors.New("exit status 1")
			}
		case "npm":
			if len(args) >= 2 && args[0] == "prefix" && args[1] == "-g" {
				return []byte("/opt/homebrew\n"), nil
			}
			if len(args) >= 3 && args[0] == "list" {
				return []byte("@github/copilot@1.0.0\n"), nil
			}
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "@github/copilot", BinaryName: "copilot", BrewPackage: "copilot-cli"})
	if got != Npm {
		t.Fatalf("DetectManager() = %q, want %q", got, Npm)
	}
}

func TestDetectManagerUsesConfiguredBrewPackageForCopilotCask(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	binaryLookup = func(name string) (string, error) {
		if name == "copilot" {
			return "/opt/homebrew/bin/copilot", nil
		}
		return "", errExecutableNotFound
	}
	commandOutput = func(name string, args ...string) ([]byte, error) {
		switch name {
		case "brew":
			if len(args) >= 1 && args[0] == "--prefix" {
				return []byte("/opt/homebrew\n"), nil
			}
			if len(args) >= 2 && args[0] == "list" && args[1] == "copilot-cli" {
				return []byte("/opt/homebrew/Caskroom/copilot-cli/1.0.34/copilot\n"), nil
			}
		case "npm":
			if len(args) >= 2 && args[0] == "prefix" && args[1] == "-g" {
				return []byte("/opt/homebrew\n"), nil
			}
			if len(args) >= 3 && args[0] == "list" {
				return []byte("(empty)\n"), nil
			}
		}
		return nil, errExecutableNotFound
	}

	got := DetectManager(Tool{Package: "@github/copilot", BinaryName: "copilot", BrewPackage: "copilot-cli"})
	if got != Brew {
		t.Fatalf("DetectManager() = %q, want %q", got, Brew)
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
