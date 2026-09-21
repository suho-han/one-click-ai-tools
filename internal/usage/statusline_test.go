package usage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleStatuslineResults() []UsageResult {
	return []UsageResult{
		{
			Provider: "codex",
			Unit:     "percent",
			Used:     "80.0",
			Status:   "ok",
			Buckets:  map[string]string{"5h": "80.0", "7d": "55.0"},
		},
		{
			Provider: "claude",
			Unit:     "percent",
			Used:     "12.0",
			Status:   "ok",
			Buckets:  map[string]string{"5h": "12.0"},
		},
	}
}

func TestUsageSeverityThresholds(t *testing.T) {
	tests := []struct {
		name   string
		result UsageResult
		want   string
	}{
		{name: "error status", result: UsageResult{Status: "error", Unit: "percent", Used: "1"}, want: "CRIT"},
		{name: "warn status", result: UsageResult{Status: "warn", Unit: "percent", Used: "1"}, want: "WARN"},
		{name: "ok low", result: UsageResult{Status: "ok", Unit: "percent", Used: "10"}, want: "OK"},
		{name: "warn threshold", result: UsageResult{Status: "ok", Unit: "percent", Used: "85"}, want: "WARN"},
		{name: "crit threshold", result: UsageResult{Status: "ok", Unit: "percent", Used: "95"}, want: "CRIT"},
		{name: "bucket escalates", result: UsageResult{Status: "ok", Unit: "percent", Used: "1", Buckets: map[string]string{"7d": "90"}}, want: "WARN"},
		{name: "non-percent unknown", result: UsageResult{Status: "ok", Unit: "USD", Used: "3.50"}, want: "UNKNOWN"},
		{name: "dataless unknown", result: UsageResult{Status: "ok", Unit: "percent"}, want: "UNKNOWN"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := UsageSeverity(tc.result); got != tc.want {
				t.Fatalf("UsageSeverity() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStatuslineSeverityFoldsToBarClass(t *testing.T) {
	ok := UsageResult{Provider: "codex", Status: "ok", Unit: "percent", Used: "10"}
	warn := UsageResult{Provider: "claude", Status: "ok", Unit: "percent", Used: "88"}
	crit := UsageResult{Provider: "kimi", Status: "error"}
	unknown := UsageResult{Provider: "copilot", Status: "ok", Unit: "msgs", Used: "12"}

	tests := []struct {
		name    string
		results []UsageResult
		want    string
	}{
		{name: "empty is ok", results: nil, want: "ok"},
		{name: "all ok", results: []UsageResult{ok, unknown}, want: "ok"},
		{name: "warn escalates", results: []UsageResult{ok, warn}, want: "warn"},
		{name: "crit wins", results: []UsageResult{ok, warn, crit}, want: "error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := StatuslineSeverity(tc.results); got != tc.want {
				t.Fatalf("StatuslineSeverity() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderWaybarJSON(t *testing.T) {
	var buf bytes.Buffer
	results := sampleStatuslineResults()
	if err := RenderWaybarJSON(&buf, results, DisplayModeRemaining); err != nil {
		t.Fatalf("RenderWaybarJSON: %v", err)
	}

	var payload struct {
		Text    string `json:"text"`
		Tooltip string `json:"tooltip"`
		Class   string `json:"class"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode waybar payload: %v\nraw: %s", err, buf.String())
	}
	// Remaining mode: codex compact value comes from the 7d bucket (55 used ->
	// 45 left), claude from 5h (12 used -> 88 left).
	if payload.Text != "X-45% C-88%" {
		t.Fatalf("text = %q, want %q", payload.Text, "X-45% C-88%")
	}
	// Severity reads the raw used values (80/12), not the inverted display.
	if payload.Class != "ok" {
		t.Fatalf("class = %q, want ok", payload.Class)
	}
	if !strings.Contains(payload.Tooltip, "[ok] codex") || !strings.Contains(payload.Tooltip, "5h 20.0%") {
		t.Fatalf("tooltip = %q, want provider lines with inverted buckets", payload.Tooltip)
	}
	if strings.Contains(payload.Tooltip, "\n\n") {
		t.Fatalf("tooltip = %q, want one line per provider", payload.Tooltip)
	}
}

func TestRenderWaybarJSONWarnThresholdEscalatesClass(t *testing.T) {
	var buf bytes.Buffer
	results := []UsageResult{
		{Provider: "codex", Status: "ok", Unit: "percent", Used: "90.0", Buckets: map[string]string{"7d": "90.0"}},
	}
	if err := RenderWaybarJSON(&buf, results, DisplayModeUsed); err != nil {
		t.Fatalf("RenderWaybarJSON: %v", err)
	}
	var payload struct{ Class string }
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Class != "warn" {
		t.Fatalf("class = %q, want warn", payload.Class)
	}
}

func TestRenderWaybarJSONErrorEscalatesClass(t *testing.T) {
	var buf bytes.Buffer
	results := []UsageResult{{Provider: "kimi", Status: "error", Unit: "percent", Used: "n/a"}}
	if err := RenderWaybarJSON(&buf, results, DisplayModeUsed); err != nil {
		t.Fatalf("RenderWaybarJSON: %v", err)
	}
	var payload struct{ Class string }
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Class != "error" {
		t.Fatalf("class = %q, want error", payload.Class)
	}
}

func TestRenderPolybarLineEscapesPercentAndColorsSeverity(t *testing.T) {
	var buf bytes.Buffer
	results := []UsageResult{
		{Provider: "codex", Unit: "percent", Used: "20.0", Status: "ok"},
		{Provider: "claude", Unit: "percent", Used: "90.0", Status: "ok"},
	}
	if err := RenderPolybarLine(&buf, results, DisplayModeUsed); err != nil {
		t.Fatalf("RenderPolybarLine: %v", err)
	}
	out := buf.String()
	// Literal "%" must be doubled for polybar; the 90%-used token carries the
	// warn color while the ok token stays unstyled.
	if !strings.Contains(out, `X-20%%`) {
		t.Fatalf("output = %q, want escaped percent on ok token", out)
	}
	if !strings.Contains(out, "%{F"+severityColorWarn+"}C-90%%%{F-}") {
		t.Fatalf("output = %q, want warn-colored used token", out)
	}
	if strings.Count(out, "%{F-}") != 1 {
		t.Fatalf("output = %q, want exactly one color reset", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("output = %q, want no ANSI escapes", out)
	}
}

func TestRenderSwiftBarProtocol(t *testing.T) {
	var buf bytes.Buffer
	results := sampleStatuslineResults()
	if err := RenderSwiftBar(&buf, results, DisplayModeUsed); err != nil {
		t.Fatalf("RenderSwiftBar: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2+len(results) {
		t.Fatalf("line count = %d, want title + separator + %d providers:\n%s", len(lines), len(results), buf.String())
	}
	if !strings.HasPrefix(lines[0], "X-55% C-12% | color=") {
		t.Fatalf("title line = %q, want compact title with color param", lines[0])
	}
	if lines[1] != "---" {
		t.Fatalf("separator = %q, want ---", lines[1])
	}
	for i, line := range lines[2:] {
		wantPrefix := []string{"[ok] codex", "[ok] claude"}[i]
		if !strings.HasPrefix(line, wantPrefix) || !strings.Contains(line, "color=") {
			t.Fatalf("provider line %d = %q, want prefix %q with color param", i, line, wantPrefix)
		}
	}
}

func TestStatuslineRenderersHandleEmptyResults(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderWaybarJSON(&buf, nil, DisplayModeUsed); err != nil {
		t.Fatalf("waybar empty: %v", err)
	}
	if !strings.Contains(buf.String(), `"text":"-"`) {
		t.Fatalf("waybar empty output = %q, want placeholder text", buf.String())
	}

	buf.Reset()
	if err := RenderPolybarLine(&buf, nil, DisplayModeUsed); err != nil {
		t.Fatalf("polybar empty: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "-" {
		t.Fatalf("polybar empty output = %q, want -", buf.String())
	}

	buf.Reset()
	if err := RenderSwiftBar(&buf, nil, DisplayModeUsed); err != nil {
		t.Fatalf("swiftbar empty: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "oct | color=") {
		t.Fatalf("swiftbar empty output = %q, want oct fallback title", buf.String())
	}
}

func TestLoadSnapshotRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage-latest.json")
	results := sampleStatuslineResults()
	if err := SaveSnapshot(path, results, time.Now()); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	snapshot, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if len(snapshot.Results) != len(results) {
		t.Fatalf("results = %d, want %d", len(snapshot.Results), len(results))
	}
	for i, want := range results {
		if snapshot.Results[i].Provider != want.Provider || snapshot.Results[i].Used != want.Used {
			t.Fatalf("result %d = %+v, want provider %s used %s", i, snapshot.Results[i], want.Provider, want.Used)
		}
	}
	if snapshot.CapturedAt == "" {
		t.Fatalf("captured_at empty after round trip")
	}
}

func TestLoadSnapshotErrorsOnMissingOrCorruptFile(t *testing.T) {
	if _, err := LoadSnapshot(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatalf("missing snapshot: want error")
	}
	corrupt := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(corrupt, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSnapshot(corrupt); err == nil || !strings.Contains(err.Error(), "parse snapshot") {
		t.Fatalf("corrupt snapshot err = %v, want parse error", err)
	}
}

func TestStatuslineFiltersUnconfiguredProviders(t *testing.T) {
	usable := UsageResult{Provider: "codex", Status: "ok", Unit: "percent", Used: "80.0", Buckets: map[string]string{"7d": "55.0"}}
	// cursor/qwen/minimax-shaped rows: warn status, no usable percentage.
	noAuth := UsageResult{Provider: "cursor-agent", Status: "warn", Unit: "sessions", Used: "0", Message: "No Cursor auth token found"}
	emptyWindow := UsageResult{Provider: "kimi", Status: "warn", Unit: "percent", Message: "Kimi API returned no usage data"}
	// A hard failure carries no number either but must stay visible.
	authError := UsageResult{Provider: "copilot", Status: "error", Unit: "requests", Used: "n/a", Message: "Invalid API Token (HTTP 401)"}
	results := []UsageResult{usable, noAuth, emptyWindow, authError}

	var buf bytes.Buffer
	if err := RenderWaybarJSON(&buf, results, DisplayModeUsed); err != nil {
		t.Fatalf("RenderWaybarJSON: %v", err)
	}
	var payload struct {
		Text    string `json:"text"`
		Tooltip string `json:"tooltip"`
		Class   string `json:"class"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// codex's compact value comes from the 7d bucket (55); copilot stays as
	// the visible "?"-token error.
	if payload.Text != "X-55% P-?" {
		t.Fatalf("text = %q, want usable + error tokens only", payload.Text)
	}
	if strings.Contains(payload.Tooltip, "cursor-agent") || strings.Contains(payload.Tooltip, "kimi") {
		t.Fatalf("tooltip = %q, want no-data providers hidden", payload.Tooltip)
	}
	if !strings.Contains(payload.Tooltip, "[error] copilot") {
		t.Fatalf("tooltip = %q, want the actionable error kept", payload.Tooltip)
	}
	if !strings.Contains(payload.Tooltip, "2 provider(s) hidden") {
		t.Fatalf("tooltip = %q, want hidden-provider note", payload.Tooltip)
	}
	// The informational warns must not escalate the bar; the real error must.
	if payload.Class != "error" {
		t.Fatalf("class = %q, want error (auth failure kept)", payload.Class)
	}

	// Without the failure, only the usable provider remains and class is ok.
	buf.Reset()
	if err := RenderWaybarJSON(&buf, []UsageResult{usable, noAuth, emptyWindow}, DisplayModeUsed); err != nil {
		t.Fatalf("RenderWaybarJSON: %v", err)
	}
	payload = struct {
		Text    string `json:"text"`
		Tooltip string `json:"tooltip"`
		Class   string `json:"class"`
	}{}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Text != "X-55%" || payload.Class != "ok" {
		t.Fatalf("text/class = %q/%q, want clean ok bar", payload.Text, payload.Class)
	}
	if !strings.Contains(payload.Tooltip, "2 provider(s) hidden") {
		t.Fatalf("tooltip = %q, want hidden-provider note", payload.Tooltip)
	}
}

func TestRenderSwiftBarHidesNoDataProviders(t *testing.T) {
	var buf bytes.Buffer
	results := []UsageResult{
		{Provider: "codex", Status: "ok", Unit: "percent", Used: "55.0", Buckets: map[string]string{"7d": "55.0"}},
		{Provider: "minimax", Status: "warn", Unit: "percent", Message: "No data: MiniMax API token not found"},
	}
	if err := RenderSwiftBar(&buf, results, DisplayModeUsed); err != nil {
		t.Fatalf("RenderSwiftBar: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "minimax") {
		t.Fatalf("swiftbar output = %q, want minimax hidden", out)
	}
	if !strings.Contains(out, "1 provider(s) hidden") {
		t.Fatalf("swiftbar output = %q, want hidden-provider note", out)
	}
}

func TestRenderPolybarHidesNoDataProviders(t *testing.T) {
	var buf bytes.Buffer
	results := []UsageResult{
		{Provider: "codex", Status: "ok", Unit: "percent", Used: "55.0", Buckets: map[string]string{"7d": "55.0"}},
		{Provider: "qwen", Status: "warn", Unit: "percent", Message: "No data"},
	}
	if err := RenderPolybarLine(&buf, results, DisplayModeUsed); err != nil {
		t.Fatalf("RenderPolybarLine: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != `X-55%%` {
		t.Fatalf("polybar output = %q, want only the usable token", out)
	}
}
