package schedule

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderTestPlist(t *testing.T, interval string, hour int) []byte {
	t.Helper()
	var buf strings.Builder
	err := renderLaunchAgentPlist(&buf, launchAgentTemplateData{
		Label:                "com.oct.agent-update",
		BinaryPath:           "/usr/local/bin/oct",
		Command:              "agent-update",
		Interval:             interval,
		Hour:                 hour,
		StartIntervalSeconds: startIntervalSeconds(interval),
		LogPath:              "/tmp/oct.log",
	})
	if err != nil {
		t.Fatalf("renderLaunchAgentPlist: %v", err)
	}
	return []byte(buf.String())
}

// TestParseLaunchdScheduleRoundTrip pins the Describe contract: every interval
// Enable can write into the plist template parses back to the same pair.
func TestParseLaunchdScheduleRoundTrip(t *testing.T) {
	cases := []struct {
		interval     string
		hour         int
		wantInterval string
		wantHour     int
	}{
		{OneHourInterval, 9, OneHourInterval, 0},
		{SixHourInterval, 9, SixHourInterval, 0},
		{TwelveHourInterval, 9, TwelveHourInterval, 0},
		{DailyInterval, 9, DailyInterval, 9},
		{WeeklyInterval, 8, WeeklyInterval, 8},
	}
	for _, tc := range cases {
		gotInterval, gotHour, ok := parseLaunchdSchedule(renderTestPlist(t, tc.interval, tc.hour))
		if !ok {
			t.Fatalf("parseLaunchdSchedule(%s) not recognized", tc.interval)
		}
		if gotInterval != tc.wantInterval || gotHour != tc.wantHour {
			t.Fatalf("parseLaunchdSchedule(%s, %d) = (%s, %d), want (%s, %d)",
				tc.interval, tc.hour, gotInterval, gotHour, tc.wantInterval, tc.wantHour)
		}
	}
}

func TestParseLaunchdSchedule_Unrecognized(t *testing.T) {
	// A hand-edited StartInterval oct never writes must stay "enabled with
	// unknown interval", not a disabled task.
	if _, _, ok := parseLaunchdSchedule(renderTestPlist(t, DailyInterval, 9)[:0]); ok {
		t.Fatal("empty plist should not parse")
	}

	handEdited := strings.Replace(string(renderTestPlist(t, OneHourInterval, 0)),
		"<integer>3600</integer>", "<integer>7200</integer>", 1)
	if _, _, ok := parseLaunchdSchedule([]byte(handEdited)); ok {
		t.Fatal("unrecognized StartInterval should not parse to an interval")
	}
}

func TestParseCronSchedule_RoundTripAndRejects(t *testing.T) {
	cases := []struct {
		entry        string
		wantInterval string
		wantHour     int
		wantOK       bool
	}{
		{"0 9 * * * '/oct' agent-update # oct-managed:agent-update", DailyInterval, 9, true},
		{"0 8 * * 1 '/oct' agent-update # oct-managed:agent-update", WeeklyInterval, 8, true},
		{"0 */12 * * * '/oct' agent-update # oct-managed:agent-update", TwelveHourInterval, 0, true},
		{"0 */6 * * * '/oct' agent-update # oct-managed:agent-update", SixHourInterval, 0, true},
		{"0 * * * * '/oct' agent-update # oct-managed:agent-update", OneHourInterval, 0, true},
		// Hand-edited or unexpected shapes stay enabled-but-unknown.
		{"*/15 * * * * '/oct' agent-update # oct-managed:agent-update", "", 0, false},
		{"0 9 1 * * '/oct' agent-update # oct-managed:agent-update", "", 0, false},
		{"not a crontab line", "", 0, false},
	}
	for _, tc := range cases {
		interval, hour, ok := parseCronSchedule(tc.entry)
		if ok != tc.wantOK || interval != tc.wantInterval || hour != tc.wantHour {
			t.Fatalf("parseCronSchedule(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.entry, interval, hour, ok, tc.wantInterval, tc.wantHour, tc.wantOK)
		}
	}
}

func TestMacOSDescribe_ReadsPlistFromHome(t *testing.T) {
	origHome := homeDirPath
	t.Cleanup(func() { homeDirPath = origHome })
	home := t.TempDir()
	homeDirPath = func() (string, error) { return home, nil }

	m := &MacOS{LabelPrefix: "com.oct"}

	// No plist -> disabled.
	state, err := m.Describe(AgentUpdateTask)
	if err != nil {
		t.Fatalf("Describe without plist: %v", err)
	}
	if state.Enabled {
		t.Fatal("expected disabled without a plist")
	}

	// Enabled daily@8: write the plist exactly like Enable does.
	plistPath := launchAgentPath(home, "com.oct", AgentUpdateTask)
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		t.Fatalf("mkdir LaunchAgents: %v", err)
	}
	if err := writeFileAtomic(plistPath, renderTestPlist(t, DailyInterval, 8), 0o644); err != nil {
		t.Fatalf("write plist: %v", err)
	}
	state, err = m.Describe(AgentUpdateTask)
	if err != nil {
		t.Fatalf("Describe with plist: %v", err)
	}
	if !state.Enabled || state.Interval != DailyInterval || state.Hour != 8 {
		t.Fatalf("Describe = %+v, want enabled daily@8", state)
	}
}

func TestLinuxDescribe_ReadsCrontab(t *testing.T) {
	origList := linuxCrontabList
	t.Cleanup(func() { linuxCrontabList = origList })

	l := &Linux{}

	linuxCrontabList = func() ([]byte, error) {
		return []byte("0 9 * * * '/oct' agent-update >> /tmp/log 2>&1 # oct-managed:agent-update\n"), nil
	}
	state, err := l.Describe(AgentUpdateTask)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if !state.Enabled || state.Interval != DailyInterval || state.Hour != 9 {
		t.Fatalf("Describe = %+v, want enabled daily@9", state)
	}

	linuxCrontabList = func() ([]byte, error) { return nil, errors.New("no crontab for user") }
	state, err = l.Describe(AgentUpdateTask)
	if err != nil {
		t.Fatalf("Describe without crontab: %v", err)
	}
	if state.Enabled {
		t.Fatal("expected disabled without a crontab entry")
	}

	linuxCrontabList = func() ([]byte, error) {
		return []byte("*/15 * * * * '/oct' agent-update # oct-managed:agent-update\n"), nil
	}
	state, err = l.Describe(AgentUpdateTask)
	if err != nil {
		t.Fatalf("Describe with hand-edited entry: %v", err)
	}
	if !state.Enabled || state.Interval != "" {
		t.Fatalf("Describe = %+v, want enabled with unknown interval", state)
	}
}
