package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/schedule"
)

// fakeConfigScheduler records scheduler calls so the config API's
// agent-update schedule surface can be tested without touching launchd.
type fakeConfigScheduler struct {
	current     schedule.CurrentSchedule
	describeErr error
	enabled     []schedule.CurrentSchedule
	disabled    int
}

func (f *fakeConfigScheduler) Enable(_ schedule.Task, interval string, hour int) error {
	f.enabled = append(f.enabled, schedule.CurrentSchedule{Enabled: true, Interval: interval, Hour: hour})
	f.current = schedule.CurrentSchedule{Enabled: true, Interval: interval, Hour: hour}
	return nil
}

func (f *fakeConfigScheduler) Disable(_ schedule.Task) error {
	f.disabled++
	f.current = schedule.CurrentSchedule{}
	return nil
}

func (f *fakeConfigScheduler) Status(schedule.Task) (string, error) {
	if f.current.Enabled {
		return "enabled", nil
	}
	return "disabled", nil
}

func (f *fakeConfigScheduler) Describe(schedule.Task) (schedule.CurrentSchedule, error) {
	return f.current, f.describeErr
}

func withFakeConfigScheduler(t *testing.T, fake *fakeConfigScheduler) {
	t.Helper()
	orig := configScheduleScheduler
	configScheduleScheduler = func() (schedule.Scheduler, error) { return fake, nil }
	t.Cleanup(func() { configScheduleScheduler = orig })
}

// boolPtr/intPtr come from config_api_test.go.

func TestApplyAgentUpdateScheduleUpdate(t *testing.T) {
	tests := []struct {
		name        string
		current     schedule.CurrentSchedule
		describeErr error
		payload     *configScheduleUpdatePayload
		wantEnable  *schedule.CurrentSchedule
		wantDisable int
		wantErr     bool
	}{
		{
			name:    "nil payload never touches the scheduler",
			current: schedule.CurrentSchedule{Enabled: true, Interval: "daily", Hour: 8},
			payload: nil,
		},
		{
			name:    "matching enabled schedule is skipped",
			current: schedule.CurrentSchedule{Enabled: true, Interval: "daily", Hour: 8},
			payload: &configScheduleUpdatePayload{Enabled: boolPtr(true), Interval: "daily", Hour: intPtr(8)},
		},
		{
			name:    "matching hourly schedule ignores the hour",
			current: schedule.CurrentSchedule{Enabled: true, Interval: "1h", Hour: 0},
			payload: &configScheduleUpdatePayload{Enabled: boolPtr(true), Interval: "1h", Hour: intPtr(5)},
		},
		{
			name:       "changed interval enables with the new value",
			current:    schedule.CurrentSchedule{Enabled: true, Interval: "daily", Hour: 8},
			payload:    &configScheduleUpdatePayload{Enabled: boolPtr(true), Interval: "weekly", Hour: intPtr(8)},
			wantEnable: &schedule.CurrentSchedule{Enabled: true, Interval: "weekly", Hour: 8},
		},
		{
			name:       "off to daily enables with the payload hour",
			current:    schedule.CurrentSchedule{},
			payload:    &configScheduleUpdatePayload{Enabled: boolPtr(true), Interval: "daily", Hour: intPtr(7)},
			wantEnable: &schedule.CurrentSchedule{Enabled: true, Interval: "daily", Hour: 7},
		},
		{
			name:        "disable runs only when currently enabled",
			current:     schedule.CurrentSchedule{Enabled: true, Interval: "daily", Hour: 8},
			payload:     &configScheduleUpdatePayload{Enabled: boolPtr(false)},
			wantDisable: 1,
		},
		{
			name:    "disable on an already disabled schedule is skipped",
			current: schedule.CurrentSchedule{},
			payload: &configScheduleUpdatePayload{Enabled: boolPtr(false)},
		},
		{
			name:    "enabled with unknown interval round-trips when unchanged",
			current: schedule.CurrentSchedule{Enabled: true},
			payload: &configScheduleUpdatePayload{Enabled: boolPtr(true), Interval: "", Hour: intPtr(9)},
		},
		{
			name:    "enabled with unknown interval from a disabled state is rejected",
			current: schedule.CurrentSchedule{},
			payload: &configScheduleUpdatePayload{Enabled: boolPtr(true), Interval: "", Hour: intPtr(9)},
			wantErr: true,
		},
		{
			name:    "hour outside 0-23 is rejected",
			current: schedule.CurrentSchedule{},
			payload: &configScheduleUpdatePayload{Enabled: boolPtr(true), Interval: "daily", Hour: intPtr(24)},
			wantErr: true,
		},
		{
			name:        "describe failure surfaces instead of rewriting",
			current:     schedule.CurrentSchedule{},
			describeErr: errors.New("launchd unreachable"),
			payload:     &configScheduleUpdatePayload{Enabled: boolPtr(false)},
			wantErr:     true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeConfigScheduler{current: tc.current, describeErr: tc.describeErr}
			withFakeConfigScheduler(t, fake)

			err := applyAgentUpdateScheduleUpdate(tc.payload)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("applyAgentUpdateScheduleUpdate: %v", err)
			}
			if got := len(fake.enabled); (tc.wantEnable != nil) != (got == 1) {
				t.Fatalf("enable calls = %d, want 1=%v", got, tc.wantEnable != nil)
			}
			if tc.wantEnable != nil && fake.enabled[0] != *tc.wantEnable {
				t.Fatalf("enabled with %+v, want %+v", fake.enabled[0], *tc.wantEnable)
			}
			if fake.disabled != tc.wantDisable {
				t.Fatalf("disable calls = %d, want %d", fake.disabled, tc.wantDisable)
			}
		})
	}
}

// TestConfigListJSONIncludesScheduleAndVersionShape pins the snapshot keys the
// menubar settings decode: agent_update_schedule always present, versions
// absent without --probe-versions.
func TestConfigListJSONIncludesScheduleAndVersionShape(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.SetConfigFile("/tmp/does-not-exist.yaml")

	snapshot := buildConfigSnapshot("/tmp/does-not-exist.yaml")
	// Build twice through the JSON encoding to guarantee stable field names.
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if !strings.Contains(string(encoded), `"agent_update_schedule"`) {
		t.Fatalf("snapshot missing agent_update_schedule: %s", encoded)
	}
	if strings.Contains(string(encoded), `"version"`) {
		t.Fatalf("snapshot without probing must not carry versions: %s", encoded)
	}

	var decoded struct {
		AgentUpdateSchedule configScheduleSnapshot `json:"agent_update_schedule"`
		Tools               []configToolStatus     `json:"tools"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
}

func TestDisplayVersion(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"2.1.288 (Claude Code)", "2.1.288"},
		{"0.160.0", "0.160.0"},
		{"opencode v2.0.20", "2.0.20"},
		{"GitHub Copilot CLI 1.0.86.", "1.0.86"},
		{"2026.09.18-9a7762b", "2026.09.18-9a7762b"},
		{"1.2.16", "1.2.16"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := displayVersion(tc.raw); got != tc.want {
			t.Fatalf("displayVersion(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
