//go:build !windows

package sequence

import (
	"os"
	"syscall"
)

func openJournalFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND|syscall.O_NOFOLLOW, 0600)
}
