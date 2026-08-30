//go:build !windows

// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.

package platform

import "os"

func securePrivateFile(path string) error {
	return os.Chmod(path, 0o600)
}
