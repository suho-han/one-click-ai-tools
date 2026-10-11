package analyticsserver

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func feedbackPost(s *Service, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/v1/feedback", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "metadata-sentinel")
	r.RemoteAddr = "metadata-sentinel:1234"
	w := httptest.NewRecorder()
	s.IngestHandler().ServeHTTP(w, r)
	return w
}
func feedbackBody(message string) []byte {
	b, _ := json.Marshal(map[string]string{"message": message})
	return b
}

func TestFeedbackPersistenceAndPrivacy(t *testing.T) {
	s, path := newService(t)
	message := "  한글 피드백\nAzure 확인  "
	before := time.Now().UTC().Add(-time.Second)
	if w := feedbackPost(s, feedbackBody(message)); w.Code != 202 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var stored, stamp string
	if err := s.db.QueryRow("SELECT message,received_at FROM feedback").Scan(&stored, &stamp); err != nil {
		t.Fatal(err)
	}
	ts, err := time.Parse(time.RFC3339, stamp)
	if err != nil || !strings.HasSuffix(stamp, "Z") || ts.Before(before) || ts.After(time.Now()) || stored != message {
		t.Fatalf("stored %q at %q", stored, stamp)
	}
	var events int
	if err := s.db.QueryRow("SELECT count(*) FROM events").Scan(&events); err != nil || events != 0 {
		t.Fatalf("events %d: %v", events, err)
	}
	s.Close()
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.db.QueryRow("SELECT message FROM feedback").Scan(&stored); err != nil || stored != message {
		t.Fatalf("reopen %q: %v", stored, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("metadata-sentinel")) {
		t.Fatal("HTTP metadata persisted")
	}
}

func TestFeedbackValidation(t *testing.T) {
	s, _ := newService(t)
	for _, body := range [][]byte{feedbackBody(strings.Repeat("a", 4000)), []byte(`{"message":"` + strings.Repeat(`\u0061`, 4000) + `"}`)} {
		if w := feedbackPost(s, body); w.Code != 202 {
			t.Fatalf("boundary %d", w.Code)
		}
	}
	for name, body := range map[string][]byte{
		"oversize message": feedbackBody(strings.Repeat("a", 4001)), "blank": feedbackBody(" \n\t"), "missing": []byte(`{}`), "null": []byte(`{"message":null}`), "number": []byte(`{"message":1}`), "case": []byte(`{"Message":"sentinel"}`), "unknown": []byte(`{"message":"sentinel","ip":"sentinel"}`), "duplicate": []byte(`{"message":"a","message":"sentinel"}`), "trailing": []byte(`{"message":"sentinel"} {}`), "malformed": []byte(`{"message":`), "invalid utf8": append([]byte(`{"message":"`), 0xff), "oversize body": []byte(strings.Repeat(" ", 32769)), "empty": nil,
	} {
		t.Run(name, func(t *testing.T) {
			w := feedbackPost(s, body)
			if w.Code != 400 || w.Body.String() != "invalid feedback\n" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%d %q", w.Code, w.Body.String())
			}
		})
	}
	for _, method := range []string{"GET", "PUT"} {
		w := httptest.NewRecorder()
		s.IngestHandler().ServeHTTP(w, httptest.NewRequest(method, "/v1/feedback", nil))
		if w.Code != 405 || w.Header().Get("Allow") != "POST" {
			t.Fatal(w.Code)
		}
	}
	r := httptest.NewRequest("POST", "/v1/feedback", bytes.NewReader(feedbackBody("sentinel")))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()
	s.IngestHandler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	r = httptest.NewRequest("POST", "/v1/feedback", strings.NewReader(strings.Repeat(" ", 32769)))
	r.ContentLength = -1
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.IngestHandler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM feedback").Scan(&count); err != nil || count != 2 {
		t.Fatalf("%d %v", count, err)
	}
}

func TestFeedbackQuotaStorageAndRetention(t *testing.T) {
	s, _ := newService(t)
	for i := range 120 {
		if w := feedbackPost(s, feedbackBody("feedback")); w.Code != 202 {
			t.Fatalf("request %d: %d", i, w.Code)
		}
	}
	if w := feedbackPost(s, feedbackBody("feedback")); w.Code != 429 {
		t.Fatal(w.Code)
	}
	e := sampleEvent("oct_command_started", "22222222-2222-4222-8222-222222222222", time.Now().UTC().Format(time.RFC3339))
	for range 120 {
		w := post(t, s.IngestHandler(), payload(t, e))
		if w.Code != 202 && w.Code != 204 {
			t.Fatal(w.Code)
		}
	}
	if w := post(t, s.IngestHandler(), payload(t, e)); w.Code != 429 {
		t.Fatal(w.Code)
	}
	old := time.Now().UTC().Add(-91 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.db.Exec("INSERT INTO feedback(received_at,message) VALUES(?,?)", old, "expired"); err != nil {
		t.Fatal(err)
	}
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM feedback").Scan(&count); err != nil || count != 120 {
		t.Fatalf("%d %v", count, err)
	}
	s.feedbackRate = rateLimiter{}
	s.Close()
	if w := feedbackPost(s, feedbackBody("private-sentinel")); w.Code != 503 || strings.Contains(w.Body.String(), "sentinel") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if err := s.Prune(); err == nil {
		t.Fatal("closed storage prune succeeded")
	}
}
