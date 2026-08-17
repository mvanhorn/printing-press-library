// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.
//go:build !windows

package store

import (
	"errors"
	"os"
	"syscall"
)

type platformFileLock struct{}

func tryPlatformFileLock(file *os.File) (platformFileLock, bool, error) {
	err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return platformFileLock{}, false, nil
	}
	if err != nil {
		return platformFileLock{}, false, err
	}
	return platformFileLock{}, true, nil
}

func releasePlatformFileLock(file *os.File, _ *platformFileLock) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
