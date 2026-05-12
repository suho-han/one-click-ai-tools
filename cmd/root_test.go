package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommand(t *testing.T) {
	root := rootCmd
	b := bytes.NewBufferString("")
	root.SetOut(b)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("root.Execute() failed: %v", err)
	}

	out := b.String()
	if !contains(out, "A high-performance CLI tool") {
		t.Errorf("expected help message to contain description, got: %s", out)
	}
	if !contains(out, "agent-update") {
		t.Errorf("expected help to include agent-update command, got: %s", out)
	}
	if !contains(out, "update") {
		t.Errorf("expected help to include update command, got: %s", out)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
