package schedule

import (
	"fmt"
	"runtime"
)

type Scheduler interface {
	Enable(interval string, hour int) error
	Disable() error
	Status() (string, error)
}

func GetScheduler() (Scheduler, error) {
	switch runtime.GOOS {
	case "darwin":
		return &MacOS{Label: "com.oct.agent-update"}, nil
	case "linux":
		return &Linux{}, nil
	case "windows":
		return &Windows{}, nil
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
