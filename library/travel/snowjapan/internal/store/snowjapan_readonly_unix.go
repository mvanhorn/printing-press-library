//go:build linux || darwin

package store

import (
	"fmt"
	"os"
	"syscall"
)

func snowJapanFileLinks(_ string, info os.FileInfo) (uint64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("filesystem link count is unavailable")
	}
	return uint64(stat.Nlink), nil
}
