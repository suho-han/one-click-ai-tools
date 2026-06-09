package schedule

import (
	"fmt"
	"os"
	"strings"
)

type Linux struct{}

func (l *Linux) Enable(task Task, interval string, hour int) error {
	cfg, err := taskDetails(task)
	if err != nil {
		return err
	}

	binPath := resolveBinaryPath()

	home, _ := homeDirPath()
	logPath := filepathJoin(home, ".oct", "logs", cfg.LogFile)
	os.MkdirAll(filepathJoin(home, ".oct", "logs"), 0o755)

	cronExpr := cronExpression(interval, hour)
	cronEntry := fmt.Sprintf("%s %s %s >> %s 2>&1  %s", cronExpr, binPath, cfg.Command, logPath, cronMarker(task))

	out, err := linuxCrontabList()
	if err != nil {
		out = []byte{}
	}
	lines := strings.Split(string(out), "\n")

	var newLines []string
	for _, line := range lines {
		if line == "" || strings.Contains(line, cronMarker(task)) {
			continue
		}
		newLines = append(newLines, line)
	}
	newLines = append(newLines, cronEntry)

	newCrontab := strings.Join(newLines, "\n") + "\n"
	if err := linuxCrontabWrite(newCrontab); err != nil {
		return err
	}

	return nil
}

func (l *Linux) Disable(task Task) error {
	out, err := linuxCrontabList()
	if err != nil {
		return nil
	}

	lines := strings.Split(string(out), "\n")
	var newLines []string
	found := false
	for _, line := range lines {
		if strings.Contains(line, cronMarker(task)) {
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
		return linuxCrontabRemove()
	}

	return linuxCrontabWrite(newCrontab)
}

func (l *Linux) Status(task Task) (string, error) {
	out, err := linuxCrontabList()
	if err != nil {
		return "disabled", nil
	}
	if strings.Contains(string(out), cronMarker(task)) {
		return "enabled", nil
	}
	return "disabled", nil
}

func filepathJoin(elem ...string) string {
	return strings.Join(elem, string(os.PathSeparator))
}

func cronExpression(interval string, hour int) string {
	if interval == "weekly" {
		return fmt.Sprintf("0 %d * * 1", hour)
	}
	return fmt.Sprintf("0 %d * * *", hour)
}

func cronMarker(task Task) string {
	return fmt.Sprintf("# oct-managed:%s", task)
}
