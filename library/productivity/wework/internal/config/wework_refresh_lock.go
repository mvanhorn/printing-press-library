// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored cross-process ownership for WeWork's rotating refresh-token
// chain. A refresh token may be consumed only once, so every process reloads
// the latest persisted rotation after acquiring the same private lock.

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/cliutil"
)

const (
	weworkRefreshLockWait  = 30 * time.Second
	weworkRefreshLockStale = 2 * time.Minute
	weworkRefreshLockPoll  = 25 * time.Millisecond
)

func (c *Config) acquireWeworkRefreshLock() (func(), error) {
	credentialsPath, err := c.weworkCredentialsPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(credentialsPath), 0o700); err != nil {
		return nil, fmt.Errorf("creating refresh-lock directory: %w", err)
	}
	lockPath := credentialsPath + ".wework-refresh.lock"
	deadline := time.Now().Add(weworkRefreshLockWait)
	for {
		if err := os.Mkdir(lockPath, 0o700); err == nil {
			return func() { _ = os.Remove(lockPath) }, nil
		} else if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquiring refresh lock: %w", err)
		}

		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > weworkRefreshLockStale {
			if removeErr := os.Remove(lockPath); removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
				continue
			}
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for another wework-pp-cli process to finish rotating the refresh token")
		}
		time.Sleep(weworkRefreshLockPoll)
	}
}

func (c *Config) weworkCredentialsPath() (string, error) {
	defaultDir, err := cliutil.ConfigDir()
	if err != nil {
		return "", err
	}
	defaultConfig := filepath.Join(defaultDir, "config.toml")
	if c == nil || c.Path == "" || filepath.Clean(c.Path) == filepath.Clean(defaultConfig) {
		return cliutil.CredentialsFilePath()
	}
	return cliutil.CredentialsFilePathForConfig(c.Path)
}

// reloadWeworkRotatingCredentials refreshes the in-memory copy after lock
// acquisition. Another process may have rotated and atomically persisted the
// token family while this process was waiting.
func (c *Config) reloadWeworkRotatingCredentials() error {
	if c == nil {
		return errors.New("nil config")
	}
	defaultDir, err := cliutil.ConfigDir()
	if err != nil {
		return err
	}
	defaultConfig := filepath.Join(defaultDir, "config.toml")
	var creds *cliutil.Credentials
	var ok bool
	if c.Path == "" || filepath.Clean(c.Path) == filepath.Clean(defaultConfig) {
		creds, ok, err = cliutil.LoadCredentials()
	} else {
		creds, ok, err = cliutil.LoadCredentialsForConfig(c.Path)
	}
	if err != nil {
		return fmt.Errorf("reloading rotated credentials: %w", err)
	}
	if !ok || creds == nil {
		return nil
	}
	if creds.WeworkToken != "" {
		c.WeworkToken = creds.WeworkToken
	}
	if creds.AccessToken != "" {
		c.AccessToken = creds.AccessToken
	}
	if creds.RefreshToken != "" {
		c.RefreshToken = creds.RefreshToken
	}
	if !creds.TokenExpiry.IsZero() {
		c.TokenExpiry = creds.TokenExpiry
	}
	return nil
}
