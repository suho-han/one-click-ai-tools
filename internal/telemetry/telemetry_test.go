package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testOptions(home, endpoint string) Options {
	return Options{Endpoint: endpoint, HomeDir: home, Version: "1.2.3"}
}

func testCommand() Command {
	return Command{Path: "oct alert config show", Source: "noninteractive"}
}

func keys(m map[string]json.RawMessage) []string {
	result := make([]string, 0, len(m))
	for key := range m {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func TestCapturePairAllowlistAndInstallationRetention(t *testing.T) {
	var mu sync.Mutex
	var received [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/events" || r.Header.Get("User-Agent") != "oct-telemetry" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s %v", r.Method, r.URL.Path, r.Header)
		}
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode capture: %v", err)
		}
		mu.Lock()
		received = append(received, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	home := t.TempDir()
	for _, success := range []bool{true, false} {
		opts := testOptions(home, server.URL+"/v1/events")
		opts.Providers = []string{"codex", "zai"}
		s := Start(context.Background(), opts, testCommand())
		if s == nil {
			t.Fatal("expected enabled session")
		}
		time.Sleep(time.Millisecond)
		s.Finish(success, true)
		s.Finish(!success, true) // cannot produce a second finished event
	}
	mu.Lock()
	bodies := append([][]byte(nil), received...)
	mu.Unlock()
	if len(bodies) != 4 {
		t.Fatalf("capture count = %d, want 4", len(bodies))
	}
	var installation string
	invocations := map[string]bool{}
	pairs := map[string]map[string]map[string]json.RawMessage{}
	for _, body := range bodies {
		if strings.Contains(string(body), "secret-password") || strings.Contains(string(body), "/private/path") || strings.Contains(string(body), "error-sentinel") {
			t.Errorf("sensitive input leaked: %s", body)
		}
		var event map[string]json.RawMessage
		if err := json.Unmarshal(body, &event); err != nil {
			t.Fatal(err)
		}
		if got, want := keys(event), []string{"distinct_id", "event", "properties", "timestamp"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("event fields = %v, want %v", got, want)
		}
		var name, id, stamp string
		json.Unmarshal(event["event"], &name)
		json.Unmarshal(event["distinct_id"], &id)
		json.Unmarshal(event["timestamp"], &stamp)
		if _, err := time.Parse(time.RFC3339, stamp); err != nil || !strings.HasSuffix(stamp, "Z") {
			t.Errorf("invalid UTC timestamp %q: %v", stamp, err)
		}
		if !validUUID(id) {
			t.Errorf("invalid installation UUID %q", id)
		}
		if installation != "" && installation != id {
			t.Errorf("installation changed: %q vs %q", installation, id)
		}
		installation = id
		var props map[string]json.RawMessage
		json.Unmarshal(event["properties"], &props)
		var invocation string
		json.Unmarshal(props["invocation_id"], &invocation)
		if !validUUID(invocation) {
			t.Errorf("invalid invocation UUID %q", invocation)
		}
		if pairs[invocation] == nil {
			pairs[invocation] = map[string]map[string]json.RawMessage{}
		}
		pairs[invocation][name] = props
		invocations[invocation] = true
		base := map[string]any{"schema_version": float64(2), "command": "oct alert config show", "source": "noninteractive", "version": "1.2.3", "os": runtime.GOOS, "arch": runtime.GOARCH, "invocation_id": invocation}
		for key, want := range base {
			var got any
			json.Unmarshal(props[key], &got)
			if got != want {
				t.Errorf("%s = %v, want %v", key, got, want)
			}
		}
		var providers []string
		if err := json.Unmarshal(props["providers"], &providers); err != nil || !reflect.DeepEqual(providers, []string{"codex", "zai"}) {
			t.Errorf("providers = %v (%v)", providers, err)
		}
		var family string
		if err := json.Unmarshal(props["device_family"], &family); err != nil {
			t.Errorf("device_family = %s (%v)", props["device_family"], err)
		}
		wantFamily := map[string]string{"darwin": "Mac", "windows": "Windows PC", "linux": "Linux PC"}[runtime.GOOS]
		if wantFamily == "" {
			wantFamily = "Other"
		}
		if runtime.GOOS != "darwin" && family != wantFamily {
			t.Errorf("device_family = %q, want %q", family, wantFamily)
		}
		if runtime.GOOS == "darwin" && !validMacFamily(family) {
			t.Errorf("invalid Mac family %q", family)
		}
		wantKeys := []string{"arch", "command", "device_family", "invocation_id", "os", "providers", "schema_version", "source", "version"}
		if name == "oct_command_finished" {
			wantKeys = append(wantKeys, "duration_ms", "success")
			sort.Strings(wantKeys)
			var duration int64
			if err := json.Unmarshal(props["duration_ms"], &duration); err != nil || duration < 0 {
				t.Errorf("invalid duration: %s (%v)", props["duration_ms"], err)
			}
		}
		if got := keys(props); !reflect.DeepEqual(got, wantKeys) {
			t.Errorf("%s fields = %v, want %v", name, got, wantKeys)
		}
	}
	if len(invocations) != 2 {
		t.Errorf("invocations = %d, want 2", len(invocations))
	}
	var outcomes []bool
	for _, pair := range pairs {
		if len(pair) != 2 || pair["oct_command_started"] == nil || pair["oct_command_finished"] == nil {
			t.Errorf("incomplete pair: %v", pair)
			continue
		}
		var outcome bool
		json.Unmarshal(pair["oct_command_finished"]["success"], &outcome)
		outcomes = append(outcomes, outcome)
	}
	sort.Slice(outcomes, func(i, j int) bool { return !outcomes[i] && outcomes[j] })
	if !reflect.DeepEqual(outcomes, []bool{false, true}) {
		t.Errorf("outcomes = %v", outcomes)
	}
	data, err := os.ReadFile(filepath.Join(home, ".oct", "telemetry", "installation-id"))
	if err != nil || strings.TrimSpace(string(data)) != installation {
		t.Errorf("persisted ID = %q, err %v; want %q", data, err, installation)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(filepath.Join(home, ".oct", "telemetry")); err != nil || info.Mode().Perm() != 0700 {
			t.Errorf("directory mode: %v %v", info, err)
		}
		if info, err := os.Stat(filepath.Join(home, ".oct", "telemetry", "installation-id")); err != nil || info.Mode().Perm() != 0600 {
			t.Errorf("file mode: %v %v", info, err)
		}
	}
}

func TestDisabledAndInvalidInputsHaveNoEffects(t *testing.T) {
	cases := []struct {
		name, endpoint string
		command        Command
	}{
		{"no endpoint", "", testCommand()},
		{"remote HTTP", "http://example.com/v1/events", testCommand()},
		{"localhost name", "http://localhost/v1/events", testCommand()},
		{"credentials in endpoint", "https://user@example.com/v1/events", testCommand()},
		{"invalid source", "https://analytics.example.com/v1/events", Command{Path: "oct usage", Source: "private-source"}},
		{"arguments in path", "https://analytics.example.com/v1/events", Command{Path: "oct usage --prompt=secret-password", Source: "cli"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := filepath.Join(t.TempDir(), "untouched")
			var nilSession *Session
			nilSession.Finish(true, true)
			s := Start(context.Background(), Options{Endpoint: tc.endpoint, HomeDir: home}, tc.command)
			if s != nil {
				t.Fatal("invalid options created a session")
			}
			if _, err := os.Stat(home); !os.IsNotExist(err) {
				t.Errorf("disabled session touched home: %v", err)
			}
		})
	}
}

func TestEndpointAllowsOnlyHTTPSOrNumericLoopback(t *testing.T) {
	for _, tc := range []struct {
		url  string
		want bool
	}{
		{"https://analytics.example.com/v1/events", true},
		{"http://127.0.0.1:8080/v1/events", true},
		{"http://[::1]:8080/v1/events", true},
		{"http://localhost:8080/v1/events", false},
		{"http://192.168.1.1/v1/events", false},
		{"https://user:secret@analytics.example.com/v1/events", false},
		{"https://analytics.example.com/v1/events#fragment", false},
	} {
		if got := validEndpoint(tc.url); got != tc.want {
			t.Errorf("validEndpoint(%q) = %t, want %t", tc.url, got, tc.want)
		}
	}
}

func TestInstallationIDRejectsCorruptionAndNonregularFiles(t *testing.T) {
	for _, contents := range []string{"", "not-a-uuid", "00000000-0000-4000-8000-000000000000extra", "00000000-0000-1000-8000-000000000000"} {
		t.Run(contents, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, ".oct", "telemetry", "installation-id")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			if s := Start(context.Background(), testOptions(home, "https://analytics.example.com/v1/events"), testCommand()); s != nil {
				t.Fatal("corrupt ID created a session")
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != contents {
				t.Errorf("existing damaged ID changed: %q %v", data, err)
			}
		})
	}
	for _, kind := range []string{"symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, ".oct", "telemetry", "installation-id")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			var err error
			if kind == "directory" {
				err = os.Mkdir(path, 0700)
			} else {
				target := filepath.Join(home, "target")
				if err = os.WriteFile(target, []byte("00000000-0000-4000-8000-000000000000"), 0600); err == nil {
					err = os.Symlink(target, path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if s := Start(context.Background(), testOptions(home, "https://analytics.example.com/v1/events"), testCommand()); s != nil {
				t.Fatal("nonregular ID created a session")
			}
		})
	}
}

func TestUnreadableInstallationIDIsNotReplaced(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".oct", "telemetry", "installation-id")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	id := "00000000-0000-4000-8000-000000000000"
	if err := os.WriteFile(path, []byte(id), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0600) })
	if file, err := os.Open(path); err == nil {
		file.Close()
		t.Skip("this user can read files despite mode 0000")
	}
	if s := Start(context.Background(), testOptions(home, "https://analytics.example.com/v1/events"), testCommand()); s != nil {
		t.Fatal("unreadable ID created a session")
	}
	if info, err := os.Lstat(path); err != nil || info.Mode().Perm() != 0000 {
		t.Errorf("unreadable ID was replaced: %v %v", info, err)
	}
}

