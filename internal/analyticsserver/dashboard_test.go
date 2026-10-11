package analyticsserver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDashboardMetricsAndRetentionWithoutLogin(t *testing.T) {
	s, _ := newService(t)
	monday := time.Now().UTC().Truncate(24 * time.Hour)
	monday = monday.AddDate(0, 0, -(int(monday.Weekday())+6)%7)
	prior := monday.AddDate(0, 0, -7)
	otherInstallation := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	start := func(invocation string, when time.Time, id, source string) {
		t.Helper()
		e := sampleEvent("oct_command_started", invocation, when.Format(time.RFC3339))
		e["distinct_id"] = id
		e["properties"].(map[string]any)["source"] = source
		if w := post(t, s.IngestHandler(), payload(t, e)); w.Code != http.StatusAccepted {
			t.Fatalf("start: %d", w.Code)
		}
	}
	finish := func(invocation string, when time.Time, id string, ms int, success bool) {
		t.Helper()
		e := sampleEvent("oct_command_finished", invocation, when.Format(time.RFC3339))
		e["distinct_id"] = id
		e["properties"].(map[string]any)["duration_ms"] = ms
		e["properties"].(map[string]any)["success"] = success
		if w := post(t, s.IngestHandler(), payload(t, e)); w.Code != http.StatusAccepted {
			t.Fatalf("finish: %d", w.Code)
		}
	}
	start("11111111-0000-4000-8000-000000000001", prior, installation, "cli")
	start("11111111-0000-4000-8000-000000000002", monday, installation, "menubar")
	start("11111111-0000-4000-8000-000000000003", monday, otherInstallation, "cli")
	start("11111111-0000-4000-8000-000000000004", monday, "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", "noninteractive")
	finish("11111111-0000-4000-8000-000000000001", prior, installation, 10, true)
	finish("11111111-0000-4000-8000-000000000002", monday, installation, 42, false)
	finish("11111111-0000-4000-8000-000000000003", monday, otherInstallation, 100, true)
	data, err := s.statistics()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Commands) != 3 || len(data.Outcomes) != 1 || data.Outcomes[0].Success != 2 || data.Outcomes[0].Failure != 1 {
		t.Fatalf("counts: %#v %#v", data.Commands, data.Outcomes)
	}
	if len(data.Durations) != 1 || data.Durations[0].P50 != 42 || data.Durations[0].P95 != 100 {
		t.Fatalf("percentiles: %#v", data.Durations)
	}
	if len(data.Daily) != 2 || data.Daily[0].Count != 2 || len(data.Weekly) != 2 || data.Weekly[0].Count != 2 {
		t.Fatalf("active: %#v %#v", data.Daily, data.Weekly)
	}
	if len(data.Retention) != 2 || data.Retention[0].Active != 2 || data.Retention[0].Returning != 1 || data.Retention[1].Returning != 0 {
		t.Fatalf("returning: %#v", data.Retention)
	}

	handler := s.DashboardHandler()
	request := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		for _, header := range []string{"Cache-Control", "Referrer-Policy", "Content-Security-Policy"} {
			if w.Header().Get(header) == "" {
				t.Errorf("missing %s", header)
			}
		}
		return w
	}
	if w := request("/"); w.Code != http.StatusNotFound {
		t.Fatalf("wrong route: %d", w.Code)
	}
	public := httptest.NewRecorder()
	s.IngestHandler().ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/oct-analytics", nil))
	if public.Code != http.StatusNotFound {
		t.Fatalf("dashboard exposed on ingestion handler: %d", public.Code)
	}
	w := request("/oct-analytics")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Observed weekly returning installations") ||
		!strings.Contains(w.Body.String(), "90 days") || !strings.Contains(w.Body.String(), "oct alert config show") ||
		w.Header().Get("WWW-Authenticate") != "" {
		t.Fatalf("dashboard: %d %q", w.Code, w.Body.String())
	}
}

