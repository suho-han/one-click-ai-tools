//go:build darwin && cgo

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func stopMenubarInstances() (menubarStopResult, error) {
	targets, err := findMenubarStopTargets()
	if err != nil {
		return menubarStopResult{}, err
	}
	result := menubarStopResult{}
	var killErrs []error
	for _, target := range targets {
		if err := syscall.Kill(target.pid, syscall.SIGTERM); err != nil {
			if err == syscall.ESRCH {
				continue
			}
			killErrs = append(killErrs, fmt.Errorf("pid %d: %w", target.pid, err))
			continue
		}
		result.Stopped++
		result.PIDs = append(result.PIDs, strconv.Itoa(target.pid))
	}
	if len(killErrs) > 0 {
		return result, fmt.Errorf("failed to stop some menubar instances: %v", killErrs)
	}
	return result, nil
}

type menubarTargetProcess struct {
	pid     int
	command string
}

func findMenubarStopTargets() ([]menubarTargetProcess, error) {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return nil, err
	}
	currentPID := os.Getpid()
	var targets []menubarTargetProcess
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		pid, command, ok := parsePSLine(string(line))
		if !ok || !isMenubarStopTarget(pid, currentPID, command) {
			continue
		}
		targets = append(targets, menubarTargetProcess{pid: pid, command: command})
	}
	return targets, nil
}

// findMenubarInstancePIDs reports running menubar instances (the single-
// instance check for `oct menubar`). Empty when none are running or the
// process table cannot be read.
func findMenubarInstancePIDs() []int {
	targets, err := findMenubarStopTargets()
	if err != nil {
		return nil
	}
	pids := make([]int, 0, len(targets))
	for _, target := range targets {
		pids = append(pids, target.pid)
	}
	return pids
}

// waitForMenubarExit polls until the stopped instances are gone (SIGTERM is
// a request, not a guarantee) and SIGKILLs any straggler after the timeout.
func waitForMenubarExit(pids []string, timeout time.Duration) {
	pending := make([]int, 0, len(pids))
	for _, raw := range pids {
		if pid, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && pid > 0 {
			pending = append(pending, pid)
		}
	}
	deadline := time.Now().Add(timeout)
	for len(pending) > 0 && time.Now().Before(deadline) {
		var remaining []int
		for _, pid := range pending {
			if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
				continue
			}
			remaining = append(remaining, pid)
		}
		pending = remaining
		if len(pending) == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	for _, pid := range pending {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func parsePSLine(line string) (int, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, "", false
	}
	pidText, command, ok := strings.Cut(line, " ")
	if !ok {
		return 0, "", false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidText))
	if err != nil {
		return 0, "", false
	}
	return pid, strings.TrimSpace(command), true
}
