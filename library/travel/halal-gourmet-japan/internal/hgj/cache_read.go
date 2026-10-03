// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

// CacheVisibilityError refuses to label a potentially older checkpoint as current.
type CacheVisibilityError struct{ Reason string }

func (e *CacheVisibilityError) Error() string {
	return "cache_visibility_unavailable: " + e.Reason + ". Finish all inspections and close other database writers, then retry saved reads; live inspection with --no-cache remains available."
}

type SavedReadGuard struct {
	path, canonical      string
	selected, main, wal  os.FileInfo
	snapshot, privateDir string
}

// Path is the canonical database target. The guard still tracks the selected
// path independently, so retargeting an alias cannot produce a successful read.
func (g *SavedReadGuard) Path() string {
	if g.snapshot != "" {
		return g.snapshot
	}
	return g.canonical
}

func (g *SavedReadGuard) Close() error {
	if g.privateDir == "" {
		return nil
	}
	err := os.RemoveAll(g.privateDir)
	g.privateDir, g.snapshot = "", ""
	return err
}

func cacheFileInfo(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return info, err
}

// BeginSavedRead keeps the framework's immutable reader while refusing active WAL state.
func BeginSavedRead(path string) (*SavedReadGuard, error) {
	return BeginSavedReadContext(context.Background(), path)
}

func BeginSavedReadContext(ctx context.Context, path string) (*SavedReadGuard, error) {
	g, err := savedReadState(path, cacheFileInfo)
	if err != nil {
		return nil, err
	}
	if g.main != nil {
		if err = g.snapshotWithOpen(ctx, os.Open); err != nil {
			return nil, err
		}
	}
	return g, nil
}

const maxSavedReadSnapshotBytes int64 = 128 << 20

