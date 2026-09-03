// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !windows

package approval

import (
	"errors"
	"os"
	"sync"
)

func acquireOperationFileLock(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, ErrOperationBusy
	}
	if err != nil {
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	var once sync.Once
	return func() {
		once.Do(func() { _ = os.Remove(path) })
	}, nil
}