func TestPruneOldRows(t *testing.T) {
	s, path := newService(t)
	old := time.Now().Add(-91 * 24 * time.Hour).UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO events(event,distinct_id,timestamp,schema_version,command,source,version,os,arch,invocation_id)
	VALUES('oct_command_started',?,?,?,?,?,?,?,?,?)`, installation, old, 1, "oct usage", "cli", "1.0.0", "linux", "amd64", "cccccccc-cccc-4ccc-8ccc-cccccccccccc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO installations(distinct_id,last_seen,providers,device_family,os,arch)
		VALUES(?,?,?,?,?,?)`, installation, old, `["codex"]`, "Mac mini", "darwin", "arm64"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var count int
	if err := reopened.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil || count != 0 {
		t.Fatalf("startup prune failed: %d %v", count, err)
	}
	if err := reopened.db.QueryRow("SELECT COUNT(*) FROM installations").Scan(&count); err != nil || count != 0 {
		t.Fatalf("startup installation prune failed: %d %v", count, err)
	}
	if _, err := reopened.db.Exec(`INSERT INTO events(event,distinct_id,timestamp,schema_version,command,source,version,os,arch,invocation_id)
VALUES('oct_command_started',?,?,?,?,?,?,?,?,?)`, installation, old, 1, "oct usage", "cli", "1.0.0", "linux", "amd64", "dddddddd-dddd-4ddd-8ddd-dddddddddddd"); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.db.Exec(`INSERT INTO installations(distinct_id,last_seen,providers,device_family,os,arch)
		VALUES(?,?,?,?,?,?)`, installation, old, `["codex"]`, "Mac mini", "darwin", "arm64"); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Prune(); err != nil {
		t.Fatal(err)
	}
	if err := reopened.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil || count != 0 {
		t.Fatalf("daily prune failed: %d %v", count, err)
	}
	if err := reopened.db.QueryRow("SELECT COUNT(*) FROM installations").Scan(&count); err != nil || count != 0 {
		t.Fatalf("daily installation prune failed: %d %v", count, err)
	}
}

func TestClearUsageRecordsRequiresConfirmationAndPreservesFeedback(t *testing.T) {
	s, _ := newService(t)
	now := time.Now().UTC().Format(time.RFC3339)
	invocation := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	for _, kind := range []string{"oct_command_started", "oct_command_finished"} {
		e := sampleEvent(kind, invocation, now)
		props := e["properties"].(map[string]any)
		props["schema_version"] = 2
		props["providers"] = []string{"codex"}
		props["device_family"] = "Mac mini"
		if w := post(t, s.IngestHandler(), payload(t, e)); w.Code != http.StatusAccepted {
			t.Fatalf("seed %s: %d", kind, w.Code)
		}
	}
	if _, err := s.db.Exec("INSERT INTO feedback(received_at,message) VALUES(?,?)", now, "independent feedback"); err != nil {
		t.Fatal(err)
	}
	handler := s.DashboardHandler()
	request := func(method, path string, form url.Values) *httptest.ResponseRecorder {
		t.Helper()
		body := ""
		if form != nil {
			body = form.Encode()
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if method == http.MethodPost {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := request(http.MethodGet, "/oct-analytics?confirm=clear", nil); w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), s.clearCSRF) || !strings.Contains(w.Body.String(), "does not remove submitted feedback") {
		t.Fatalf("confirmation unavailable: %d", w.Code)
	}
	for _, form := range []url.Values{
		{"confirm": {"DELETE ALL"}},
		{"csrf": {"wrong"}, "confirm": {"DELETE ALL"}},
		{"csrf": {s.clearCSRF}, "confirm": {"not confirmed"}},
		{"csrf": {s.clearCSRF}, "confirm": {"DELETE ALL"}, "unexpected": {"x"}},
	} {
		if w := request(http.MethodPost, "/oct-analytics", form); w.Code != http.StatusForbidden {
			t.Fatalf("unconfirmed deletion accepted: %d", w.Code)
		}
	}
	var events, installs int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&events); err != nil || events != 2 {
		t.Fatalf("records changed before confirmation: %d %v", events, err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM installations").Scan(&installs); err != nil || installs != 1 {
		t.Fatalf("installation changed before confirmation: %d %v", installs, err)
	}
	confirmed := url.Values{"csrf": {s.clearCSRF}, "confirm": {"DELETE ALL"}}
	if w := request(http.MethodPost, "/oct-analytics", confirmed); w.Code != http.StatusSeeOther ||
		w.Header().Get("Location") != "/oct-analytics" {
		t.Fatalf("confirmed deletion: %d %q", w.Code, w.Header().Get("Location"))
	}
	for _, table := range []string{"events", "installations"} {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s not cleared: %d %v", table, count, err)
		}
	}
	var feedback int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM feedback").Scan(&feedback); err != nil || feedback != 1 {
		t.Fatalf("unrelated feedback removed: %d %v", feedback, err)
	}
	e := sampleEvent("oct_command_started", "ffffffff-ffff-4fff-8fff-ffffffffffff", now)
	props := e["properties"].(map[string]any)
	props["schema_version"] = 2
	props["providers"] = []string{"codex"}
	props["device_family"] = "Mac mini"
	if w := post(t, s.IngestHandler(), payload(t, e)); w.Code != http.StatusAccepted {
		t.Fatalf("future events did not resume: %d", w.Code)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM installations").Scan(&installs); err != nil || installs != 1 {
		t.Fatalf("installation did not reappear: %d %v", installs, err)
	}
}