func TestConcurrentFirstInstallationNeverIssuesDifferentIDs(t *testing.T) {
	home := t.TempDir()
	const workers = 32
	var wg sync.WaitGroup
	ids := make(chan string, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := installationID(home)
			if err == nil {
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)
	stored, err := readInstallationID(filepath.Join(home, ".oct", "telemetry", "installation-id"))
	if err != nil || !validUUID(stored) {
		t.Fatalf("invalid persisted UUID %q: %v", stored, err)
	}
	count := 0
	for id := range ids {
		count++
		if id != stored {
			t.Errorf("concurrent ID %q differs from persisted ID %q", id, stored)
		}
	}
	if count == 0 {
		t.Fatal("no initial writer succeeded")
	}
}

func TestFailuresAndRedirectsAreDroppedWithoutRetries(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { redirected.Add(1) }))
	defer target.Close()
	for _, status := range []int{http.StatusInternalServerError, http.StatusTooManyRequests, http.StatusTemporaryRedirect} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var received atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				if status == http.StatusTemporaryRedirect {
					w.Header().Set("Location", target.URL)
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			s := Start(context.Background(), testOptions(t.TempDir(), server.URL), testCommand())
			if s == nil {
				t.Fatal("expected session")
			}
			s.Finish(false, true)
			if got := received.Load(); got != 2 {
				t.Errorf("got %d requests, want exactly 2 (no retries)", got)
			}
		})
	}
	if got := redirected.Load(); got != 0 {
		t.Errorf("followed %d redirects", got)
	}
}

