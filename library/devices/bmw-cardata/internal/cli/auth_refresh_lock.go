// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/config"

	"github.com/gofrs/flock"
)

// withCardataRefreshLock coordinates refresh across CLI and MCP processes.
// The caller reloads the config after acquiring the lock, since another
// process may already have rotated the single-use refresh token.
func withCardataRefreshLock(ctx context.Context, configPath string, fn func() error) (err error) {
	if configPath == "" {
		return fmt.Errorf("cannot refresh OAuth credentials without a config path")
	}
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolving OAuth config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return fmt.Errorf("creating OAuth config directory: %w", err)
	}
	canonicalPath, err := config.CanonicalPath(absPath)
	if err != nil {
		return fmt.Errorf("resolving OAuth config path: %w", err)
	}
	lock := flock.New(canonicalPath+".refresh.lock", flock.SetPermissions(0o600))
	locked, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("waiting for OAuth refresh lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("OAuth refresh lock was not acquired")
	}
	defer func() {
		if unlockErr := lock.Unlock(); err == nil && unlockErr != nil {
			err = fmt.Errorf("releasing OAuth refresh lock: %w", unlockErr)
		}
	}()
	return fn()
}
