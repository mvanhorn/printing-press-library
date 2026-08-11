// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
//go:build windows

package cli

// withLedgerLock is a no-op on Windows (no portable flock); the atomic
// temp+rename ledger write still prevents torn files, so concurrent writers
// can at worst lose an update, never corrupt the ledger.
func withLedgerLock(dir string, fn func() error) error {
	return fn()
}
