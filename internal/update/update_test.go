package update

import (
	"testing"
)

func TestToolFiltering(t *testing.T) {
	enabledTools := []string{"gemini", "cursor-agent"}

	var toolsToUpdate []Tool
	for _, et := range enabledTools {
		for _, tool := range Tools {
			if et == tool.BinaryName {
				toolsToUpdate = append(toolsToUpdate, tool)
				break
			}
		}
	}

	if len(toolsToUpdate) != 2 {
		t.Errorf("Expected 2 tools to be enabled, got %d", len(toolsToUpdate))
	}

	if toolsToUpdate[0].BinaryName != "gemini" {
		t.Errorf("Expected gemini to be enabled, got %s", toolsToUpdate[0].BinaryName)
	}
	if toolsToUpdate[1].BinaryName != "cursor-agent" {
		t.Errorf("Expected cursor-agent to be enabled, got %s", toolsToUpdate[1].BinaryName)
	}
}
