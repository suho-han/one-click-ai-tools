package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/suho-han/one-click-ai-tools/internal/telemetry"
)

func TestCLIAnalyticsCommandOutcomeAndOptOut(t *testing.T) {
	viperResetForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CI", "false")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("OCT_TELEMETRY_ENABLED", "")
	cfg := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(cfg, []byte("enabled_tools: [codex]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var events []struct {
		Event      string `json:"event"`
		DistinctID string `json:"distinct_id"`
		Properties struct {
			Command      string   `json:"command"`
			Source       string   `json:"source"`
			InvocationID string   `json:"invocation_id"`
			Providers    []string `json:"providers"`
			DeviceFamily string   `json:"device_family"`
			Success      *bool    `json:"success"`
		} `json:"properties"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var event struct {
			Event      string `json:"event"`
			DistinctID string `json:"distinct_id"`
			Properties struct {
				Command      string   `json:"command"`
				Source       string   `json:"source"`
				InvocationID string   `json:"invocation_id"`
				Providers    []string `json:"providers"`
				DeviceFamily string   `json:"device_family"`
				Success      *bool    `json:"success"`
			} `json:"properties"`
		}
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("decode event: %v", err)
		}
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	previousEndpoint := telemetry.Endpoint
	telemetry.Endpoint = server.URL + "/v1/events"
	t.Cleanup(func() { telemetry.Endpoint = previousEndpoint })
	run := func(args ...string) (int, string, string) {
		var out, errout bytes.Buffer
		code := runCLI(append([]string{"--config", cfg}, args...), &out, &errout)
		return code, out.String(), errout.String()
	}
	if code, _, stderr := run("alert", "config", "show"); code != 0 || stderr != "" {
		t.Fatalf("success: %d %q", code, stderr)
	}
	if code, _, stderr := run("alert", "config", "set", "threshold_percent", "abc"); code != 1 || stderr == "" {
		t.Fatalf("failure: %d %q", code, stderr)
	}
	mu.Lock()
	if len(events) != 4 {
		t.Fatalf("events=%d, want 4", len(events))
	}
	id := events[0].DistinctID
	pairs := map[string][]string{}
	for _, e := range events {
		if e.DistinctID != id || e.Properties.Command != "oct alert config show" && e.Properties.Command != "oct alert config set" || e.Properties.Source != "noninteractive" {
			t.Fatalf("unexpected event: %+v", e)
		}
		if !reflect.DeepEqual(e.Properties.Providers, []string{"codex"}) || e.Properties.DeviceFamily == "" {
			t.Fatalf("missing selected provider or device family: %+v", e.Properties)
		}
		pairs[e.Properties.InvocationID] = append(pairs[e.Properties.InvocationID], e.Event)
		if e.Event == "oct_command_finished" && e.Properties.Command == "oct alert config set" && (e.Properties.Success == nil || *e.Properties.Success) {
			t.Fatalf("failure recorded as success: %+v", e)
		}
	}
	mu.Unlock()
	if len(pairs) != 2 {
		t.Fatalf("invocations=%d, want 2", len(pairs))
	}
	for _, pair := range pairs {
		if len(pair) != 2 {
			t.Fatalf("incomplete pair: %v", pair)
		}
	}
	if code, _, stderr := run("config", "set", "telemetry-enabled", "false"); code != 0 {
		t.Fatalf("disable: %d %q", code, stderr)
	}
	if code, _, stderr := run("alert", "config", "show"); code != 0 {
		t.Fatalf("disabled command: %d %q", code, stderr)
	}
	mu.Lock()
	count := len(events)
	mu.Unlock()
	if count != 4 {
		t.Fatalf("opt-out sent %d events", count-4)
	}
}

func TestCLIAnalyticsDisabledHasNoInstallationState(t *testing.T) {
	viperResetForTest(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CI", "false")
	t.Setenv("DO_NOT_TRACK", "1")
	previous := telemetry.Endpoint
	telemetry.Endpoint = "https://analytics.example.com/v1/events"
	t.Cleanup(func() { telemetry.Endpoint = previous })
	cfg := writeTempConfig(t)
	var out, errout bytes.Buffer
	if code := runCLI([]string{"--config", cfg, "alert", "config", "show"}, &out, &errout); code != 0 {
		t.Fatalf("disabled run: %d %q", code, errout.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".oct", "telemetry")); !os.IsNotExist(err) {
		t.Fatalf("disabled run made telemetry state: %v", err)
	}
}

func TestTelemetryEnvironmentOptOutPrecedence(t *testing.T) {
	previous := telemetry.Endpoint
	telemetry.Endpoint = "https://analytics.example.com/v1/events"
	t.Cleanup(func() { telemetry.Endpoint = previous })
	for _, tc := range []struct {
		name, ci, dnt, setting string
		want                   bool
	}{
		{"normal", "", "", "", true},
		{"explicit enabled", "", "", "true", true},
		{"DNT wins", "", "1", "true", false},
		{"CI wins", "yes", "", "true", false},
		{"CI false", "FALSE", "", "", true},
		{"CI zero", "0", "", "", true},
		{"malformed setting", "", "", "perhaps", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CI", tc.ci)
			t.Setenv("DO_NOT_TRACK", tc.dnt)
			t.Setenv("OCT_TELEMETRY_ENABLED", tc.setting)
			if got := telemetryAllowedSetting(true); got != tc.want {
				t.Fatalf("allowed = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestBackgroundVersionCheckIsExcludedButUpdateRemainsObservable(t *testing.T) {
	flag := agentUpdateCmd.Flags().Lookup("check")
	previous, changed := flag.Value.String(), flag.Changed
	t.Cleanup(func() {
		_ = flag.Value.Set(previous)
		flag.Changed = changed
	})
	if err := flag.Value.Set("true"); err != nil {
		t.Fatal(err)
	}
	if commandEligible(agentUpdateCmd) {
		t.Fatal("automatic version check would create an analytics event")
	}
	if err := flag.Value.Set("false"); err != nil {
		t.Fatal(err)
	}
	if !commandEligible(agentUpdateCmd) {
		t.Fatal("explicit agent update should remain observable")
	}
}

func TestSelectedTelemetryProvidersOnlyIncludesExplicitRegistryEntries(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configured []string
		want       []string
	}{
		{"unconfigured", nil, []string{}},
		{"empty list does not imply all tools", []string{}, []string{}},
		{"canonical aliases and labels", []string{"claude-code", "codex:work-email", "codex:private", "gemini", "zhipu", "unknown", "  "}, []string{"agy", "claude", "codex", "zai"}},
		{"comma separated environment", []string{"deepseek,openrouter,cursor,nonexistent"}, []string{"cursor-agent", "deepseek", "openrouter"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectedTelemetryProviders(tc.configured); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("selected providers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCLIAnalyticsProvidersExcludeImplicitDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		want         []string
	}{
		{"unset despite agent order", "agent_order: [codex]\n", []string{}},
		{"empty means no explicit selection", "enabled_tools: []\n", []string{}},
		{"selected aliases and unknown entries", "enabled_tools: [gemini, codex:private-label, zhipu, nonexistent, codex]\n", []string{"agy", "codex", "zai"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viperResetForTest(t)
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("CI", "false")
			t.Setenv("DO_NOT_TRACK", "")
			t.Setenv("OCT_TELEMETRY_ENABLED", "")
			cfg := filepath.Join(home, "config.yaml")
			if err := os.WriteFile(cfg, []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var providers [][]string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()
				var event struct {
					Properties struct {
						Providers    []string `json:"providers"`
						DeviceFamily string   `json:"device_family"`
					} `json:"properties"`
				}
				if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
					t.Errorf("decode: %v", err)
				}
				if event.Properties.DeviceFamily == "" {
					t.Error("device family missing")
				}
				mu.Lock()
				providers = append(providers, event.Properties.Providers)
				mu.Unlock()
			}))
			defer server.Close()
			previous := telemetry.Endpoint
			telemetry.Endpoint = server.URL
			t.Cleanup(func() { telemetry.Endpoint = previous })
			var out, errout bytes.Buffer
			if code := runCLI([]string{"--config", cfg, "alert", "config", "show"}, &out, &errout); code != 0 {
				t.Fatalf("CLI exited %d: %s", code, errout.String())
			}
			mu.Lock()
			defer mu.Unlock()
			if len(providers) != 2 {
				t.Fatalf("event count = %d", len(providers))
			}
			for _, got := range providers {
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("selected providers = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
