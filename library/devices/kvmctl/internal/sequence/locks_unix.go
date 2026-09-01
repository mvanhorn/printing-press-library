//go:build !windows

package sequence

import (
	"os"
	"syscall"
)

func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
}

// openSharedLockFile opens the cross-user device lock. The permissive 0666 mode
// (subject to umask on creation, corrected by the caller's chmod) is required so
// a different OS user can open the same inode and contend on flock. O_NOFOLLOW
// still refuses to traverse a symlink planted in the shared directory.
func openSharedLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0666)
}

func lockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

func unlockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
