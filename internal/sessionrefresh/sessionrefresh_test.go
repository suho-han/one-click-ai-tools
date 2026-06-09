package sessionrefresh

import (
	"testing"

	"github.com/suho-han/one-click-tools/internal/update"
)

func TestRefreshUnknownProvider(t *testing.T) {
	results := Refresh(RefreshOptions{Providers: []string{"nope"}, DryRun: true})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "unsupported" {
		t.Fatalf("expected unsupported, got %#v", results[0])
	}
}

func TestRefreshAliasMapsGeminiToAntigravity(t *testing.T) {
	results := Refresh(RefreshOptions{Providers: []string{"gemini"}, DryRun: true})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Provider != "agy" {
		t.Fatalf("expected gemini alias to resolve to agy, got %#v", results[0])
	}
}

func TestResolveToolMatchesAlias(t *testing.T) {
	tool, ok := resolveTool(update.NormalizeToolName("agent"))
	if !ok {
		t.Fatal("expected to resolve Cursor tool from agent alias")
	}
	if tool.BinaryName != "cursor-agent" {
		t.Fatalf("expected cursor-agent, got %q", tool.BinaryName)
	}
}

func TestFirstNonEmptyLine(t *testing.T) {
	if got := firstNonEmptyLine("\n hello\nworld"); got != "hello" {
		t.Fatalf("unexpected first non-empty line: %q", got)
	}
}
