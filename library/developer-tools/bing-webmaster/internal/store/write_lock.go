// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

// Write-lock helpers for the SQLite store.
//
// Merge-reconciliation shim (reprint 2026-09-05, press 4.31.1): the refreshed
// internal/store/candidates.go serializes writers through
// lockForWrite/unlockAfterWrite, while the preserved store.go predates that
// API. These definitions mirror the current generator template verbatim so
// the preserved store keeps working unchanged. A future from-scratch reprint
// that rebuilds the store layer absorbs this file naturally.

package store

import (
	"os"
)

// hardenSQLiteFiles is best-effort so stores on filesystems without Unix modes
// remain usable. The deferred call catches files the SQLite driver creates.
func hardenSQLiteFiles(dbPath string) {
	for _, path := range []string{dbPath, dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}

		file, err := os.Open(path)
		if err != nil {
			continue
		}
		openInfo, statErr := file.Stat()
		pathInfo, lstatErr := os.Lstat(path)
		if statErr == nil && lstatErr == nil && pathInfo.Mode().IsRegular() && os.SameFile(openInfo, pathInfo) {
			_ = file.Chmod(0o600)
		}
		_ = file.Close()
	}
}

// lockForWrite serializes a DB write and hardens the SQLite files for the
// lifetime of every serialized writer. TRUNCATE journaling reuses its journal
// file and can restore its mode when a later transaction starts, after the
// one-time OpenWithContext hardening has already run.
func (s *Store) lockForWrite() {
	s.writeMu.Lock()
	hardenSQLiteFiles(s.path)
}

func (s *Store) unlockAfterWrite() {
	hardenSQLiteFiles(s.path)
	s.writeMu.Unlock()
}