func TestStalledRequestsRespectCombinedFlushBound(t *testing.T) {
	var arrived atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if arrived.Add(1) == 1 {
			close(started)
		}
		<-release
	}))
	defer server.Close()
	defer close(release)
	s := Start(context.Background(), testOptions(t.TempDir(), server.URL), testCommand())
	if s == nil {
		t.Fatal("expected session")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("start request did not reach the server")
	}
	began := time.Now()
	s.Finish(true, true)
	if elapsed := time.Since(began); elapsed > 400*time.Millisecond {
		t.Errorf("finish blocked %v; expected roughly 200ms maximum", elapsed)
	}
	if arrived.Load() == 0 {
		t.Error("server did not receive either capture")
	}
}

func TestFinishDisabledDoesNotSendFinished(t *testing.T) {
	var mu sync.Mutex
	var names []string
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Event string `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
		}
		mu.Lock()
		names = append(names, body.Event)
		mu.Unlock()
		if body.Event == "oct_command_started" {
			close(started)
		}
	}))
	defer server.Close()
	s := Start(context.Background(), testOptions(t.TempDir(), server.URL), testCommand())
	if s == nil {
		t.Fatal("expected session")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("start capture not received")
	}
	s.Finish(true, false)
	s.Finish(true, true)
	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(names, []string{"oct_command_started"}) {
		t.Errorf("disabled finish sent %v", names)
	}
}
