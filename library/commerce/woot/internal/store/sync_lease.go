// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrSyncLeaseHeld = errors.New("another sync is already using this database")

// SyncLease is an interprocess lock held for an entire sync lifecycle. The
// lock file remains on disk after release; the operating-system lock, not file
// existence, owns exclusivity and is released automatically if a process dies.
type SyncLease struct {
	file  *os.File
	state platformFileLock
}

func AcquireSyncLease(dbPath string) (*SyncLease, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("database path is required for sync lease")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating sync lease directory: %w", err)
	}
	file, err := os.OpenFile(dbPath+".sync.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening sync lease: %w", err)
	}
	state, acquired, err := tryPlatformFileLock(file)
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("acquiring sync lease: %w", err)
	}
	if !acquired {
		_ = file.Close()
		return nil, ErrSyncLeaseHeld
	}
	return &SyncLease{file: file, state: state}, nil
}

func (l *SyncLease) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := releasePlatformFileLock(l.file, &l.state)
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(unlockErr, closeErr)
}
