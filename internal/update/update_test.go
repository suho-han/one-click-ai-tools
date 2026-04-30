package update

import (
	"testing"
)

func TestToolFiltering(t *testing.T) {
	enabledTools := []string{"gemini"}
	
	var toolsToUpdate []Tool
	for _, et := range enabledTools {
		for _, tool := range Tools {
			if et == tool.BinaryName {
				toolsToUpdate = append(toolsToUpdate, tool)
				break
			}
		}
	}

	if len(toolsToUpdate) != 1 {
		t.Errorf("Expected 1 tool to be enabled, got %d", len(toolsToUpdate))
	}

	if toolsToUpdate[0].BinaryName != "gemini" {
		t.Errorf("Expected gemini to be enabled, got %s", toolsToUpdate[0].BinaryName)
	}
}
