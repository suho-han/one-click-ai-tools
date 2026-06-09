package schedule

import (
	"fmt"
	"os/exec"
)

type Windows struct{}

func (w *Windows) Enable(task Task, interval string, hour int) error {
	cfg, err := taskDetails(task)
	if err != nil {
		return err
	}

	binPath := resolveBinaryPath()

	taskName := windowsTaskName(task)
	exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()

	scheduleType := windowsScheduleType(interval)
	startTime := fmt.Sprintf("%02d:00", hour)
	cmd := exec.Command("schtasks", "/Create", "/TN", taskName, "/TR", binPath+" "+cfg.Command, "/SC", scheduleType, "/ST", startTime, "/F")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks failed: %v, output: %s", err, string(output))
	}

	return nil
}

func (w *Windows) Disable(task Task) error {
	cmd := exec.Command("schtasks", "/Delete", "/TN", windowsTaskName(task), "/F")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks failed: %v, output: %s", err, string(output))
	}
	return nil
}

func (w *Windows) Status(task Task) (string, error) {
	cmd := exec.Command("schtasks", "/Query", "/TN", windowsTaskName(task))
	if err := cmd.Run(); err == nil {
		return "enabled", nil
	}
	return "disabled", nil
}

func windowsScheduleType(interval string) string {
	if interval == "weekly" {
		return "WEEKLY"
	}
	return "DAILY"
}

func windowsTaskName(task Task) string {
	if task == SessionRefreshTask {
		return "OneClickToolsSessionRefresh"
	}
	return "OneClickToolsUpdate"
}
