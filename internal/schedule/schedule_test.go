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