// SQLite opens connections lazily. Its immutable reader must use a private
// image copied from a verified pinned descriptor, never reopen a checked path.
func (g *SavedReadGuard) snapshotWithOpen(ctx context.Context, open func(string) (*os.File, error)) (retErr error) {
	defer func() {
		if retErr != nil {
			_ = g.Close()
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	fd, err := open(g.canonical)
	if err != nil {
		return err
	}
	defer func() {
		if e := fd.Close(); retErr == nil {
			retErr = e
		}
	}()
	pinned, err := fd.Stat()
	if err != nil {
		return err
	}
	if !sameCacheFile(g.main, pinned) {
		return &CacheVisibilityError{Reason: "the opened cache descriptor does not match the verified database inode"}
	}
	if pinned.Size() > maxSavedReadSnapshotBytes {
		return &CacheVisibilityError{Reason: "the cache exceeds the 128 MiB saved-reader snapshot limit"}
	}
	dir, err := os.MkdirTemp("", "hgj-saved-reader-")
	if err != nil {
		return err
	}
	g.privateDir = dir
	canonicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	g.privateDir = canonicalDir
	destination := filepath.Join(canonicalDir, "snapshot")
	if err = validateCachePath(destination); err != nil {
		return &CacheVisibilityError{Reason: "temporary saved-reader paths cannot contain percent, question-mark or hash bytes; choose a TMPDIR with a plain filesystem path"}
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if out != nil {
			if e := out.Close(); retErr == nil {
				retErr = e
			}
		}
	}()
	buffer := make([]byte, 64<<10)
	var copied int64
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		n, readErr := fd.Read(buffer)
		if n > 0 {
			copied += int64(n)
			if copied > maxSavedReadSnapshotBytes {
				return &CacheVisibilityError{Reason: "the cache grew beyond the 128 MiB saved-reader snapshot limit"}
			}
			written, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err = out.Close(); err != nil {
		return err
	}
	// The successful explicit close replaces the deferred close.
	out = nil
	after, err := fd.Stat()
	if err != nil {
		return err
	}
	if copied != pinned.Size() || !sameCacheFile(pinned, after) {
		return &CacheVisibilityError{Reason: "the pinned database image changed while its reader snapshot was copied"}
	}
	if err = g.Check(); err != nil {
		return err
	}
	g.snapshot = destination
	return nil
}

func savedReadState(path string, stat func(string) (os.FileInfo, error)) (*SavedReadGuard, error) {
	if err := validateCachePath(path); err != nil {
		return nil, err
	}
	selectedPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	selected, err := cacheSelectedInfo(selectedPath)
	if err != nil {
		return nil, err
	}
	canonical, err := cacheCanonicalPath(selectedPath, selected)
	if err != nil {
		return nil, err
	}
	main, err := stat(canonical)
	if err != nil {
		return nil, err
	}
	if main != nil {
		if !main.Mode().IsRegular() {
			return nil, fmt.Errorf("saved cache must be a regular database file")
		}
		if count, known := cacheLinkCount(main); known && count > 1 {
			return nil, &CacheVisibilityError{Reason: "the database has multiple hard links and its WAL location is ambiguous; use a database with one hard link"}
		}
	}
	selectedTarget, err := stat(selectedPath)
	if err != nil {
		return nil, err
	}
	wal, err := stat(canonical + "-wal")
	if err != nil {
		return nil, err
	}
	mainAfter, err := stat(canonical)
	if err != nil {
		return nil, err
	}
	selectedTargetAfter, err := stat(selectedPath)
	if err != nil {
		return nil, err
	}
	selectedAfter, err := cacheSelectedInfo(selectedPath)
	if err != nil {
		return nil, err
	}
	canonicalAfter, err := cacheCanonicalPath(selectedPath, selectedAfter)
	if err != nil {
		return nil, err
	}
	if canonical != canonicalAfter || !sameCacheFile(selected, selectedAfter) || !sameCacheFile(main, mainAfter) || !sameCacheFile(main, selectedTarget) || !sameCacheFile(main, selectedTargetAfter) {
		return nil, &CacheVisibilityError{Reason: "the cache changed while its visibility was checked"}
	}
	if count, known := cacheLinkCount(mainAfter); known && count > 1 {
		return nil, &CacheVisibilityError{Reason: "the database gained a hard link while its visibility was checked"}
	}
	if wal != nil && wal.Size() > 0 {
		return nil, &CacheVisibilityError{Reason: "the saved cache has a non-empty WAL and may omit completed observations until its writers close"}
	}
	return &SavedReadGuard{path: selectedPath, canonical: canonical, selected: selected, main: main, wal: wal}, nil
}

func cacheSelectedInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return info, err
}

func cacheCanonicalPath(path string, selected os.FileInfo) (string, error) {
	if err := validateCachePath(path); err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(path)
	if os.IsNotExist(err) && selected == nil {
		parent, tail := filepath.Dir(path), []string{filepath.Base(path)}
		for {
			resolved, parentErr := filepath.EvalSymlinks(parent)
			if parentErr == nil {
				for i := len(tail) - 1; i >= 0; i-- {
					resolved = filepath.Join(resolved, tail[i])
				}
				if err := validateCachePath(resolved); err != nil {
					return "", err
				}
				return resolved, nil
			}
			if !os.IsNotExist(parentErr) || filepath.Dir(parent) == parent {
				break
			}
			if info, e := os.Lstat(parent); e == nil && info.Mode()&os.ModeSymlink != 0 {
				break
			}
			tail = append(tail, filepath.Base(parent))
			parent = filepath.Dir(parent)
		}
		return "", &CacheVisibilityError{Reason: "the selected cache directory cannot be resolved to a stable target"}
	}
	if err != nil {
		return "", &CacheVisibilityError{Reason: "the selected cache path cannot be resolved to a stable database target"}
	}
	if err := validateCachePath(canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

func validateCachePath(path string) error {
	if strings.ContainsAny(path, "%?#") {
		return &CacheVisibilityError{Reason: "cache paths cannot contain percent, question-mark or hash bytes; choose a plain filesystem name before SQLite file-URI opening"}
	}
	return nil
}

// SavedWriteGuard permits the writer's own WAL, but binds its file identity
// and sidecar location before source snapshots can be written.
type SavedWriteGuard struct {
	path, canonical string
	selected, main  os.FileInfo
}

func BeginSavedWrite(path string) (*SavedWriteGuard, error) {
	if err := validateCachePath(path); err != nil {
		return nil, err
	}
	selectedPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	selected, err := cacheSelectedInfo(selectedPath)
	if err != nil {
		return nil, err
	}
	canonical, err := cacheCanonicalPath(selectedPath, selected)
	if err != nil {
		return nil, err
	}
	main, err := cacheFileInfo(canonical)
	if err != nil {
		return nil, err
	}
	g := &SavedWriteGuard{path: selectedPath, canonical: canonical, selected: selected, main: main}
	if err = g.Check(); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *SavedWriteGuard) Path() string { return g.canonical }

func (g *SavedWriteGuard) Check() error {
	selected, err := cacheSelectedInfo(g.path)
	if err != nil {
		return err
	}
	canonical, err := cacheCanonicalPath(g.path, selected)
	if err != nil {
		return err
	}
	main, err := cacheFileInfo(canonical)
	if err != nil {
		return err
	}
	target, err := cacheFileInfo(g.path)
	if err != nil {
		return err
	}
	if canonical != g.canonical || (g.selected != nil && (selected == nil || !os.SameFile(g.selected, selected))) || (g.main != nil && (main == nil || !os.SameFile(g.main, main))) || !sameCacheFile(main, target) {
		return &CacheVisibilityError{Reason: "the selected cache path or database target changed during this write"}
	}
	if main != nil {
		if !main.Mode().IsRegular() {
			return fmt.Errorf("saved cache must be a regular database file")
		}
		if count, known := cacheLinkCount(main); known && count > 1 {
			return &CacheVisibilityError{Reason: "the database has multiple hard links and its WAL location is ambiguous; use a database with one hard link"}
		}
	}
	if g.main == nil {
		g.main = main
	}
	if g.selected == nil {
		g.selected = selected
	}
	return nil
}

// FileInfo.Sys is platform-specific. Unix exposes Nlink; hosts without that
// metadata still receive canonical-path and selected-target identity checks.
func cacheLinkCount(info os.FileInfo) (uint64, bool) {
	if info == nil {
		return 0, false
	}
	v := reflect.ValueOf(info.Sys())
	if !v.IsValid() {
		return 0, false
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 0, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0, false
	}
	n := v.FieldByName("Nlink")
	if n.IsValid() && n.CanUint() {
		return n.Uint(), true
	}
	return 0, false
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
	if g.canonical != after.canonical || !sameCacheFile(g.selected, after.selected) || !sameCacheFile(g.main, after.main) || !sameCacheFile(g.wal, after.wal) {
		return &CacheVisibilityError{Reason: "the saved cache changed during this read"}
	}
	if g.main != nil && !g.main.Mode().IsRegular() {
		return fmt.Errorf("saved cache must be a regular database file")
	}
	return nil
}
