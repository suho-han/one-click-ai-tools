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
