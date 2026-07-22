package schedule

import (
	"bytes"
	"strings"
	"testing"
)

type testScheduleError string

func (e testScheduleError) Error() string { return string(e) }

func TestResolveBinaryPathPrefersExecutable(t *testing.T) {
	origExec := executablePath
	origLookPath := lookPath
	t.Cleanup(func() {
		executablePath = origExec
		lookPath = origLookPath
	})

	executablePath = func() (string, error) { return "/tmp/current-oct", nil }
	lookPath = func(string) (string, error) { return "/tmp/path-oct", nil }

	if got := resolveBinaryPath(); got != "/tmp/current-oct" {
		t.Fatalf("resolveBinaryPath() = %q, want executable path", got)
	}
}

func TestResolveBinaryPathFallsBackToLookPath(t *testing.T) {
	origExec := executablePath
	origLookPath := lookPath
	t.Cleanup(func() {
		executablePath = origExec
		lookPath = origLookPath
	})

	executablePath = func() (string, error) { return "", testScheduleError("boom") }
	lookPath = func(string) (string, error) { return "/tmp/path-oct", nil }

	if got := resolveBinaryPath(); got != "/tmp/path-oct" {
		t.Fatalf("resolveBinaryPath() = %q, want lookPath fallback", got)
	}
}

func TestCronExpression(t *testing.T) {
	if got := cronExpression("daily", 3); got != "0 3 * * *" {
		t.Fatalf("daily cron expression mismatch: %q", got)
	}
	if got := cronExpression("weekly", 9); got != "0 9 * * 1" {
		t.Fatalf("weekly cron expression mismatch: %q", got)
	}
	if got := cronExpression(TwelveHourInterval, 9); got != "0 */12 * * *" {
		t.Fatalf("12h cron expression mismatch: %q", got)
	}
	if got := cronExpression(SixHourInterval, 9); got != "0 */6 * * *" {
		t.Fatalf("6h cron expression mismatch: %q", got)
	}
	if got := cronExpression(OneHourInterval, 9); got != "0 * * * *" {
		t.Fatalf("1h cron expression mismatch: %q", got)
	}
}

func TestWindowsScheduleType(t *testing.T) {
	if got := windowsScheduleType("daily"); got != "DAILY" {
		t.Fatalf("daily schedule type mismatch: %q", got)
	}
	if got := windowsScheduleType("weekly"); got != "WEEKLY" {
		t.Fatalf("weekly schedule type mismatch: %q", got)
	}
	if got := windowsScheduleType(SixHourInterval); got != "HOURLY" {
		t.Fatalf("6h schedule type mismatch: %q", got)
	}
	if got := windowsScheduleModifier(SixHourInterval); got != "6" {
		t.Fatalf("6h schedule modifier mismatch: %q", got)
	}
	if got := windowsStartTime(SixHourInterval, 9); got != "00:00" {
		t.Fatalf("6h start time mismatch: %q", got)
	}
}

func TestParseTask(t *testing.T) {
	if got, err := ParseTask(" AGENT-UPDATE "); err != nil || got != AgentUpdateTask {
		t.Fatalf("ParseTask(agent-update) = %q, %v", got, err)
	}
	if got, err := ParseTask("session-refresh"); err != nil || got != SessionRefreshTask {
		t.Fatalf("ParseTask(session-refresh) = %q, %v", got, err)
	}
	if _, err := ParseTask("unknown"); err == nil {
		t.Fatalf("expected ParseTask to reject unknown task")
	}
}

func TestTaskHelpers(t *testing.T) {
	if got := cronMarker(SessionRefreshTask); got != "# oct-managed:session-refresh" {
		t.Fatalf("unexpected cron marker: %q", got)
	}
	if got := windowsTaskName(SessionRefreshTask); got != "OneClickToolsSessionRefresh" {
		t.Fatalf("unexpected windows task name: %q", got)
	}
	if got := launchAgentLabel("com.oct", SessionRefreshTask); got != "com.oct.session-refresh" {
		t.Fatalf("unexpected launch agent label: %q", got)
	}
}

