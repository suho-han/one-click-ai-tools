package schedule

import "testing"

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
