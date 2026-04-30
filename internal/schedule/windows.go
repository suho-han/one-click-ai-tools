package schedule

import (
	"fmt"
	"os"
	"os/exec"
)

type Windows struct{}

func (w *Windows) Enable(interval string, hour int) error {
	binPath, err := exec.LookPath("oct")
	if err != nil {
		binPath, _ = os.Executable()
	}

	taskName := "OneClickToolsUpdate"
	
	// Delete existing task if any
	exec.Command("schtasks", "/Delete", "/TN", taskName, "/F").Run()

	var scheduleType string
	if interval == "weekly" {
		scheduleType = "WEEKLY"
	} else {
		scheduleType = "DAILY"
	}

	startTime := fmt.Sprintf("%02d:00", hour)
	
	cmd := exec.Command("schtasks", "/Create", "/TN", taskName, "/TR", binPath+" agent-update", "/SC", scheduleType, "/ST", startTime, "/F")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks failed: %v, output: %s", err, string(output))
	}

	return nil
}

func (w *Windows) Disable() error {
	taskName := "OneClickToolsUpdate"
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks failed: %v, output: %s", err, string(output))
	}
	return nil
}

func (w *Windows) Status() (string, error) {
	taskName := "OneClickToolsUpdate"
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName)
	if err := cmd.Run(); err == nil {
		return "enabled", nil
	}
	return "disabled", nil
}