func TestParseInterval(t *testing.T) {
	tests := map[string]string{
		"daily":  DailyInterval,
		"1d":     DailyInterval,
		"weekly": WeeklyInterval,
		"1w":     WeeklyInterval,
		"12h":    TwelveHourInterval,
		"6h":     SixHourInterval,
		"1h":     OneHourInterval,
		"hourly": OneHourInterval,
	}

	for raw, want := range tests {
		if got, err := ParseInterval(raw); err != nil || got != want {
			t.Fatalf("ParseInterval(%s) = %q, %v, want %q", raw, got, err, want)
		}
	}

	if _, err := ParseInterval("monthly"); err == nil {
		t.Fatal("expected ParseInterval to reject unsupported interval")
	}
}

func TestParseHour(t *testing.T) {
	if got, err := ParseHour("9"); err != nil || got != 9 {
		t.Fatalf("ParseHour(9) = %d, %v", got, err)
	}
	for _, raw := range []string{"-1", "24", "abc"} {
		if _, err := ParseHour(raw); err == nil {
			t.Fatalf("expected ParseHour to reject %q", raw)
		}
	}
}

func TestValidateScheduleTimingNormalizesAndRejectsInvalidHour(t *testing.T) {
	interval, err := validateScheduleTiming(" WEEKLY ", 23)
	if err != nil {
		t.Fatalf("validateScheduleTiming returned error: %v", err)
	}
	if interval != "weekly" {
		t.Fatalf("validateScheduleTiming interval = %q, want weekly", interval)
	}
	if _, err := validateScheduleTiming("daily", 24); err == nil {
		t.Fatal("expected invalid hour to be rejected")
	}
}

func TestWindowsTaskCommandQuotesExecutablePath(t *testing.T) {
	got := windowsTaskCommand(`C:\Program Files\oct\oct.exe`, "session-refresh")
	want := `"C:\Program Files\oct\oct.exe" session-refresh`
	if got != want {
		t.Fatalf("windowsTaskCommand() = %q, want %q", got, want)
	}
}

func TestFormatScheduleOmitsHourForSubdailyIntervals(t *testing.T) {
	if got := FormatSchedule(DailyInterval, 9); got != "daily, 09:00" {
		t.Fatalf("daily format mismatch: %q", got)
	}
	if got := FormatSchedule(SixHourInterval, 9); got != "6h" {
		t.Fatalf("6h format mismatch: %q", got)
	}
}

func TestRenderLaunchAgentPlistEscapesXMLValues(t *testing.T) {
	var buf bytes.Buffer
	err := renderLaunchAgentPlist(&buf, launchAgentTemplateData{
		Label:      "com.oct.agent-update",
		BinaryPath: "/tmp/oct & tools/oct",
		Command:    string(AgentUpdateTask),
		Interval:   "daily",
		Hour:       9,
		LogPath:    "/tmp/oct <logs>/agent-update.log",
	})
	if err != nil {
		t.Fatalf("renderLaunchAgentPlist returned error: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "/tmp/oct & tools/oct") || !strings.Contains(got, "/tmp/oct &amp; tools/oct") {
		t.Fatalf("expected escaped binary path, got %s", got)
	}
	if strings.Contains(got, "/tmp/oct <logs>/agent-update.log") || !strings.Contains(got, "/tmp/oct &lt;logs&gt;/agent-update.log") {
		t.Fatalf("expected escaped log path, got %s", got)
	}
}

func TestRenderLaunchAgentPlistUsesStartIntervalForSubdailySchedules(t *testing.T) {
	var buf bytes.Buffer
	err := renderLaunchAgentPlist(&buf, launchAgentTemplateData{
		Label:                "com.oct.session-refresh",
		BinaryPath:           "/tmp/oct",
		Command:              string(SessionRefreshTask),
		Interval:             SixHourInterval,
		Hour:                 9,
		StartIntervalSeconds: startIntervalSeconds(SixHourInterval),
		LogPath:              "/tmp/session-refresh.log",
	})
	if err != nil {
		t.Fatalf("renderLaunchAgentPlist returned error: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "<key>StartInterval</key>") || !strings.Contains(got, "<integer>21600</integer>") {
		t.Fatalf("expected StartInterval 21600, got %s", got)
	}
	if strings.Contains(got, "StartCalendarInterval") {
		t.Fatalf("subdaily schedule should not render StartCalendarInterval: %s", got)
	}
}
