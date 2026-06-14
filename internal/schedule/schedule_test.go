package schedule

import "testing"

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
}

func TestWindowsScheduleType(t *testing.T) {
	if got := windowsScheduleType("daily"); got != "DAILY" {
		t.Fatalf("daily schedule type mismatch: %q", got)
	}
	if got := windowsScheduleType("weekly"); got != "WEEKLY" {
		t.Fatalf("weekly schedule type mismatch: %q", got)
	}
}

func TestParseTask(t *testing.T) {
	if got, err := ParseTask("agent-update"); err != nil || got != AgentUpdateTask {
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
	if got, err := ParseInterval("daily"); err != nil || got != "daily" {
		t.Fatalf("ParseInterval(daily) = %q, %v", got, err)
	}
	if got, err := ParseInterval("weekly"); err != nil || got != "weekly" {
		t.Fatalf("ParseInterval(weekly) = %q, %v", got, err)
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

func TestWindowsTaskCommandQuotesExecutablePath(t *testing.T) {
	got := windowsTaskCommand(`C:\Program Files\oct\oct.exe`, "session-refresh")
	want := `"C:\Program Files\oct\oct.exe" session-refresh`
	if got != want {
		t.Fatalf("windowsTaskCommand() = %q, want %q", got, want)
	}
}
