package analyticsserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const installation = "11111111-1111-4111-8111-111111111111"

func sampleEvent(kind, invocation, when string) map[string]any {
	props := map[string]any{
		"schema_version": 1, "command": "oct alert config show", "source": "cli",
		"version": "1.2.3", "os": "darwin", "arch": "arm64", "invocation_id": invocation,
	}
	if kind == "oct_command_finished" {
		props["success"] = true
		props["duration_ms"] = 42
	}
	return map[string]any{"event": kind, "distinct_id": installation, "timestamp": when, "properties": props}
}

func payload(t *testing.T, event map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func post(t *testing.T, handler http.Handler, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "private-sentinel-ip:49152"
	r.Header.Set("X-Forwarded-For", "private-sentinel-forwarded")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func newService(t *testing.T) (*Service, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("analytics server requires POSIX private file permissions")
	}
	path := filepath.Join(t.TempDir(), "analytics.sqlite")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestIngestStrictAllowlistAndPrivacy(t *testing.T) {
	s, path := newService(t)
	when := time.Now().UTC().Format(time.RFC3339)
	base := sampleEvent("oct_command_started", "22222222-2222-4222-8222-222222222222", when)
	if w := post(t, s.IngestHandler(), payload(t, base)); w.Code != http.StatusAccepted {
		t.Fatalf("valid: %d", w.Code)
	}

	cases := map[string]func(map[string]any){
		"extra raw body":       func(e map[string]any) { e["raw_body"] = "secret-sentinel" },
		"extra IP":             func(e map[string]any) { e["ip"] = "private-sentinel-ip" },
		"old api key":          func(e map[string]any) { e["api_key"] = "secret-sentinel" },
		"unknown property":     func(e map[string]any) { e["properties"].(map[string]any)["prompt"] = "secret-sentinel" },
		"posthog property":     func(e map[string]any) { e["properties"].(map[string]any)["$process_person_profile"] = false },
		"success on start":     func(e map[string]any) { e["properties"].(map[string]any)["success"] = false },
		"args in command":      func(e map[string]any) { e["properties"].(map[string]any)["command"] = "oct alert --token abc" },
		"invalid installation": func(e map[string]any) { e["distinct_id"] = "email@example.com" },
		"invalid source":       func(e map[string]any) { e["properties"].(map[string]any)["source"] = "private-sentinel" },
		"missing command":      func(e map[string]any) { delete(e["properties"].(map[string]any), "command") },
		"case-insensitive key": func(e map[string]any) { e["Event"] = e["event"]; delete(e, "event") },
		"stale timestamp": func(e map[string]any) {
			e["timestamp"] = time.Now().Add(-91 * 24 * time.Hour).UTC().Format(time.RFC3339)
		},
		"future timestamp": func(e map[string]any) {
			e["timestamp"] = time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)
		},
	}
	for name, modify := range cases {
		t.Run(name, func(t *testing.T) {
			e := sampleEvent("oct_command_started", "33333333-3333-4333-8333-333333333333", when)
			modify(e)
			w := post(t, s.IngestHandler(), payload(t, e))
			if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), "sentinel") {
				t.Fatalf("status %d body %q", w.Code, w.Body.String())
			}
		})
	}
	for _, body := range [][]byte{
		[]byte(`{"event":"oct_command_started","event":"oct_command_finished"}`),
		[]byte(strings.Repeat("x", maxEventBytes+1)),
		[]byte(`null`),
		[]byte(`{"event":null}`),
	} {
		if w := post(t, s.IngestHandler(), body); w.Code != http.StatusBadRequest {
			t.Fatalf("malformed status %d", w.Code)
		}
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM events").Scan(&count); err != nil || count != 1 {
		t.Fatalf("stored count %d: %v", count, err)
	}
	var columns []string
	rows, err := s.db.Query("PRAGMA table_info(events)")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, name)
	}
	rows.Close()
	for _, col := range columns {
		if col == "ip" || col == "headers" || col == "body" || col == "user_agent" {
			t.Fatalf("unsafe column: %s", col)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-sentinel-ip", "private-sentinel-forwarded", "secret-sentinel"} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatalf("persisted request metadata: %s", secret)
		}
	}
}

