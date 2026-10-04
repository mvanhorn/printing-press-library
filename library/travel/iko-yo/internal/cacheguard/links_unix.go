//go:build unix

// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cacheguard

import (
	"fmt"
	"os"
	"syscall"
)

func linkCount(_ string, info os.FileInfo) (uint64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("file link count unavailable")
	}
	return uint64(stat.Nlink), nil
}
