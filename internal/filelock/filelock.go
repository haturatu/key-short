package filelock

import "github.com/gofrs/flock"

// Acquire uses flock on Unix and LockFileEx on Windows. Closing the handle
// releases the lock, including after a crash; lock files are never deleted.
func Acquire(path string) (*flock.Flock, bool, error) {
	f := flock.New(path)
	locked, err := f.TryLock()
	if err != nil || !locked {
		f.Close()
	}
	return f, locked, err
}
