//go:build !windows

// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"syscall"
)

func creditBudgetFileID(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("journal has no Unix file identity")
	}
	return fmt.Sprintf("%d:%d", st.Dev, st.Ino), nil
}
