// Package analyticsserver stores non-identifying command events and explicitly
// submitted feedback text, never HTTP metadata or client addresses.
package analyticsserver

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const maxEventBytes = 2048

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var commandPath = regexp.MustCompile(`^oct(?: [a-z0-9]+(?:-[a-z0-9]+)*)+$`)
var token = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
var allowedProviders = map[string]bool{
	"agy": true, "claude": true, "commandcode": true, "cursor-agent": true,
	"copilot": true, "opencode": true, "codex": true, "kimi": true,
	"zai": true, "qwen": true, "deepseek": true, "openrouter": true,
	"minimax": true, "grok": true,
}

var allowedDeviceFamilies = map[string]bool{
	"Mac mini": true, "MacBook Air": true, "MacBook Pro": true,
	"Mac Studio": true, "iMac": true, "Mac Pro": true, "Mac": true,
	"Windows PC": true, "Linux PC": true, "Other": true,
}

type event struct {
	Event      string `json:"event"`
	DistinctID string `json:"distinct_id"`
	Timestamp  string `json:"timestamp"`
	Properties struct {
		SchemaVersion int      `json:"schema_version"`
		Command       string   `json:"command"`
		Source        string   `json:"source"`
		Version       string   `json:"version"`
		OS            string   `json:"os"`
		Arch          string   `json:"arch"`
		InvocationID  string   `json:"invocation_id"`
		Success       *bool    `json:"success,omitempty"`
		DurationMS    *int64   `json:"duration_ms,omitempty"`
		Providers     []string `json:"providers"`
		DeviceFamily  string   `json:"device_family"`
	} `json:"properties"`
}

// Service has separate public ingestion and private dashboard handlers. Close it
// after stopping both listeners.
type Service struct {
	db           *sql.DB
	rate         rateLimiter
	feedbackRate rateLimiter
	clearCSRF    string
}

// Open creates a durable SQLite database.
func Open(path string) (*Service, error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return nil, errors.New("database must be a private regular file (0600)")
		}
	} else if !os.IsNotExist(err) {
		return nil, errors.New("cannot inspect database")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, errors.New("cannot open database file")
	}
	if err = f.Close(); err != nil {
		return nil, errors.New("cannot close database file")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errors.New("cannot initialize database")
	}
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = DELETE",
		"PRAGMA secure_delete = ON",
		`CREATE TABLE IF NOT EXISTS events (
		  event TEXT NOT NULL CHECK(event IN ('oct_command_started','oct_command_finished')),
		  distinct_id TEXT NOT NULL,
		  timestamp TEXT NOT NULL,
		  schema_version INTEGER NOT NULL,
		  command TEXT NOT NULL,
		  source TEXT NOT NULL,
		  version TEXT NOT NULL,
		  os TEXT NOT NULL,
		  arch TEXT NOT NULL,
		  invocation_id TEXT NOT NULL,
		  success INTEGER,
		  duration_ms INTEGER,
		  UNIQUE(invocation_id, event)
		)`,
		"CREATE INDEX IF NOT EXISTS events_by_event_time ON events(event, timestamp)",
		"CREATE INDEX IF NOT EXISTS events_by_command ON events(command, source)",
		`CREATE TABLE IF NOT EXISTS installations (
		  distinct_id TEXT PRIMARY KEY,
		  last_seen TEXT NOT NULL,
		  providers TEXT NOT NULL,
		  device_family TEXT NOT NULL,
		  os TEXT NOT NULL,
		  arch TEXT NOT NULL
		)`,
		"CREATE INDEX IF NOT EXISTS events_by_installation ON events(distinct_id, event)",
		`CREATE TABLE IF NOT EXISTS feedback (
		  id INTEGER PRIMARY KEY,
		  received_at TEXT NOT NULL,
		  message TEXT NOT NULL
		)`,
		"CREATE INDEX IF NOT EXISTS feedback_by_received_at ON feedback(received_at)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, errors.New("cannot initialize analytics tables")
		}
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		db.Close()
		return nil, errors.New("cannot initialize dashboard action")
	}
	s := &Service{db: db, clearCSRF: hex.EncodeToString(secret[:])}
	if err := s.Prune(); err != nil {
		db.Close()
		return nil, errors.New("cannot prune expired analytics events")
	}
	return s, nil
}

func (s *Service) Close() error { return s.db.Close() }

// Prune removes events, installation observations and feedback older than 90 days.
// The executable runs this on startup and each day.
func (s *Service) Prune() error {
	cutoff := time.Now().UTC().Add(-90 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := s.db.Exec("DELETE FROM events WHERE timestamp < ?", cutoff); err != nil {
		return err
	}
	if _, err := s.db.Exec("DELETE FROM installations WHERE last_seen < ?", cutoff); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM feedback WHERE received_at < ?", cutoff)
	return err
}

// clearRecords removes usage event history and latest installation observations
// atomically. Feedback and client-side installation IDs remain untouched.
func (s *Service) clearRecords() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM events"); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM installations"); err != nil {
		return err
	}
	return tx.Commit()
}

