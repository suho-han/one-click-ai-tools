//go:build windows

package notify

// lockStateFile is a best-effort no-op on windows: advisory file locking is
// not available without additional OS APIs. Concurrent alert writers are
// uncommon there, and writeFileAtomic keeps the state file itself from ever
// being observed truncated.
func lockStateFile(path string) (func(), error) {
	return func() {}, nil
}
