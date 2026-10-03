// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hostelworld

import (
	"context"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CacheVisibilityError prevents successful output from an older checkpoint.
type CacheVisibilityError struct{ Reason string }

func (e *CacheVisibilityError) Error() string {
	return "cache_visibility_unavailable: " + e.Reason + ". Close other cache writers and retry; live planning without --save remains available."
}

type CacheGuard struct {
	selected, path string
	main           os.FileInfo
	header         [32]byte
	privateDir     string
}

func (g *CacheGuard) Path() string  { return g.path }
func (g *CacheGuard) Missing() bool { return g.main == nil }

// Resolve the selected file's physical pathname, including existing ancestors
// of a not-yet-created cache. Broken symlinks are ambiguous, not empty caches.
func canonicalCachePath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if info, e := os.Lstat(path); e == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", &CacheVisibilityError{Reason: "the selected cache has a broken symlink"}
	} else if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = canonicalCachePath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func cacheState(selected string, readHeader, allowSidecars bool) (*CacheGuard, error) {
	absolute, err := filepath.Abs(selected)
	if err != nil {
		return nil, err
	}
	path, err := canonicalCachePath(absolute)
	if err != nil {
		return nil, err
	}
	// The generated store concatenates its URI parameters. Refuse delimiters
	// rather than accidentally open a different file or drop mode=ro.
	if strings.ContainsAny(path, "%?#") {
		return nil, &CacheVisibilityError{Reason: "the selected cache path cannot be represented unambiguously by the SQLite reader"}
	}
	g := &CacheGuard{selected: absolute, path: path}
	if !allowSidecars {
		seen := map[string]bool{}
		for _, base := range []string{path, absolute} {
			for _, suffix := range []string{"-wal", "-journal"} {
				name := base + suffix
				if seen[name] {
					continue
				}
				seen[name] = true
				if _, err := os.Lstat(name); err == nil {
					return nil, &CacheVisibilityError{Reason: "the cache has a WAL or rollback journal and its completed observations cannot be read safely"}
				} else if !os.IsNotExist(err) {
					return nil, err
				}
			}
		}
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return g, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &CacheVisibilityError{Reason: "the selected cache is not a regular database file"}
	}
	links, err := cacheLinkCount(path, info)
	if err != nil {
		return nil, err
	}
	if links != 1 {
		return nil, &CacheVisibilityError{Reason: "the cache has multiple hard links, so another pathname could hide its journal"}
	}
	selectedInfo, err := os.Stat(absolute)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, selectedInfo) {
		return nil, &CacheVisibilityError{Reason: "the selected cache pathname changed identity"}
	}
	g.main = info
	if readHeader {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		opened, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, err
		}
		if !sameCacheState(info, opened) {
			f.Close()
			return nil, &CacheVisibilityError{Reason: "the cache changed while opening its identity guard"}
		}
		header := make([]byte, 100)
		_, readErr := io.ReadFull(f, header)
		closeErr := f.Close()
		if readErr != nil {
			return nil, &CacheVisibilityError{Reason: "the cache has no complete SQLite header"}
		}
		if closeErr != nil {
			return nil, closeErr
		}
		g.header = sha256.Sum256(header)
		after, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !sameCacheState(info, after) {
			return nil, &CacheVisibilityError{Reason: "the cache changed while its header was checked"}
		}
	}
	return g, nil
}

func sameCacheState(a, b os.FileInfo) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func BeginCacheRead(path string) (*CacheGuard, error)  { return cacheState(path, true, false) }
func BeginCacheWrite(path string) (*CacheGuard, error) { return cacheState(path, false, true) }

// Check runs after every selected row and status query, before any success is
// emitted. Immutable readers must not silently omit a concurrent WAL commit.
func (g *CacheGuard) Check() error {
	after, err := cacheState(g.selected, true, false)
	if err != nil {
		return err
	}
	if g.path != after.path || !sameCacheState(g.main, after.main) || g.header != after.header {
		return &CacheVisibilityError{Reason: "the selected cache changed during this read"}
	}
	return nil
}

// BindWriter adopts a new file after creation or verifies an existing file.
// The writer owns its WAL, so only identity and unique-link checks apply here.
func (g *CacheGuard) BindWriter() error {
	after, err := cacheState(g.selected, false, true)
	if err != nil {
		return err
	}
	if after.main == nil || g.path != after.path || (g.main != nil && !os.SameFile(g.main, after.main)) {
		return &CacheVisibilityError{Reason: "the selected cache changed while opening its writer"}
	}
	g.main = after.main
	return nil
}
func (g *CacheGuard) CheckWriter() error {
	after, err := cacheState(g.selected, false, true)
	if err != nil {
		return err
	}
	if g.main == nil || after.main == nil || g.path != after.path || !os.SameFile(g.main, after.main) {
		return &CacheVisibilityError{Reason: "the selected cache changed during its write"}
	}
	return nil
}
func (g *CacheGuard) CheckAfterWriterClose() error {
	return g.CheckWriter()
}

// Snapshot pins a verified descriptor before copying. The SQLite connection
// opens only this private immutable copy, so lazy driver opens cannot select a
// substituted inode even when the caller's original pathname is restored.
const MaxCacheSnapshotBytes int64 = 512 * 1024 * 1024

func (g *CacheGuard) Snapshot(ctx context.Context) (string, error) {
	if g.main == nil {
		return "", &CacheVisibilityError{Reason: "the selected cache is missing"}
	}
	if g.main.Size() > MaxCacheSnapshotBytes {
		return "", &CacheVisibilityError{Reason: "the cache exceeds the 512 MiB bounded read snapshot limit"}
	}
	if err := g.Check(); err != nil {
		return "", err
	}
	f, err := os.Open(g.path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !sameCacheState(g.main, opened) {
		return "", &CacheVisibilityError{Reason: "the selected cache changed before its descriptor was pinned"}
	}
	dir, err := os.MkdirTemp("", "hostelworld-cache-read-")
	if err != nil {
		return "", err
	}
	g.privateDir = dir
	path := filepath.Join(dir, "snapshot")
	out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		g.Close()
		return "", err
	}
	buf := make([]byte, 128*1024)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			out.Close()
			g.Close()
			return "", err
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > MaxCacheSnapshotBytes {
				out.Close()
				g.Close()
				return "", &CacheVisibilityError{Reason: "the cache exceeded the bounded read snapshot limit"}
			}
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				g.Close()
				return "", err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			g.Close()
			return "", readErr
		}
	}
	if err := out.Close(); err != nil {
		g.Close()
		return "", err
	}
	after, err := f.Stat()
	if err != nil {
		g.Close()
		return "", err
	}
	if !sameCacheState(g.main, after) || total != g.main.Size() {
		g.Close()
		return "", &CacheVisibilityError{Reason: "the pinned cache changed while its snapshot was copied"}
	}
	if err := g.Check(); err != nil {
		g.Close()
		return "", err
	}
	return path, nil
}

// Close removes the private snapshot on every success or failure path. It must
// be deferred before the Store close, so SQLite releases its handle first.
func (g *CacheGuard) Close() error {
	if g == nil || g.privateDir == "" {
		return nil
	}
	dir := g.privateDir
	g.privateDir = ""
	return os.RemoveAll(dir)
}
