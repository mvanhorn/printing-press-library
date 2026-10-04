// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SnowJapanWritableStore ensures a source capture uses one canonical database
// and its WAL. A hard-link alias could create a second WAL for the same inode
// and lose another writer's observations, so aliases are rejected before open
// and before/after each save. Symlink aliases use the real database path.
type SnowJapanWritableStore struct {
	*Store
	selected, canonical string
	original            os.FileInfo
	closed              bool
}

func snowJapanWritablePath(path string) (selected, canonical string, info os.FileInfo, err error) {
	selected, err = filepath.Abs(path)
	if err != nil {
		return
	}
	canonical, err = filepath.EvalSymlinks(selected)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
		ancestor := selected
		var tail []string
		for {
			_, statErr := os.Lstat(ancestor)
			if statErr == nil {
				break
			}
			if !os.IsNotExist(statErr) {
				err = statErr
				return
			}
			tail = append([]string{filepath.Base(ancestor)}, tail...)
			parent := filepath.Dir(ancestor)
			if parent == ancestor {
				err = fmt.Errorf("saved database has no resolvable parent")
				return
			}
			ancestor = parent
		}
		var resolved string
		resolved, err = filepath.EvalSymlinks(ancestor)
		if err != nil {
			return
		} // Reject dangling symlinks rather than creating through them.
		canonical = filepath.Join(append([]string{resolved}, tail...)...)
	}
	if strings.ContainsAny(canonical, "%?#") {
		err = fmt.Errorf("saved database path contains unsupported SQLite URI punctuation")
		return
	}
	info, err = os.Stat(canonical)
	if os.IsNotExist(err) {
		info = nil
		err = nil
	}
	return
}

func OpenSnowJapanWritableContext(ctx context.Context, path string) (*SnowJapanWritableStore, error) {
	selected, canonical, original, err := snowJapanWritablePath(path)
	if err != nil {
		return nil, err
	}
	if original == nil {
		if err := os.MkdirAll(filepath.Dir(canonical), 0o700); err != nil {
			return nil, err
		}
		parent, err := os.OpenRoot(filepath.Dir(canonical))
		if err != nil {
			return nil, err
		}
		file, err := parent.OpenFile(filepath.Base(canonical), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		parentErr := parent.Close()
		if err != nil {
			return nil, errors.Join(fmt.Errorf("creating selected saved database: %w", err), parentErr)
		}
		original, err = file.Stat()
		if err = errors.Join(err, parentErr, file.Close()); err != nil {
			return nil, err
		}
	}
	writer := &SnowJapanWritableStore{selected: selected, canonical: canonical, original: original}
	if err := writer.check(); err != nil {
		return nil, err
	}
	writer.Store, err = OpenWithContext(ctx, canonical)
	if err != nil {
		return nil, err
	}
	writer.DB().SetMaxOpenConns(1)
	writer.DB().SetMaxIdleConns(1)
	writer.DB().SetConnMaxLifetime(0)
	writer.DB().SetConnMaxIdleTime(0)
	if err := writer.DB().PingContext(ctx); err != nil {
		return nil, errors.Join(err, writer.Store.Close())
	}
	if err := writer.check(); err != nil {
		return nil, errors.Join(err, writer.Store.Close())
	}
	return writer, nil
}

func (w *SnowJapanWritableStore) check() error {
	canonical, err := filepath.EvalSymlinks(w.selected)
	if err != nil || canonical != w.canonical {
		return fmt.Errorf("saved database path identity changed during capture")
	}
	for _, path := range []string{w.selected, w.canonical} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || !os.SameFile(w.original, info) {
			return fmt.Errorf("saved database identity changed during capture")
		}
		links, err := snowJapanFileLinks(path, info)
		if err != nil {
			return err
		}
		if links != 1 {
			return fmt.Errorf("saved database has ambiguous hard-link aliases; capture into a database file with one link")
		}
	}
	// Parent-directory symlinks name the same sidecars and are safe. A
	// final-file symlink must not hide a separately named alias WAL/journal.
	if w.selected != w.canonical {
		for _, suffix := range []string{"-wal", "-shm", "-journal"} {
			alias, err := os.Stat(w.selected + suffix)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			real, err := os.Stat(w.canonical + suffix)
			if err != nil || !os.SameFile(alias, real) {
				return fmt.Errorf("saved database alias has a separate journal or WAL; close its writer before capture")
			}
		}
	}
	return nil
}

func (w *SnowJapanWritableStore) UpsertBatch(resource string, facts []json.RawMessage) (stored, failed int, err error) {
	if err = w.check(); err != nil {
		return
	}
	stored, failed, err = w.Store.UpsertBatch(resource, facts)
	err = errors.Join(err, w.check())
	return
}
func (w *SnowJapanWritableStore) CaptureSnowJapan(ctx context.Context, facts []json.RawMessage, full bool) error {
	if err := w.check(); err != nil {
		return err
	}
	return errors.Join(w.Store.CaptureSnowJapan(ctx, facts, full), w.check())
}
func (w *SnowJapanWritableStore) CaptureSnowJapanSeason(ctx context.Context, season string, facts []json.RawMessage) error {
	if err := w.check(); err != nil {
		return err
	}
	return errors.Join(w.Store.CaptureSnowJapanSeason(ctx, season, facts), w.check())
}
func (w *SnowJapanWritableStore) SaveSyncState(resource, cursor string, count int) error {
	if err := w.check(); err != nil {
		return err
	}
	return errors.Join(w.Store.SaveSyncState(resource, cursor, count), w.check())
}
func (w *SnowJapanWritableStore) Close() error {
	if w.closed {
		return nil
	}
	before := w.check()
	err := errors.Join(before, w.Store.Close(), w.check())
	w.closed = true
	return err
}
