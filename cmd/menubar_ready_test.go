package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMenubarReadyEnvironment_includesOctPathAndReadyFile(t *testing.T) {
	env := menubarLaunchEnvironment([]string{"PATH=/bin"}, "/tmp/oct", "/tmp/ready")

	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "OCT_MENUBAR_OCT_PATH=/tmp/oct") {
		t.Fatalf("launch env missing OCT_MENUBAR_OCT_PATH: %v", env)
	}
	if !strings.Contains(joined, "OCT_MENUBAR_READY_FILE=/tmp/ready") {
		t.Fatalf("launch env missing OCT_MENUBAR_READY_FILE: %v", env)
	}
}

func TestWaitForMenubarReady_returnsNilWhenReadyFileExists(t *testing.T) {
	readyFile := filepath.Join(t.TempDir(), "ready")
	if err := os.WriteFile(readyFile, []byte("ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := waitForMenubarReady(readyFile, nil, 50*time.Millisecond); err != nil {
		t.Fatalf("waitForMenubarReady returned error: %v", err)
	}
}

func TestWaitForMenubarReady_returnsProcessErrorBeforeTimeout(t *testing.T) {
	processDone := make(chan error, 1)
	processDone <- errors.New("helper exited")

	err := waitForMenubarReady(filepath.Join(t.TempDir(), "missing"), processDone, time.Second)
	if err == nil || !strings.Contains(err.Error(), "helper exited") {
		t.Fatalf("waitForMenubarReady error = %v, want helper exited", err)
	}
}

func TestWaitForMenubarReady_timesOutWithoutReadyFile(t *testing.T) {
	err := waitForMenubarReady(filepath.Join(t.TempDir(), "missing"), nil, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("waitForMenubarReady error = %v, want timeout", err)
	}
}

func TestMenubarCommandReturnsForegroundFailure(t *testing.T) {
	oldRunMenubar := runMenubarCommand
	oldDaemon := menubarDaemon
	t.Cleanup(func() {
		runMenubarCommand = oldRunMenubar
		menubarDaemon = oldDaemon
	})

	runMenubarCommand = func() error {
		return errors.New("ready timeout")
	}
	menubarDaemon = false

	err := menubarCmd.RunE(menubarCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "menubar failed: ready timeout") {
		t.Fatalf("menubar command error = %v, want foreground failure", err)
	}
}

func TestMenubarCommandWritesDaemonStartedOnSuccess(t *testing.T) {
	oldStart := startMenubarDetachedCommand
	oldDaemon := menubarDaemon
	oldOut := menubarCmd.OutOrStdout()
	t.Cleanup(func() {
		startMenubarDetachedCommand = oldStart
		menubarDaemon = oldDaemon
		menubarCmd.SetOut(oldOut)
	})

	startMenubarDetachedCommand = func() error {
		return nil
	}
	menubarDaemon = true
	var out bytes.Buffer
	menubarCmd.SetOut(&out)

	if err := menubarCmd.RunE(menubarCmd, nil); err != nil {
		t.Fatalf("menubar daemon command returned error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "menubar daemon started" {
		t.Fatalf("daemon output = %q, want menubar daemon started", got)
	}
}
