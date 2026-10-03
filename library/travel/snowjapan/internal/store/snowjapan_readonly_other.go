//go:build !linux && !darwin && !windows

package store

import (
	"fmt"
	"os"
)

func snowJapanFileLinks(_ string, _ os.FileInfo) (uint64, error) {
	return 0, fmt.Errorf("filesystem alias verification is unsupported on this platform")
}
