// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// HAND-WRITTEN — not generated. Locks down the SQLite store directory and
// file permissions for personal nutrition/weight data. Approved at the Phase
// 1.5 manifest gate (commitment C-1) — narrows the generator's default
// 0o755/umask to 0o700/0o600 to match the cookie file's posture.

package store

import (
	"os"
	"path/filepath"
)

// SecurePerms applies owner-only permissions to the store directory and the
// SQLite file at dbPath. It is idempotent — safe to call on every Open.
//
// Recovers from any user chmod drift; the cost is two stat+chmod calls per
// open, both no-ops when the modes already match.
func SecurePerms(dbPath string) error {
	if dbPath == "" {
		return nil
	}
	if err := os.Chmod(filepath.Dir(dbPath), 0o700); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Chmod(dbPath, 0o600); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
