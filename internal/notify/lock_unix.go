//go:build !windows

package notify

import (
	"os"
	"syscall"
)

// lockStateFile takes an exclusive advisory lock on a stable lockfile next to
// the alert state so concurrent processes serialize their read-modify-write.
// The returned func releases the lock.
func lockStateFile(path string) (func(), error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0644)
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
