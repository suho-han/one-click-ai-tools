package schedule

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type Task string

const (
	AgentUpdateTask    Task = "agent-update"
	SessionRefreshTask Task = "session-refresh"
)

const (
	WeeklyInterval     = "weekly"
	DailyInterval      = "daily"
	TwelveHourInterval = "12h"
	SixHourInterval    = "6h"
	OneHourInterval    = "1h"
)

type Scheduler interface {
	Enable(task Task, interval string, hour int) error
	Disable(task Task) error
	Status(task Task) (string, error)
}

type taskConfig struct {
	LabelSuffix string
	Command     string
	LogFile     string
}

var (
	executablePath   = os.Executable
	lookPath         = exec.LookPath
	homeDirPath      = os.UserHomeDir
	linuxCrontabList = func() ([]byte, error) {
		return exec.Command("crontab", "-l").Output()
	}
	linuxCrontabWrite = func(content string) error {
		cmd := exec.Command("crontab", "-")
		cmd.Stdin = strings.NewReader(content)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("crontab failed: %v, output: %s", err, string(output))
		}
		return nil
	}
	linuxCrontabRemove = func() error {
		return exec.Command("crontab", "-r").Run()
	}
)

func GetScheduler() (Scheduler, error) {
	switch runtime.GOOS {
	case "darwin":
		return &MacOS{LabelPrefix: "com.oct"}, nil
	case "linux":
		return &Linux{}, nil
	case "windows":
		return &Windows{}, nil
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func ParseTask(raw string) (Task, error) {
	task := Task(strings.ToLower(strings.TrimSpace(raw)))
	switch task {
	case AgentUpdateTask:
		return AgentUpdateTask, nil
	case SessionRefreshTask:
		return SessionRefreshTask, nil
	default:
		return "", fmt.Errorf("unsupported schedule task %q (use agent-update or session-refresh)", raw)
	}
}

func ParseInterval(raw string) (string, error) {
	interval := strings.ToLower(strings.TrimSpace(raw))
	switch interval {
	case "weekly", "week", "1w", "7d":
		return WeeklyInterval, nil
	case "daily", "day", "1d", "24h":
		return DailyInterval, nil
	case "12h", "12hour", "12hours":
		return TwelveHourInterval, nil
	case "6h", "6hour", "6hours":
		return SixHourInterval, nil
	case "1h", "hourly", "hour", "1hour":
		return OneHourInterval, nil
	default:
		return "", fmt.Errorf("unsupported interval %q (use weekly, daily, 12h, 6h, or 1h)", raw)
	}
}

func ParseHour(raw string) (int, error) {
	hour, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid hour %q: must be 0-23", raw)
	}
	if hour < 0 || hour > 23 {
		return 0, fmt.Errorf("invalid hour %q: must be 0-23", raw)
	}
	return hour, nil
}
func validateScheduleTiming(rawInterval string, hour int) (string, error) {
	interval, err := ParseInterval(rawInterval)
	if err != nil {
		return "", err
	}
	if hour < 0 || hour > 23 {
		return "", fmt.Errorf("invalid hour %d: must be 0-23", hour)
	}
	return interval, nil
}

func IntervalUsesHour(interval string) bool {
	return interval == WeeklyInterval || interval == DailyInterval
}

func FormatSchedule(interval string, hour int) string {
	if IntervalUsesHour(interval) {
		return fmt.Sprintf("%s, %02d:00", interval, hour)
	}
	return interval
}

func taskDetails(task Task) (taskConfig, error) {
	switch task {
	case AgentUpdateTask:
		return taskConfig{LabelSuffix: "agent-update", Command: string(AgentUpdateTask), LogFile: "agent-update.log"}, nil
	case SessionRefreshTask:
		return taskConfig{LabelSuffix: "session-refresh", Command: string(SessionRefreshTask), LogFile: "session-refresh.log"}, nil
	default:
		return taskConfig{}, fmt.Errorf("unsupported schedule task: %s", task)
	}
}

func resolveBinaryPath() string {
	if binPath, err := executablePath(); err == nil && binPath != "" {
		return binPath
	}
	if binPath, err := lookPath("oct"); err == nil {
		return binPath
	}
	return "oct"
}