// IngestHandler accepts command events and explicitly submitted feedback.
// Limit and rate checks precede parsing; errors never reflect submitted text.
func (s *Service) IngestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/v1/feedback" {
			s.ingestFeedback(w, r)
			return
		}
		if r.URL.Path != "/v1/events" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !s.rate.allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		if r.ContentLength > maxEventBytes || r.ContentLength == 0 || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxEventBytes+1))
		if err != nil || len(body) > maxEventBytes {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		if err := checkJSONKeys(body); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		var e event
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if dec.Decode(&e) != nil || dec.Decode(&struct{}{}) != io.EOF || !validEvent(e) {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}
		var success, duration any
		if e.Properties.Success != nil {
			success = *e.Properties.Success
		}
		if e.Properties.DurationMS != nil {
			duration = *e.Properties.DurationMS
		}
		tx, err := s.db.Begin()
		if err != nil {
			http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`INSERT INTO events(event, distinct_id, timestamp, schema_version, command, source, version, os, arch, invocation_id, success, duration_ms)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(invocation_id,event) DO NOTHING`, e.Event, e.DistinctID, e.Timestamp,
			e.Properties.SchemaVersion, e.Properties.Command, e.Properties.Source, e.Properties.Version,
			e.Properties.OS, e.Properties.Arch, e.Properties.InvocationID, success, duration)
		if err != nil {
			http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
			return
		}
		n, err := res.RowsAffected()
		if err != nil {
			http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
			return
		}
		if n != 0 && e.Event == "oct_command_started" && e.Properties.SchemaVersion == 2 {
			selected, _ := json.Marshal(e.Properties.Providers)
			_, err = tx.Exec(`INSERT INTO installations(distinct_id,last_seen,providers,device_family,os,arch)
			 VALUES(?,?,?,?,?,?) ON CONFLICT(distinct_id) DO UPDATE SET
			 last_seen=excluded.last_seen, providers=excluded.providers, device_family=excluded.device_family,
			 os=excluded.os, arch=excluded.arch WHERE excluded.last_seen >= installations.last_seen`,
				e.DistinctID, e.Timestamp, string(selected), e.Properties.DeviceFamily, e.Properties.OS, e.Properties.Arch)
			if err != nil {
				http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
			return
		}
		if n == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
}

func validEvent(e event) bool {
	p := e.Properties
	if e.Event != "oct_command_started" && e.Event != "oct_command_finished" ||
		!uuidV4.MatchString(e.DistinctID) || !uuidV4.MatchString(p.InvocationID) ||
		!commandPath.MatchString(p.Command) || len(p.Command) > 120 ||
		(p.Source != "cli" && p.Source != "noninteractive" && p.Source != "menubar") ||
		(p.SchemaVersion != 1 && p.SchemaVersion != 2) || !validLabel(p.Version, 64) || !validLabel(p.OS, 32) || !validLabel(p.Arch, 32) {
		return false
	}
	if p.SchemaVersion == 1 {
		if p.Providers != nil || p.DeviceFamily != "" {
			return false
		}
	} else {
		if p.Providers == nil || !allowedDeviceFamilies[p.DeviceFamily] || len(p.Providers) > len(allowedProviders) {
			return false
		}
		seen := make(map[string]bool, len(p.Providers))
		for _, provider := range p.Providers {
			if !allowedProviders[provider] || seen[provider] {
				return false
			}
			seen[provider] = true
		}
	}
	t, err := time.Parse(time.RFC3339, e.Timestamp)
	now := time.Now().UTC()
	if err != nil || len(e.Timestamp) != len("2006-01-02T15:04:05Z") || !strings.HasSuffix(e.Timestamp, "Z") || t.Format(time.RFC3339) != e.Timestamp || t.Before(now.Add(-90*24*time.Hour)) || t.After(now.Add(5*time.Minute)) {
		return false
	}
	if e.Event == "oct_command_started" {
		return p.Success == nil && p.DurationMS == nil
	}
	return p.Success != nil && p.DurationMS != nil && *p.DurationMS >= 0
}

func validLabel(v string, max int) bool { return len(v) > 0 && len(v) <= max && token.MatchString(v) }

// Recursively detect duplicate JSON keys. A typed decode alone would silently
// accept last-key-wins payloads, including overwritten privacy fields.
func checkJSONKeys(body []byte) error {
	d := json.NewDecoder(bytes.NewReader(body))
	if err := jsonValue(d); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(body, &top) != nil || len(top) != 4 {
		return errors.New("invalid event fields")
	}
	for _, name := range []string{"event", "distinct_id", "timestamp", "properties"} {
		if _, ok := top[name]; !ok {
			return errors.New("invalid event fields")
		}
	}
	var kind string
	var props map[string]json.RawMessage
	if json.Unmarshal(top["event"], &kind) != nil || json.Unmarshal(top["properties"], &props) != nil {
		return errors.New("invalid event fields")
	}
	var schema int
	if json.Unmarshal(props["schema_version"], &schema) != nil {
		return errors.New("invalid schema version")
	}
	required := []string{"schema_version", "command", "source", "version", "os", "arch", "invocation_id"}
	if kind == "oct_command_finished" {
		required = append(required, "success", "duration_ms")
	}
	if schema == 2 {
		required = append(required, "providers", "device_family")
	}
	if len(props) != len(required) {
		return errors.New("invalid property fields")
	}
	for _, name := range required {
		if _, ok := props[name]; !ok {
			return errors.New("invalid property fields")
		}
	}
	return nil
}

func jsonValue(d *json.Decoder) error {
	v, err := d.Token()
	if err != nil {
		return err
	}
	mark, ok := v.(json.Delim)
	if !ok {
		return nil
	}
	switch mark {
	case '{':
		seen := make(map[string]bool)
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate or invalid JSON key")
			}
			seen[name] = true
			if err := jsonValue(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := jsonValue(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	_, err = d.Token()
	return err
}

// One process-wide fixed-window bound for all clients, independent of IP.
type rateLimiter struct {
	mu    sync.Mutex
	until time.Time
	count int
}

func (l *rateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if !now.Before(l.until) {
		l.until = now.Add(time.Minute)
		l.count = 0
	}
	if l.count >= 120 {
		return false
	}
	l.count++
	return true
}
