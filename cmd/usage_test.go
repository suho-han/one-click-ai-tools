package cmd

import (
	"testing"

	"github.com/spf13/viper"
)

func TestShouldAutoJSONFallback(t *testing.T) {
	tests := []struct {
		name     string
		jsonMode bool
		isTTY    bool
		want     bool
	}{
		{name: "json already requested", jsonMode: true, isTTY: false, want: false},
		{name: "tty and no json flag", jsonMode: false, isTTY: true, want: false},
		{name: "non tty and no json flag", jsonMode: false, isTTY: false, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAutoJSONFallback(tc.jsonMode, tc.isTTY)
			if got != tc.want {
				t.Fatalf("shouldAutoJSONFallback(%v, %v) = %v, want %v", tc.jsonMode, tc.isTTY, got, tc.want)
			}
		})
	}
}

func TestUsageOrderedTools_RespectsEnabledTools(t *testing.T) {
	oldOrder := viper.GetStringSlice("agent_order")
	oldEnabled := viper.GetStringSlice("enabled_tools")
	t.Cleanup(func() {
		viper.Set("agent_order", oldOrder)
		viper.Set("enabled_tools", oldEnabled)
	})

	viper.Set("agent_order", []string{"gemini", "claude", "cursor-agent", "copilot", "opencode", "codex"})
	viper.Set("enabled_tools", []string{"codex", "opencode"})

	tools := usageOrderedTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].BinaryName != "codex" || tools[1].BinaryName != "opencode" {
		t.Fatalf("unexpected tool order: %s, %s", tools[0].BinaryName, tools[1].BinaryName)
	}
}
