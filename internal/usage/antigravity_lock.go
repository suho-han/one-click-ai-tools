package usage

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// lockAntigravityUsage serializes the cache check, agy invocation, and cache
// update across oct processes. The stable sidecar remains in place after a
// process exits; the OS releases its advisory lock even on a crash.
func lockAntigravityUsage(ctx context.Context) (func(), error) {
	path := antigravityCachePath() + ".lock"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	var retry *time.Ticker
	defer func() {
		if retry != nil {
			retry.Stop()
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			file.Close()
			return nil, err
		}
		unlock, acquired, err := tryLockAntigravityFile(file)
		if err != nil {
			file.Close()
			return nil, err
		}
		if acquired {
			return func() { unlock(); file.Close() }, nil
		}
		if retry == nil {
			retry = time.NewTicker(25 * time.Millisecond)
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, ctx.Err()
		case <-retry.C:
		}
	}
}
