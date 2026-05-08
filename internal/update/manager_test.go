package update

import "testing"

func TestDetectManagerCursorAgent(t *testing.T) {
	manager := DetectManager(Tool{
		Name:       "Cursor",
		Package:    "cursor-agent",
		BinaryName: "cursor-agent",
	})

	if manager != CursorAgent {
		t.Fatalf("DetectManager(cursor-agent) = %q, want %q", manager, CursorAgent)
	}
}

func TestCursorAgentInstallCommand(t *testing.T) {
	cmd := CursorAgent.InstallCommand(Tool{BinaryName: "cursor-agent"})
	if len(cmd.Args) != 2 {
		t.Fatalf("unexpected args length: %v", cmd.Args)
	}
	if cmd.Args[0] != "cursor-agent" || cmd.Args[1] != "update" {
		t.Fatalf("CursorAgent.InstallCommand args = %v, want [cursor-agent update]", cmd.Args)
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
