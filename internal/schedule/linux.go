package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Linux struct{}

func (l *Linux) Enable(interval string, hour int) error {
	binPath, err := exec.LookPath("oct")
	if err != nil {
		binPath, _ = os.Executable()
	}

	home, _ := os.UserHomeDir()
	logPath := filepathJoin(home, ".oct", "logs", "schedule.log")
	os.MkdirAll(filepathJoin(home, ".oct", "logs"), 0755)

	var cronExpr string
	if interval == "weekly" {
		cronExpr = fmt.Sprintf("0 %d * * 1", hour)
	} else {
		cronExpr = fmt.Sprintf("0 %d * * *", hour)
	}

	cronEntry := fmt.Sprintf("%s %s agent-update >> %s 2>&1", cronExpr, binPath, logPath)

	// Get current crontab
	out, _ := exec.Command("crontab", "-l").Output()
	lines := strings.Split(string(out), "\n")

	var newLines []string
	for _, line := range lines {
		if line == "" || strings.Contains(line, "oct agent-update") {
			continue
		}
		newLines = append(newLines, line)
	}
	newLines = append(newLines, cronEntry)

	newCrontab := strings.Join(newLines, "\n") + "\n"
	
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("crontab failed: %v, output: %s", err, string(output))
	}

	return nil
}

func (l *Linux) Disable() error {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		return nil // No crontab
	}

	lines := strings.Split(string(out), "\n")
	var newLines []string
	found := false
	for _, line := range lines {
		if strings.Contains(line, "oct agent-update") {
			found = true
			continue
		}
		if line != "" {
			newLines = append(newLines, line)
		}
	}

	if !found {
		return nil
	}

	newCrontab := strings.Join(newLines, "\n") + "\n"
	if len(newLines) == 0 {
		return exec.Command("crontab", "-r").Run()
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCrontab)
	return cmd.Run()
}

func (l *Linux) Status() (string, error) {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		return "disabled", nil
	}
	if strings.Contains(string(out), "oct agent-update") {
		return "enabled", nil
	}
	return "disabled", nil
}

func filepathJoin(elem ...string) string {
	return strings.Join(elem, string(os.PathSeparator))
}
