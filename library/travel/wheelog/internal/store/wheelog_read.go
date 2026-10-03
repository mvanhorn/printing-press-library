// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// WheelogSnapshotError refuses to return an older checkpoint as saved evidence.
type WheelogSnapshotError struct{ Reason string }

func (e *WheelogSnapshotError) Error() string {
	return "cache_visibility_unavailable: " + e.Reason + ". Close other database writers, then retry the saved read. Source discovery and inspection can use --data-source live."
}

type wheelogReadGuard struct {
	path               string
	selection          string
	main, wal, journal os.FileInfo
}

func wheelogFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return info, err
}

func sameWheelogFile(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime()) && a.Mode() == b.Mode()
}

func wheelogReadState(path string, stat func(string) (os.FileInfo, error)) (*wheelogReadGuard, error) {
	main, err := stat(path)
	if err != nil {
		return nil, err
	}
	wal, err := stat(path + "-wal")
	if err != nil {
		return nil, err
	}
	journal, err := stat(path + "-journal")
	if err != nil {
		return nil, err
	}
	mainAfter, err := stat(path)
	if err != nil {
		return nil, err
	}
	if !sameWheelogFile(main, mainAfter) {
		return nil, &WheelogSnapshotError{Reason: "the saved database changed while its visibility was checked"}
	}
	if main != nil && !main.Mode().IsRegular() {
		return nil, fmt.Errorf("saved cache must be a regular database file")
	}
	if main != nil {
		singleLink, err := wheelogFileHasSingleLink(path, main)
		if err != nil {
			return nil, &WheelogSnapshotError{Reason: "saved database hard-link visibility is unavailable: " + err.Error()}
		}
		if !singleLink {
			return nil, &WheelogSnapshotError{Reason: "the saved database does not have exactly one hard link, so its WAL path cannot be verified"}
		}
	}
	if wal != nil && wal.Size() > 0 {
		return nil, &WheelogSnapshotError{Reason: "a non-empty WAL may contain newer committed observations"}
	}
	if journal != nil && journal.Size() > 0 {
		return nil, &WheelogSnapshotError{Reason: "a non-empty rollback journal prevents a stable saved snapshot"}
	}
	if main == nil && (wal != nil || journal != nil) {
		return nil, &WheelogSnapshotError{Reason: "database sidecars exist without the main saved database"}
	}
	return &wheelogReadGuard{path: path, main: main, wal: wal, journal: journal}, nil
}

func (g *wheelogReadGuard) check(stat func(string) (os.FileInfo, error)) error {
	after, err := wheelogReadState(g.path, stat)
	if err != nil {
		return err
	}
	if !sameWheelogFile(g.main, after.main) || !sameWheelogFile(g.wal, after.wal) || !sameWheelogFile(g.journal, after.journal) {
		return &WheelogSnapshotError{Reason: "the saved database or its sidecars changed during this read"}
	}
	if g.selection != "" {
		selected, err := stat(g.selection)
		if err != nil {
			return err
		}
		if !sameWheelogFile(g.main, selected) {
			return &WheelogSnapshotError{Reason: "the selected saved database path changed during this read"}
		}
	}
	return nil
}

// ReadWheelogSnapshot reads without migrations, permission changes or table
// creation. The framework's immutable reader avoids WAL-index mapping faults;
// this guard rejects active WAL/journal state and verifies the whole read so
// that uncheckpointed committed facts are never silently omitted.
func ReadWheelogSnapshot(ctx context.Context, path string) ([]WheelogObservation, error) {
	return readWheelogSnapshot(ctx, path, wheelogSnapshotRows)
}

func readWheelogSnapshot(ctx context.Context, path string, query func(context.Context, *Store) ([]WheelogObservation, error)) ([]WheelogObservation, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	selected, selectErr := os.Lstat(abs)
	if selectErr != nil && !os.IsNotExist(selectErr) {
		return nil, selectErr
	}
	target := abs
	if selected != nil {
		target, err = filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, &WheelogSnapshotError{Reason: "the selected saved database target is unavailable"}
		}
	}
	guard, err := wheelogReadState(target, wheelogFileInfo)
	if err != nil {
		return nil, err
	}
	guard.selection = abs
	if err := guard.check(wheelogFileInfo); err != nil {
		return nil, err
	}
	if guard.main == nil {
		if err := guard.check(wheelogFileInfo); err != nil {
			return nil, err
		}
		return []WheelogObservation{}, nil
	}
	snapshot, cleanup, err := pinWheelogSnapshot(ctx, target, guard)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	uri := url.URL{Path: filepath.ToSlash(snapshot)}
	reader, err := OpenReadOnlyContext(ctx, uri.EscapedPath())
	if err != nil {
		return nil, err
	}
	items, err := query(ctx, reader)
	closeErr := reader.Close()
	if checkErr := guard.check(wheelogFileInfo); checkErr != nil {
		return nil, checkErr
	}
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return items, nil
}

func wheelogSnapshotRows(ctx context.Context, reader *Store) ([]WheelogObservation, error) {
	var exists int
	if err := reader.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='wheelog_shortlist'").Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return []WheelogObservation{}, nil
	}
	return reader.WheelogList(ctx)
}
