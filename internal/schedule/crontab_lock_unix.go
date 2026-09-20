//go:build !windows

package schedule

import (
	"os"
	"path/filepath"
	"syscall"
)

// lockCrontab takes an exclusive advisory lock on a stable lockfile so two
// concurrent oct processes cannot interleave their crontab read-modify-write
// and drop each other's entries. Mirrors the notify state-file lock pattern.
// The returned func releases the lock.
func lockCrontab() (func(), error) {
	home, err := homeDirPath()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".oct")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "crontab.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
