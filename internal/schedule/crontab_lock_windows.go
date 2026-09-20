//go:build windows

package schedule

// lockCrontab is a no-op on windows: the crontab scheduler only runs on Linux.
func lockCrontab() (func(), error) {
	return func() {}, nil
}
