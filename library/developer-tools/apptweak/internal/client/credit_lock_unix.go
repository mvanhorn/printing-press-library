//go:build !windows

package client

import (
	"fmt"
	"os"
	"syscall"
)

func lockCreditJournal(f *os.File) (func() error, error) {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return nil, err
	}
	return func() error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }, nil
}

func creditJournalIdentity(f *os.File) (string, error) {
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("journal has no Unix file identity")
	}
	return fmt.Sprintf("%d:%d", st.Dev, st.Ino), nil
}

func creditJournalPrivate(_ *os.File, info os.FileInfo) error {
	if info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("Unix group or other permissions are present")
	}
	return nil
}
