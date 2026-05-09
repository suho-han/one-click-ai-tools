package update

import (
	"testing"
)

func TestToolFiltering(t *testing.T) {
	ordered := GetOrderedTools(nil)

	result := GetFilteredTools([]string{"gemini"}, ordered)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].BinaryName != "gemini" {
		t.Fatalf("expected gemini, got %s", result[0].BinaryName)
	}
}

func TestToolFilteringCaseInsensitive(t *testing.T) {
	ordered := GetOrderedTools(nil)

	result := GetFilteredTools([]string{"Gemini"}, ordered)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool for 'Gemini', got %d", len(result))
	}
	if result[0].BinaryName != "gemini" {
		t.Fatalf("expected gemini, got %s", result[0].BinaryName)
	}
}

func TestToolFilteringMultiple(t *testing.T) {
	ordered := GetOrderedTools(nil)

	result := GetFilteredTools([]string{"gemini", "cursor-agent"}, ordered)
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
	if result[0].BinaryName != "gemini" {
		t.Fatalf("expected gemini, got %s", result[0].BinaryName)
	}
	if result[1].BinaryName != "cursor-agent" {
		t.Fatalf("expected cursor-agent, got %s", result[1].BinaryName)
	}
}

func TestToolFilteringEmpty(t *testing.T) {
	ordered := GetOrderedTools(nil)

	result := GetFilteredTools([]string{}, ordered)
	if len(result) != len(Tools) {
		t.Fatalf("expected all %d tools when enabled is empty, got %d", len(Tools), len(result))
	}
}

func TestIsNpmPermissionError(t *testing.T) {
	if !isNpmPermissionError("npm ERR! code EACCES\nnpm ERR! permission denied") {
		t.Fatal("expected EACCES output to be detected as npm permission error")
	}
	if isNpmPermissionError("npm ERR! 404 Not Found") {
		t.Fatal("did not expect unrelated npm error to be treated as permission error")
	}
}

func TestIsNpmConflictError(t *testing.T) {
	if !isNpmConflictError("npm ERR! code EEXIST\nnpm ERR! File exists: /home/me/.local/bin/claude") {
		t.Fatal("expected EEXIST output to be detected as conflict")
	}
	if !isNpmConflictError("npm ERR! code ENOTEMPTY\nnpm ERR! ENOTEMPTY: directory not empty, rename '/a' -> '/b'") {
		t.Fatal("expected ENOTEMPTY output to be detected as conflict")
	}
	if isNpmConflictError("npm ERR! 404 Not Found") {
		t.Fatal("did not expect unrelated npm error to be treated as conflict")
	}
}

func TestExtractNpmDestPath(t *testing.T) {
	output := "npm ERR! code ENOTEMPTY\nnpm ERR! dest /home/me/.local/lib/node_modules/@github/.copilot-abc123\n"
	if got := extractNpmDestPath(output); got != "/home/me/.local/lib/node_modules/@github/.copilot-abc123" {
		t.Fatalf("extractNpmDestPath() = %q", got)
	}
}

func TestShouldRemoveNpmConflictDest(t *testing.T) {
	home := "/home/me"

	if !shouldRemoveNpmConflictDest("/home/me/.local/lib/node_modules/@github/.copilot-abc123", home) {
		t.Fatal("expected local npm temp directory to be removable")
	}
	if shouldRemoveNpmConflictDest("/usr/local/lib/node_modules/@github/.copilot-abc123", home) {
		t.Fatal("did not expect global directory to be removable")
	}
	if shouldRemoveNpmConflictDest("/home/me/.local/lib/node_modules/@github/copilot", home) {
		t.Fatal("did not expect non-temp package directory to be removable")
	}
}
