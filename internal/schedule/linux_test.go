package schedule

import (
	"errors"
	"strings"
	"testing"
)

func TestLinuxLogPathUsesForwardSlashes(t *testing.T) {
	got := linuxLogPath("/tmp/test-home", "agent-update.log")
	want := "/tmp/test-home/.oct/logs/agent-update.log"
	if got != want {
		t.Fatalf("linuxLogPath() = %q, want %q", got, want)
	}
}

func TestLinuxEnableWritesTaskSpecificCronEntryWithoutDuplicates(t *testing.T) {
	origExec := executablePath
	origHome := homeDirPath
	origList := linuxCrontabList
	origWrite := linuxCrontabWrite
	origRemove := linuxCrontabRemove
	t.Cleanup(func() {
		executablePath = origExec
		homeDirPath = origHome
		linuxCrontabList = origList
		linuxCrontabWrite = origWrite
		linuxCrontabRemove = origRemove
	})

	executablePath = func() (string, error) { return "/tmp/oct-under-test", nil }
	homeDirPath = func() (string, error) { return "/tmp/test-home", nil }
	linuxCrontabList = func() ([]byte, error) {
		return []byte("0 1 * * * /usr/bin/old oct agent-update >> /tmp/old.log 2>&1  # oct-managed:agent-update\n"), nil
	}
	var written string
	linuxCrontabWrite = func(content string) error {
		written = content
		return nil
	}
	linuxCrontabRemove = func() error { return nil }

	l := &Linux{}
	if err := l.Enable(AgentUpdateTask, "daily", 3); err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}

	if strings.Count(written, cronMarker(AgentUpdateTask)) != 1 {
		t.Fatalf("expected exactly one managed entry, got %q", written)
	}
	if !strings.Contains(written, "/tmp/oct-under-test agent-update >> /tmp/test-home/.oct/logs/agent-update.log") {
		t.Fatalf("expected current binary + task log path, got %q", written)
	}
	if !strings.Contains(written, "0 3 * * *") {
		t.Fatalf("expected daily 3AM cron expression, got %q", written)
	}
}

func TestLinuxDisableRemovesOnlySelectedTask(t *testing.T) {
	origList := linuxCrontabList
	origWrite := linuxCrontabWrite
	origRemove := linuxCrontabRemove
	t.Cleanup(func() {
		linuxCrontabList = origList
		linuxCrontabWrite = origWrite
		linuxCrontabRemove = origRemove
	})

	linuxCrontabList = func() ([]byte, error) {
		return []byte(strings.Join([]string{
			"0 3 * * * /tmp/oct agent-update >> /tmp/a.log 2>&1  # oct-managed:agent-update",
			"0 9 * * * /tmp/oct session-refresh >> /tmp/s.log 2>&1  # oct-managed:session-refresh",
			"",
		}, "\n")), nil
	}
	var written string
	linuxCrontabWrite = func(content string) error {
		written = content
		return nil
	}
	linuxCrontabRemove = func() error {
		t.Fatalf("did not expect full crontab removal")
		return nil
	}

	l := &Linux{}
	if err := l.Disable(SessionRefreshTask); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}

	if strings.Contains(written, cronMarker(SessionRefreshTask)) {
		t.Fatalf("session-refresh entry should be removed, got %q", written)
	}
	if !strings.Contains(written, cronMarker(AgentUpdateTask)) {
		t.Fatalf("agent-update entry should remain, got %q", written)
	}
}

func TestLinuxDisableRemovesCrontabWhenLastManagedTaskDeleted(t *testing.T) {
	origList := linuxCrontabList
	origWrite := linuxCrontabWrite
	origRemove := linuxCrontabRemove
	t.Cleanup(func() {
		linuxCrontabList = origList
		linuxCrontabWrite = origWrite
		linuxCrontabRemove = origRemove
	})

	linuxCrontabList = func() ([]byte, error) {
		return []byte("0 9 * * * /tmp/oct session-refresh >> /tmp/s.log 2>&1  # oct-managed:session-refresh\n"), nil
	}
	linuxCrontabWrite = func(content string) error {
		t.Fatalf("did not expect rewrite when removing last entry")
		return nil
	}
	removed := false
	linuxCrontabRemove = func() error {
		removed = true
		return nil
	}

	l := &Linux{}
	if err := l.Disable(SessionRefreshTask); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if !removed {
		t.Fatalf("expected crontab remove to be called")
	}
}

func TestLinuxStatusUsesManagedMarker(t *testing.T) {
	origList := linuxCrontabList
	t.Cleanup(func() {
		linuxCrontabList = origList
	})

	linuxCrontabList = func() ([]byte, error) {
		return []byte("0 9 * * * /tmp/oct session-refresh >> /tmp/s.log 2>&1  # oct-managed:session-refresh\n"), nil
	}

	l := &Linux{}
	got, err := l.Status(SessionRefreshTask)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if got != "enabled" {
		t.Fatalf("expected enabled, got %q", got)
	}
}

func TestLinuxStatusTreatsMissingCrontabAsDisabled(t *testing.T) {
	origList := linuxCrontabList
	t.Cleanup(func() {
		linuxCrontabList = origList
	})

	linuxCrontabList = func() ([]byte, error) {
		return nil, errors.New("no crontab")
	}

	l := &Linux{}
	got, err := l.Status(SessionRefreshTask)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if got != "disabled" {
		t.Fatalf("expected disabled, got %q", got)
	}
}
