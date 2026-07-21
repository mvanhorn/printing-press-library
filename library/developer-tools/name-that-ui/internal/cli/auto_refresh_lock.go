// Copyright 2026 HenryBranchAdams and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored preservation for concurrent NameThatUI auto-refresh.

package cli

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	autoRefreshLockPollInterval = 50 * time.Millisecond
	autoRefreshLockStaleAfter   = 2 * time.Minute
)

// autoRefreshLock is a process-shared, database-adjacent lease. O_EXCL works
// on Windows, macOS, and Linux; the token prevents a delayed owner from
// deleting a successor's lock after stale-lock recovery.
type autoRefreshLock struct {
	path  string
	token string
}

func acquireAutoRefreshLock(ctx context.Context, dbPath string) (*autoRefreshLock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating auto-refresh lock directory: %w", err)
	}
	deadline := time.Now().Add(refreshTimeout())
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	path := dbPath + ".auto-refresh.lock"
	token, err := newAutoRefreshLockToken()
	if err != nil {
		return nil, err
	}

	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, writeErr := io.WriteString(file, token+"\n")
			closeErr := file.Close()
			if writeErr != nil || closeErr != nil {
				_ = os.Remove(path)
				if writeErr != nil {
					return nil, fmt.Errorf("writing auto-refresh lock: %w", writeErr)
				}
				return nil, fmt.Errorf("closing auto-refresh lock: %w", closeErr)
			}
			return &autoRefreshLock{path: path, token: token}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("creating auto-refresh lock: %w", err)
		}

		if err := recoverStaleAutoRefreshLock(path, token); err != nil {
			return nil, err
		}
		if err := waitForAutoRefreshLock(ctx, deadline); err != nil {
			return nil, err
		}
	}
}

func newAutoRefreshLockToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating auto-refresh lock token: %w", err)
	}
	return fmt.Sprintf("%x", bytes), nil
}

// recoverStaleAutoRefreshLock moves, rather than removes, an expired lock.
// An old owner then finds its original path absent during Release and cannot
// remove the replacement lock a waiter may already have acquired.
func recoverStaleAutoRefreshLock(path, token string) error {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("statting auto-refresh lock: %w", err)
	}
	if time.Since(info.ModTime()) < autoRefreshLockStaleAfter {
		return nil
	}

	quarantine := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".stale-"+token)
	if err := os.Rename(path, quarantine); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("recovering stale auto-refresh lock: %w", err)
	}
	_ = os.Remove(quarantine)
	return nil
}

func waitForAutoRefreshLock(ctx context.Context, deadline time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	wait := time.Until(deadline)
	if wait <= 0 {
		// A timer and context cancellation can become ready in the same
		// scheduler tick. Preserve the caller's cancellation contract even
		// when ctx.Err has not been published at the instant we inspect it.
		if contextDeadline, ok := ctx.Deadline(); ok && !contextDeadline.After(deadline) {
			return context.DeadlineExceeded
		}
		return fmt.Errorf("timed out waiting for auto-refresh lock after %s", refreshTimeout())
	}
	if wait > autoRefreshLockPollInterval {
		wait = autoRefreshLockPollInterval
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (l *autoRefreshLock) Release() {
	if l == nil {
		return
	}
	contents, err := os.ReadFile(l.path)
	if err != nil || string(contents) != l.token+"\n" {
		return
	}
	_ = os.Remove(l.path)
}
