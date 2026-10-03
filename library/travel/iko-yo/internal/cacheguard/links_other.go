//go:build !unix && !windows

// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cacheguard

import (
	"fmt"
	"os"
)

func linkCount(_ string, _ os.FileInfo) (uint64, error) {
	return 0, fmt.Errorf("cache link identity is unsupported on this platform")
}
