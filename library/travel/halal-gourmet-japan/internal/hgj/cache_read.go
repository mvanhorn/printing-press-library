// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"fmt"
	"os"
)

// CacheVisibilityError refuses to label a potentially older checkpoint as current.
type CacheVisibilityError struct{ Reason string }

func (e *CacheVisibilityError) Error() string {
	return "cache_visibility_unavailable: " + e.Reason + ". Finish all inspections and close other database writers, then retry saved reads; live inspection with --no-cache remains available."
}

type SavedReadGuard struct {
	path      string
	main, wal os.FileInfo
}

func cacheFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return info, err
}

// BeginSavedRead keeps the framework's immutable reader while refusing active WAL state.
func BeginSavedRead(path string) (*SavedReadGuard, error) { return savedReadState(path, cacheFileInfo) }

func savedReadState(path string, stat func(string) (os.FileInfo, error)) (*SavedReadGuard, error) {
	main, err := stat(path)
	if err != nil {
		return nil, err
	}
	wal, err := stat(path + "-wal")
	if err != nil {
		return nil, err
	}
	mainAfter, err := stat(path)
	if err != nil {
		return nil, err
	}
	if !sameCacheFile(main, mainAfter) {
		return nil, &CacheVisibilityError{Reason: "the cache changed while its visibility was checked"}
	}
	if wal != nil && wal.Size() > 0 {
		return nil, &CacheVisibilityError{Reason: "the saved cache has a non-empty WAL and may omit completed observations until its writers close"}
	}
	return &SavedReadGuard{path: path, main: main, wal: wal}, nil
}
func sameCacheFile(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// Check covers the entire read, including every selected place and both history positions.
func (g *SavedReadGuard) Check() error { return g.checkWithStat(cacheFileInfo) }

func (g *SavedReadGuard) checkWithStat(stat func(string) (os.FileInfo, error)) error {
	after, err := savedReadState(g.path, stat)
	if err != nil {
		return err
	}
	if !sameCacheFile(g.main, after.main) || !sameCacheFile(g.wal, after.wal) {
		return &CacheVisibilityError{Reason: "the saved cache changed during this read"}
	}
	if g.main != nil && !g.main.Mode().IsRegular() {
		return fmt.Errorf("saved cache must be a regular database file")
	}
	return nil
}
