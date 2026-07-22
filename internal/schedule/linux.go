package schedule

import (
	"fmt"
	"os"
	"path"
	"strings"
)

type Linux struct{}

func (l *Linux) Enable(task Task, interval string, hour int) error {
	interval, err := validateScheduleTiming(interval, hour)
	if err != nil {
		return err
	}
	cfg, err := taskDetails(task)
	if err != nil {
		return err
	}

	binPath := resolveBinaryPath()

	home, _ := homeDirPath()
	logPath := linuxLogPath(home, cfg.LogFile)
	os.MkdirAll(path.Dir(logPath), 0o755)

	cronExpr := cronExpression(interval, hour)
	cronEntry := fmt.Sprintf("%s %s %s >> %s 2>&1 %s", cronExpr, shellQuote(binPath), shellQuote(cfg.Command), shellQuote(logPath), cronMarker(task))

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

func linuxLogPath(home, logFile string) string {
	return path.Join(home, ".oct", "logs", logFile)
}

func filepathJoin(elem ...string) string {
	return path.Join(elem...)
}

func cronExpression(interval string, hour int) string {
	switch interval {
	case WeeklyInterval:
		return fmt.Sprintf("0 %d * * 1", hour)
	case DailyInterval:
		return fmt.Sprintf("0 %d * * *", hour)
	case TwelveHourInterval:
		return "0 */12 * * *"
	case SixHourInterval:
		return "0 */6 * * *"
	case OneHourInterval:
		return "0 * * * *"
	default:
		return fmt.Sprintf("0 %d * * *", hour)
	}
}

func cronMarker(task Task) string {
	return fmt.Sprintf("# oct-managed:%s", task)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
