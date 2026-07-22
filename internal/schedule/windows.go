package schedule

import (
	"fmt"
	"os/exec"
)

type Windows struct{}

func (w *Windows) Enable(task Task, interval string, hour int) error {
	interval, err := validateScheduleTiming(interval, hour)
	if err != nil {
		return err
	}
	cfg, err := taskDetails(task)
	if err != nil {
		return err
	}

	binPath := resolveBinaryPath()

	taskName := windowsTaskName(task)
	exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()

	args := []string{"/Create", "/TN", taskName, "/TR", windowsTaskCommand(binPath, cfg.Command), "/SC", windowsScheduleType(interval)}
	if modifier := windowsScheduleModifier(interval); modifier != "" {
		args = append(args, "/MO", modifier)
	}
	args = append(args, "/ST", windowsStartTime(interval, hour), "/F")
	cmd := exec.Command("schtasks", args...)
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
	switch interval {
	case WeeklyInterval:
		return "WEEKLY"
	case TwelveHourInterval, SixHourInterval, OneHourInterval:
		return "HOURLY"
	default:
		return "DAILY"
	}
}

func windowsScheduleModifier(interval string) string {
	switch interval {
	case TwelveHourInterval:
		return "12"
	case SixHourInterval:
		return "6"
	case OneHourInterval:
		return "1"
	default:
		return ""
	}
}

func windowsStartTime(interval string, hour int) string {
	if !IntervalUsesHour(interval) {
		return "00:00"
	}
	return fmt.Sprintf("%02d:00", hour)
}

func windowsTaskName(task Task) string {
	if task == SessionRefreshTask {
		return "OneClickToolsSessionRefresh"
	}
	return "OneClickToolsUpdate"
}

func windowsTaskCommand(binPath, command string) string {
	return fmt.Sprintf("\"%s\" %s", binPath, command)
}
