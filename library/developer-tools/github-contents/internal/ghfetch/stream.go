// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// ErrSizeMismatch is wrapped by StreamToFile when the byte count written
// differs from the caller's expected size. Callers distinguish it with
// errors.Is — the downloader uses it to trigger the LFS blob-API fallback
// for small expected sizes.
var ErrSizeMismatch = errors.New("size mismatch")

// processUmask is the process umask, captured once at package init —
// before any goroutines run — because reading it requires the
// non-thread-safe set-and-restore dance (syscall.Umask has no pure
// getter). Used to publish downloaded files at the mode a plain
// os.Create would have produced (0666 &^ umask) instead of
// os.CreateTemp's private 0600, WITHOUT widening beyond the user's
// umask policy: a umask-077 user keeps 0600 for private-repo content.
// syscall.Umask exists on the darwin/linux targets this module ships
// for.
var processUmask = func() int {
	u := syscall.Umask(0)
	syscall.Umask(u)
	return u
}()

// StreamToFile copies r to destPath atomically: it streams into a
// randomly-named temp file next to the destination (os.CreateTemp, so a
// repo that itself contains an entry named "<dest>.partial" — or two
// concurrent workers writing siblings — can never collide with or truncate
// the in-flight temp), fsyncs, closes, verifies the byte count when
// expectedSize >= 0, then renames into place. Parent directories are
// created as needed. On any failure the temp file is removed and destPath
// is left untouched. Returns the number of bytes written.
//
// expectedSize < 0 skips the size check (for callers that don't know the
// size up front, e.g. tarball/asset streams). A mismatch returns an error
// wrapping ErrSizeMismatch.
func StreamToFile(r io.Reader, destPath string, expectedSize int64) (int64, error) {
	dir := filepath.Dir(destPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- user-requested download output dir; conventional mkdir mode, narrowed by the process umask
			return 0, fmt.Errorf("creating output directory: %w", err)
		}
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(destPath)+".partial-*")
	if err != nil {
		return 0, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	// os.CreateTemp creates 0600; publish at the mode a plain os.Create
	// would have produced — 0666 masked by the process umask — so
	// downloaded files get conventional permissions without overriding a
	// restrictive umask (see processUmask).
	if err := tmp.Chmod(0o666 &^ os.FileMode(processUmask)); err != nil { // #nosec G302 G115 -- umask-derived (0..0o777, always fits FileMode), matches os.Create semantics
		cleanup()
		return 0, fmt.Errorf("setting file mode: %w", err)
	}

	written, copyErr := io.Copy(tmp, r)
	if copyErr != nil {
		cleanup()
		return 0, fmt.Errorf("writing %s: %w", destPath, copyErr)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return 0, fmt.Errorf("syncing %s: %w", destPath, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("closing %s: %w", destPath, err)
	}
	if expectedSize >= 0 && written != expectedSize {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("%s: %w: expected %d bytes, got %d", destPath, ErrSizeMismatch, expectedSize, written)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("renaming %s into place: %w", destPath, err)
	}
	return written, nil
}
