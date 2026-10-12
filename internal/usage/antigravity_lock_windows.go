//go:build windows

package usage

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	antigravityKernel32   = syscall.NewLazyDLL("kernel32.dll")
	antigravityLockFile   = antigravityKernel32.NewProc("LockFileEx")
	antigravityUnlockFile = antigravityKernel32.NewProc("UnlockFileEx")
)

func tryLockAntigravityFile(file *os.File) (func(), bool, error) {
	const (
		exclusive       = 0x2
		failImmediately = 0x1
		lockViolation   = syscall.Errno(33)
	)
	var overlapped syscall.Overlapped
	r1, _, err := antigravityLockFile.Call(
		file.Fd(), exclusive|failImmediately, 0, 0xFFFFFFFF, 0xFFFFFFFF,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if r1 == 0 {
		if err == lockViolation {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() {
		antigravityUnlockFile.Call(
			file.Fd(), 0, 0xFFFFFFFF, 0xFFFFFFFF,
			uintptr(unsafe.Pointer(&overlapped)),
		)
	}, true, nil
}
