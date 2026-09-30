// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// errPostmarkSyncLockBusy is returned by tryLockPostmarkSync when another
// process holds the lock.
var errPostmarkSyncLockBusy = errors.New("sync lock busy")

// acquirePostmarkSyncLock takes an exclusive lock on path, retrying until it
// is free or ctx ends. The operating system drops the lock when the holding
// process exits, so a crashed sync cannot leave it held.
func acquirePostmarkSyncLock(ctx context.Context, path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- lock file beside the CLI's own database
	if err != nil {
		return nil, fmt.Errorf("opening sync lock %s: %w", path, err)
	}
	for {
		err := tryLockPostmarkSync(f)
		if err == nil {
			return func() {
				_ = unlockPostmarkSync(f)
				_ = f.Close()
			}, nil
		}
		if !errors.Is(err, errPostmarkSyncLockBusy) {
			_ = f.Close()
			return nil, fmt.Errorf("locking %s: %w", path, err)
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, fmt.Errorf("another sync is still using this archive (lock %s): %w", path, ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
}
