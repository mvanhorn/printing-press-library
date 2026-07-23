//go:build !windows

// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored companion to private_file_windows.go.

package cliutil

// RestrictPrivateFile is a no-op on Unix: the Chmod(0600) inside
// AtomicWritePrivateFile already enforces owner-only access.
func RestrictPrivateFile(string) error { return nil }
