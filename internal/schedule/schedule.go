package schedule

import (
	"fmt"
	"runtime"
)

type Task string

const (
	AgentUpdateTask    Task = "agent-update"
	SessionRefreshTask Task = "session-refresh"
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
	switch Task(raw) {
	case AgentUpdateTask:
		return AgentUpdateTask, nil
	case SessionRefreshTask:
		return SessionRefreshTask, nil
	default:
		return "", fmt.Errorf("unsupported schedule task: %s", raw)
	}
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
