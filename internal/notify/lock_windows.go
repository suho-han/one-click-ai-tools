//go:build windows

package notify

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const lockfileExclusiveLock = 0x2 // LOCKFILE_EXCLUSIVE_LOCK: blocking, exclusive

// lockStateFile takes an exclusive byte-range lock on a stable lockfile next
// to the alert state so concurrent processes serialize their
// read-modify-write. Without it, Windows rename-onto-target fails with
// "Access is denied" while another goroutine holds the state file open.
// The returned func releases the lock.
func lockStateFile(path string) (func(), error) {
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	// Lock the full 64-bit range; blocking wait (no FAIL_IMMEDIATELY),
	// matching the unix flock semantics.
	var overlapped syscall.Overlapped
	r1, _, callErr := procLockFileEx.Call(
		f.Fd(),
		lockfileExclusiveLock,
		0,
		0xFFFFFFFF,
		0xFFFFFFFF,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if r1 == 0 {
		f.Close()
		return nil, callErr
	}
	return func() {
		procUnlockFileEx.Call(
			f.Fd(),
			0,
			0xFFFFFFFF,
			0xFFFFFFFF,
			uintptr(unsafe.Pointer(&overlapped)),
		)
		f.Close()
	}, nil
}
