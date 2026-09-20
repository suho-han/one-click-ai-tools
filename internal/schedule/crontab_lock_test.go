//go:build !windows

package schedule

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLockCrontab_ExcludesSecondHolderUntilReleased(t *testing.T) {
	origHome := homeDirPath
	t.Cleanup(func() { homeDirPath = origHome })
	home := t.TempDir()
	homeDirPath = func() (string, error) { return home, nil }

	unlock, err := lockCrontab()
	if err != nil {
		t.Fatalf("lockCrontab() error: %v", err)
	}

	// A second independent open of the same lockfile must not acquire the
	// exclusive lock while the first holder is inside the critical section.
	second, err := os.OpenFile(filepath.Join(home, ".oct", "crontab.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open second lock fd: %v", err)
	}
	defer second.Close()
	if err := syscall.Flock(int(second.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		t.Fatal("second exclusive lock succeeded while first is held; want it blocked")
	}

	unlock()

	if err := syscall.Flock(int(second.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("second exclusive lock after release: %v", err)
	}
	if err := syscall.Flock(int(second.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("unlock second fd: %v", err)
	}
}
