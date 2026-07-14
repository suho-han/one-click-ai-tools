package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestPrintJSON_SummarySchemaAndCounts(t *testing.T) {
	results := []UsageResult{
		{Provider: "alpha", Plan: "plus", PlanSource: "codex auth.jwt id_token", Status: "ok", Used: "10"},
		{Provider: "beta", Status: "ok", Used: "n/a"},                                          // warn: no numeric usage
		{Provider: "cursor", Status: "ok", Used: "0", Message: "No data: No local logs found"}, // warn: zero with not-found signal
		{Provider: "delta", Status: "warn", Used: "0", Message: "Partial: weekly bucket only"},
		{Provider: "gamma", Status: "error", Used: "n/a"},
	}

	output := captureStdout(t, func() {
		if err := PrintJSON(results); err != nil {
			t.Fatalf("PrintJSON returned error: %v", err)
		}
	})

	var payload struct {
		Summary struct {
			Total int `json:"total"`
			OK    int `json:"ok"`
			Warn  int `json:"warn"`
			Error int `json:"error"`
		} `json:"summary"`
		Results []map[string]any `json:"results"`
	}

	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("failed to parse PrintJSON output: %v\noutput=%s", err, output)
	}

	if payload.Summary.Total != 5 || payload.Summary.OK != 1 || payload.Summary.Warn != 3 || payload.Summary.Error != 1 {
		t.Fatalf("unexpected summary counts: %+v", payload.Summary)
	}
	if len(payload.Results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(payload.Results))
	}
	if payload.Results[0]["provider"] != "alpha" || payload.Results[1]["provider"] != "beta" || payload.Results[2]["provider"] != "cursor" || payload.Results[3]["provider"] != "delta" || payload.Results[4]["provider"] != "gamma" {
		t.Fatalf("result order changed unexpectedly: %+v", payload.Results)
	}
	if _, ok := payload.Results[0]["period"]; ok {
		t.Fatalf("compact JSON should omit verbose fields like period: %+v", payload.Results[0])
	}
	if payload.Results[0]["plan"] != "plus" {
		t.Fatalf("expected compact JSON to include plan, got %+v", payload.Results[0])
	}
	if payload.Results[0]["plan_source"] != "codex auth.jwt id_token" {
		t.Fatalf("expected compact JSON to include plan_source, got %+v", payload.Results[0])
	}
}

func TestClassifySummaryStatus(t *testing.T) {
	tests := []struct {
		name string
		in   UsageResult
		want string
	}{
		{name: "ok numeric usage", in: UsageResult{Status: "ok", Used: "12"}, want: "ok"},
		{name: "ok but na usage", in: UsageResult{Status: "ok", Used: "n/a"}, want: "warn"},
		{name: "ok but zero with no-data message", in: UsageResult{Status: "ok", Used: "0", Message: "No data: No local logs found"}, want: "warn"},
		{name: "ok but partial message", in: UsageResult{Status: "ok", Used: "22", Message: "Partial: weekly bucket only"}, want: "warn"},
		{name: "ok zero real value", in: UsageResult{Status: "ok", Used: "0", Message: "Fetched from Cursor API"}, want: "ok"},
		{name: "explicit warn", in: UsageResult{Status: "warn", Used: "0"}, want: "warn"},
		{name: "error", in: UsageResult{Status: "error", Used: "0"}, want: "error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifySummaryStatus(tc.in)
			if got != tc.want {
				t.Fatalf("classifySummaryStatus(%+v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestUsageParsingLogic(t *testing.T) {
	jsonData := `{"five_hour": {"utilization": 42.5}}`

	var data struct {
		FiveHour struct {
			Utilization float64 `json:"utilization"`
		} `json:"five_hour"`
	}

	err := json.Unmarshal([]byte(jsonData), &data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if data.FiveHour.Utilization != 42.5 {
		t.Errorf("Expected 42.5, got %f", data.FiveHour.Utilization)
	}
}

func TestMockServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"five_hour": {"utilization": 42.5}}`)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		FiveHour struct {
			Utilization float64 `json:"utilization"`
		} `json:"five_hour"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if data.FiveHour.Utilization != 42.5 {
		t.Errorf("Expected 42.5, got %f", data.FiveHour.Utilization)
	}
}

func TestSelectedTools_RespectsEnabledTools(t *testing.T) {
	oldOrder := viper.GetStringSlice("agent_order")
	oldEnabled := viper.GetStringSlice("enabled_tools")
	t.Cleanup(func() {
		viper.Set("agent_order", oldOrder)
		viper.Set("enabled_tools", oldEnabled)
	})

	viper.Set("agent_order", []string{"agy", "claude", "cursor-agent", "copilot", "opencode", "codex"})
	viper.Set("enabled_tools", []string{"codex", "opencode"})

	selected := SelectedTools()
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected tools, got %d", len(selected))
	}
	if selected[0].BinaryName != "codex" || selected[1].BinaryName != "opencode" {
		t.Fatalf("unexpected selected order: %s, %s", selected[0].BinaryName, selected[1].BinaryName)
	}
}

func TestColorizeHelpers_DarkTerminalFriendly(t *testing.T) {
	provider := colorizeProvider("cursor", "cursor")
	if !strings.Contains(provider, "\x1b[1;94m") {
		t.Fatalf("expected bright ANSI code for provider, got %q", provider)
	}

	status := colorizeStatus("warn", "warn")
	if !strings.Contains(status, "\x1b[1;93m") {
		t.Fatalf("expected bright yellow for warn status, got %q", status)
	}

	msg := colorizeMessage("something happened", "error")
	if !strings.Contains(msg, "\x1b[91m") {
		t.Fatalf("expected red message tint for error, got %q", msg)
	}
}

func TestTableWidthHelpers(t *testing.T) {
	if got := tableMessageWidth(80); got != 10 {
		t.Fatalf("expected width 10 at 80 cols, got %d", got)
	}
	if got := tableMessageWidth(100); got != 20 {
		t.Fatalf("expected width 20 at 100 cols, got %d", got)
	}
	if got := truncateText("abcdefghijklmnopqrstuvwxyz", 10); got != "abcdefg..." {
		t.Fatalf("unexpected truncateText result: %q", got)
	}
}

func TestProviderDisplayLabel_IconCapability(t *testing.T) {
	t.Setenv("OCT_NO_ICONS", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")

	if got := providerDisplayLabel("Cursor"); got != "▣ Cursor" {
		t.Fatalf("expected cursor icon label, got %q", got)
	}
	if got := providerDisplayLabel("Antigravity"); got != "✨ Antigravity" {
		t.Fatalf("expected antigravity icon label, got %q", got)
	}
	if got := providerDisplayLabel("OpenCode"); got != "🧩 OpenCode" {
		t.Fatalf("expected opencode icon label, got %q", got)
	}

	t.Setenv("TERM", "dumb")
	if got := providerDisplayLabel("Cursor"); got != "Cursor" {
		t.Fatalf("expected plain provider for dumb term, got %q", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read pipe failed: %v", err)
	}
	_ = r.Close()
	return strings.TrimSpace(buf.String())
}
