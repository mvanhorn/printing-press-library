// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
)

// OpenWheelogWithContext binds writes to the same canonical target as saved
// reads. Opening a hard-link alias can create a different WAL and lose a save
// when another writer checkpoints; reject aliases before writable open.
func OpenWheelogWithContext(ctx context.Context, path string) (*Store, error) {
	if strings.ContainsAny(path, "?#") {
		return nil, &WheelogSnapshotError{Reason: "writable saved paths cannot contain SQLite URI delimiters '?' or '#'"}
	}
	selection, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	selected, err := os.Lstat(selection)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var target string
	if selected != nil {
		target, err = filepath.EvalSymlinks(selection)
		if err != nil {
			return nil, &WheelogSnapshotError{Reason: "the selected saved database target is unavailable"}
		}
	} else {
		if err = os.MkdirAll(filepath.Dir(selection), 0700); err != nil {
			return nil, err
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(selection))
		if err != nil {
			return nil, err
		}
		target = filepath.Join(parent, filepath.Base(selection))
	}
	before, err := wheelogFileInfo(target)
	if strings.ContainsAny(target, "?#") {
		return nil, &WheelogSnapshotError{Reason: "writable saved targets cannot contain SQLite URI delimiters '?' or '#'"}
	}
	if err != nil {
		return nil, err
	}
	if before != nil {
		if err := wheelogWriteIdentity(selection, target, before); err != nil {
			return nil, err
		}
	}
	db, err := OpenWithContext(ctx, target)
	if err != nil {
		return nil, err
	}
	if before == nil {
		before, err = os.Stat(target)
	}
	if err == nil {
		err = wheelogWriteIdentity(selection, target, before)
	}
	if err != nil {
		_ = db.Close() // Preserve the selected-path failure, never claim a saved observation.
		return nil, err
	}
	db.wheelogWriteCheck = func() error { return wheelogWriteIdentity(selection, target, before) }
	// Domain operations share one reserved connection instead of letting later
	// transactions lazily open another selected-path connection.
	db.db.SetMaxOpenConns(1)
	db.wheelogConn, err = db.db.Conn(ctx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = db.checkWheelogWritePath(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func (s *Store) wheelogQuery(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if s.wheelogConn != nil {
		return s.wheelogConn.QueryContext(ctx, query, args...)
	}
	return s.db.QueryContext(ctx, query, args...)
}

func (s *Store) wheelogExec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.wheelogConn != nil {
		return s.wheelogConn.ExecContext(ctx, query, args...)
	}
	return s.db.ExecContext(ctx, query, args...)
}

func (s *Store) wheelogBegin(ctx context.Context) (*sql.Tx, error) {
	if s.wheelogConn != nil {
		return s.wheelogConn.BeginTx(ctx, nil)
	}
	return s.db.BeginTx(ctx, nil)
}

func wheelogWriteIdentity(selection, target string, expected os.FileInfo) error {
	main, err := os.Stat(target)
	if err != nil {
		return err
	}
	selected, err := os.Stat(selection)
	if err != nil {
		return err
	}
	if !main.Mode().IsRegular() || !os.SameFile(expected, main) || !os.SameFile(main, selected) {
		return &WheelogSnapshotError{Reason: "the selected saved database identity changed during a write"}
	}
	singleLink, err := wheelogFileHasSingleLink(target, main)
	if err != nil {
		return &WheelogSnapshotError{Reason: "saved database hard-link visibility is unavailable: " + err.Error()}
	}
	if !singleLink {
		return &WheelogSnapshotError{Reason: "the saved database does not have exactly one hard link, so its WAL path cannot be verified"}
	}
	return nil
}

func (s *Store) checkWheelogWritePath() error {
	if s.wheelogWriteCheck != nil {
		return s.wheelogWriteCheck()
	}
	return nil
}
