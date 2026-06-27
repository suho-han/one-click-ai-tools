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
)

func stopMenubarInstances() (menubarStopResult, error) {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		return menubarStopResult{}, err
	}

	currentPID := os.Getpid()
	result := menubarStopResult{}
	var killErrs []error
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		pid, command, ok := parsePSLine(string(line))
		if !ok || !isMenubarStopTarget(pid, currentPID, command) {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			if err == syscall.ESRCH {
				continue
			}
			killErrs = append(killErrs, fmt.Errorf("pid %d: %w", pid, err))
			continue
		}
		result.Stopped++
		result.PIDs = append(result.PIDs, strconv.Itoa(pid))
	}
	if len(killErrs) > 0 {
		return result, fmt.Errorf("failed to stop some menubar instances: %v", killErrs)
	}
	return result, nil
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
