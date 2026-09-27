package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestConfirmMenubarRestartDefaultsToYes(t *testing.T) {
	cases := map[string]string{
		"empty line (Enter)":    "\n",
		"y":                     "y\n",
		"YES":                   "YES\n",
		"EOF (non-interactive)": "",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			out := &bytes.Buffer{}
			if !confirmMenubarRestart(out, strings.NewReader(input), []int{42}) {
				t.Fatalf("expected default yes for input %q", input)
			}
			if !strings.Contains(out.String(), "pid 42") {
				t.Fatalf("prompt should identify the running instance, got %q", out.String())
			}
		})
	}
}

func TestConfirmMenubarRestartDeclines(t *testing.T) {
	out := &bytes.Buffer{}
	if confirmMenubarRestart(out, strings.NewReader("n\n"), []int{42}) {
		t.Fatalf("expected decline for input \"n\"")
	}
}

func TestHandleExistingMenubarInstanceStopsAndProceeds(t *testing.T) {
	origFind, origStop, origWait := findMenubarInstancePIDsCmd, stopMenubarInstancesCmd, waitForMenubarExitCmd
	t.Cleanup(func() {
		findMenubarInstancePIDsCmd, stopMenubarInstancesCmd, waitForMenubarExitCmd = origFind, origStop, origWait
	})

	findMenubarInstancePIDsCmd = func() []int { return []int{123, 456} }
	stopCalled := false
	stopMenubarInstancesCmd = func() (menubarStopResult, error) {
		stopCalled = true
		return menubarStopResult{Stopped: 2, PIDs: []string{"123", "456"}}, nil
	}
	var waitedPIDs []string
	waitForMenubarExitCmd = func(pids []string, timeout time.Duration) {
		waitedPIDs = pids
		if timeout != 3*time.Second {
			t.Fatalf("expected 3s wait timeout, got %v", timeout)
		}
	}

	cmd := &cobra.Command{}
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("\n"))

	proceed, err := handleExistingMenubarInstance(cmd)
	if err != nil {
		t.Fatalf("handleExistingMenubarInstance returned error: %v", err)
	}
	if !proceed || !stopCalled {
		t.Fatalf("expected restart: proceed=%v stopCalled=%v", proceed, stopCalled)
	}
	if strings.Join(waitedPIDs, ",") != "123,456" {
		t.Fatalf("expected wait on stopped pids, got %v", waitedPIDs)
	}
}

func TestHandleExistingMenubarInstanceKeepsRunningWhenDeclined(t *testing.T) {
	origFind, origStop := findMenubarInstancePIDsCmd, stopMenubarInstancesCmd
	t.Cleanup(func() {
		findMenubarInstancePIDsCmd, stopMenubarInstancesCmd = origFind, origStop
	})

	findMenubarInstancePIDsCmd = func() []int { return []int{123} }
	stopMenubarInstancesCmd = func() (menubarStopResult, error) {
		t.Fatalf("must not stop the instance when the user declines")
		return menubarStopResult{}, nil
	}

	cmd := &cobra.Command{}
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetIn(strings.NewReader("n\n"))

	proceed, err := handleExistingMenubarInstance(cmd)
	if err != nil {
		t.Fatalf("handleExistingMenubarInstance returned error: %v", err)
	}
	if proceed {
		t.Fatalf("expected proceed=false after decline")
	}
}

func TestHandleExistingMenubarInstanceSkipsChild(t *testing.T) {
	t.Setenv("OCT_MENUBAR_CHILD", "1")
	origFind := findMenubarInstancePIDsCmd
	t.Cleanup(func() { findMenubarInstancePIDsCmd = origFind })
	findMenubarInstancePIDsCmd = func() []int {
		t.Errorf("detached child must skip the instance check")
		return nil
	}

	cmd := &cobra.Command{}
	proceed, err := handleExistingMenubarInstance(cmd)
	if err != nil || !proceed {
		t.Fatalf("expected immediate proceed for detached child, got proceed=%v err=%v", proceed, err)
	}
}
