// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SnowJapanReadOnlyStore keeps the emitted immutable reader behind a whole-read
// filesystem guard. Immutable SQLite cannot observe an open writer's WAL;
// sidecars or path changes therefore fail the factual read instead of returning
// an older observation. No generated store implementation is changed.
type SnowJapanReadOnlyStore struct {
	*Store
	selected, canonical string
	original            os.FileInfo
}

func OpenSnowJapanReadOnlyContext(ctx context.Context, path string) (*SnowJapanReadOnlyStore, error) {
	selected, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(selected)
	if err != nil {
		return nil, fmt.Errorf("resolving saved database: %w", err)
	}
	// The emitted URI builder accepts ordinary paths, not URI punctuation.
	if strings.ContainsAny(canonical, "%?#") {
		return nil, fmt.Errorf("saved database path contains unsupported SQLite URI punctuation")
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return nil, err
	}
	reader := &SnowJapanReadOnlyStore{selected: selected, canonical: canonical, original: info}
	if err := reader.check(); err != nil {
		return nil, err
	}
	reader.Store, err = OpenReadOnlyContext(ctx, canonical)
	if err != nil {
		return nil, err
	}
	// Bind the actual SQLite handle now, inside the identity checks. Keep
	// the single connection idle between queries, with no expiry, so it
	// cannot lazily bind a replacement file after this factory returns.
	reader.DB().SetMaxOpenConns(1)
	reader.DB().SetMaxIdleConns(1)
	reader.DB().SetConnMaxLifetime(0)
	reader.DB().SetConnMaxIdleTime(0)
	if err := reader.DB().PingContext(ctx); err != nil {
		return nil, errors.Join(err, reader.Store.Close())
	}
	if err := reader.check(); err != nil {
		return nil, errors.Join(err, reader.Store.Close())
	}
	return reader, nil
}

func (r *SnowJapanReadOnlyStore) check() error {
	canonical, err := filepath.EvalSymlinks(r.selected)
	if err != nil || canonical != r.canonical {
		return fmt.Errorf("saved database path identity changed during read")
	}
	for _, path := range []string{r.selected, r.canonical} {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("checking saved database identity: %w", err)
		}
		if !info.Mode().IsRegular() || !os.SameFile(r.original, info) || info.Size() != r.original.Size() || !info.ModTime().Equal(r.original.ModTime()) {
			return fmt.Errorf("saved database changed during read; finish the writer and retry")
		}
		links, err := snowJapanFileLinks(path, info)
		if err != nil {
			return fmt.Errorf("checking saved database aliases: %w", err)
		}
		if links != 1 {
			return fmt.Errorf("saved database has ambiguous hard-link aliases; use a database file with one link")
		}
		for _, suffix := range []string{"-wal", "-shm", "-journal"} {
			_, err := os.Lstat(path + suffix)
			if err == nil {
				return fmt.Errorf("saved database has an active journal or WAL sidecar; close the writer before reading local facts")
			}
			if !os.IsNotExist(err) {
				return fmt.Errorf("checking saved database sidecar: %w", err)
			}
		}
	}
	return nil
}

// Close checks the same selected/canonical identity after every query. Callers
// must propagate this error before printing results, including early returns.
func (r *SnowJapanReadOnlyStore) Close() error {
	return errors.Join(r.Store.Close(), r.check())
}
