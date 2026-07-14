package ui

import (
	"os"
	"testing"
)

func TestIconSlug(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "ClaudeCode", want: "claudecode"},
		{in: "GitHub Copilot", want: "githubcopilot"},
		{in: "gemini-cli", want: "geminicli"},
		{in: "  ", want: ""},
	}

	for _, tc := range tests {
		got := iconSlug(tc.in)
		if got != tc.want {
			t.Fatalf("iconSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestGetEmbeddedIconPath(t *testing.T) {
	got := getEmbeddedIconPath("Codex")
	want := "assets/icons/codex.png"
	if got != want {
		t.Fatalf("getEmbeddedIconPath(Codex) = %q, want %q", got, want)
	}
}

func TestDetectRendererChoiceFromEnv(t *testing.T) {
	prev := os.Getenv("OCT_ICON_RENDERER")
	t.Cleanup(func() { _ = os.Setenv("OCT_ICON_RENDERER", prev) })

	_ = os.Setenv("OCT_ICON_RENDERER", "text")
	if got := detectRendererChoice(); got != rendererText {
		t.Fatalf("detectRendererChoice() = %q, want %q", got, rendererText)
	}
}
