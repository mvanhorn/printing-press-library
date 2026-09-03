// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build !linux

package secureio

import "os"

func openRegularNoFollow(path string, flags int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flags, perm)
}

func openDirectoryNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
