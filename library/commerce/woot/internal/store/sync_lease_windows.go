// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.
//go:build windows

package store

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

type platformFileLock struct {
	overlapped windows.Overlapped
}

func tryPlatformFileLock(file *os.File) (platformFileLock, bool, error) {
	state := platformFileLock{}
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&state.overlapped,
	)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return platformFileLock{}, false, nil
	}
	if err != nil {
		return platformFileLock{}, false, err
	}
	return state, true, nil
}

func releasePlatformFileLock(file *os.File, state *platformFileLock) error {
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &state.overlapped)
}
