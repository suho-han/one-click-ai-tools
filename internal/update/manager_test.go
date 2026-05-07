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