func TestPersistenceDedupeAndTimestamp(t *testing.T) {
	s, path := newService(t)
	when := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	id := "44444444-4444-4444-8444-444444444444"
	started := payload(t, sampleEvent("oct_command_started", id, when))
	finished := payload(t, sampleEvent("oct_command_finished", id, when))
	for _, b := range [][]byte{finished, started, started, finished} {
		w := post(t, s.IngestHandler(), b)
		if w.Code != http.StatusAccepted && w.Code != http.StatusNoContent {
			t.Fatalf("ingest: %d", w.Code)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	other, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	rows, err := other.db.Query("SELECT timestamp FROM events ORDER BY event")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != when {
			t.Fatalf("timestamp changed: %s", stored)
		}
		count++
	}
	rows.Close()
	if count != 2 {
		t.Fatalf("dedupe failed: %d", count)
	}
}

func TestInstallationSelectionsAndDeviceAreLatestAllowlistedObservations(t *testing.T) {
	s, path := newService(t)
	now := time.Now().UTC().Truncate(time.Second)
	send := func(invocation, id string, when time.Time, selected []string, device string) int {
		t.Helper()
		e := sampleEvent("oct_command_started", invocation, when.Format(time.RFC3339))
		e["distinct_id"] = id
		props := e["properties"].(map[string]any)
		props["schema_version"] = 2
		props["providers"] = selected
		props["device_family"] = device
		return post(t, s.IngestHandler(), payload(t, e)).Code
	}
	first := "aaaaaaaa-0000-4000-8000-000000000001"
	second := "aaaaaaaa-0000-4000-8000-000000000002"
	if code := send(first, installation, now.Add(-time.Minute), []string{"codex", "claude"}, "Mac mini"); code != http.StatusAccepted {
		t.Fatalf("first observation: %d", code)
	}
	if code := send(second, installation, now, []string{"codex"}, "MacBook Air"); code != http.StatusAccepted {
		t.Fatalf("updated observation: %d", code)
	}
	if code := send(first, installation, now.Add(-time.Minute), []string{"codex", "claude"}, "Mac mini"); code != http.StatusNoContent {
		t.Fatalf("duplicate observation: %d", code)
	}
	if code := send("aaaaaaaa-0000-4000-8000-000000000003", installation, now.Add(-30*time.Second), []string{"claude"}, "Mac mini"); code != http.StatusAccepted {
		t.Fatalf("older observation: %d", code)
	}
	if code := send("aaaaaaaa-0000-4000-8000-000000000004", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", now, []string{"codex", "claude"}, "Mac mini"); code != http.StatusAccepted {
		t.Fatalf("second installation: %d", code)
	}
	for _, invalid := range []struct {
		providers []string
		device    string
	}{
		{[]string{"codex:personal"}, "Mac mini"},
		{[]string{"codex", "codex"}, "Mac mini"},
		{[]string{"private@example.com"}, "Mac mini"},
		{nil, "Mac mini"},
		{[]string{"codex"}, "MacBookPro18,1"},
		{[]string{"codex"}, "user-hostname"},
	} {
		if code := send("aaaaaaaa-0000-4000-8000-000000000005", installation, now, invalid.providers, invalid.device); code != http.StatusBadRequest {
			t.Fatalf("unsafe observation accepted: %d %#v", code, invalid)
		}
	}
	var selected, device string
	if err := s.db.QueryRow("SELECT providers, device_family FROM installations WHERE distinct_id=?", installation).Scan(&selected, &device); err != nil || selected != `["codex"]` || device != "MacBook Air" {
		t.Fatalf("latest observation %q %q %v", selected, device, err)
	}
	stats, err := s.statistics()
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Providers) != 2 || stats.Providers[0].Period != "codex" || stats.Providers[0].Count != 2 ||
		stats.Providers[1].Period != "claude" || stats.Providers[1].Count != 1 {
		t.Fatalf("provider installation ranking: %#v", stats.Providers)
	}
	if len(stats.Devices) != 2 || len(stats.Installations) != 2 {
		t.Fatalf("device/installation summaries: %#v %#v", stats.Devices, stats.Installations)
	}
	found := false
	for _, row := range stats.Installations {
		if row.ID == installation[:8]+"…"+installation[len(installation)-4:] {
			found = row.Providers == "codex" && row.Device == "MacBook Air" && row.Commands == 3
		}
	}
	if !found {
		t.Fatalf("latest installation snapshot: %#v", stats.Installations)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("private@example.com")) || bytes.Contains(data, []byte("user-hostname")) {
		t.Fatal("stored rejected identifier")
	}
}

func TestRateLimitIsGlobal(t *testing.T) {
	s, _ := newService(t)
	when := time.Now().UTC().Format(time.RFC3339)
	for n := range 121 {
		b := payload(t, sampleEvent("oct_command_started", fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n), when))
		w := post(t, s.IngestHandler(), b)
		if n == 120 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d status %d", n, w.Code)
		}
		if n < 120 && w.Code != http.StatusAccepted {
			t.Fatalf("request %d status %d", n, w.Code)
		}
	}
}
