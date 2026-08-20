// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import (
	"fmt"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// MatchGlobs reports whether relPath (a slash-separated path relative to
// the fetch root) should be included given includes and excludes glob
// lists. An empty includes list means "include everything"; excludes is
// applied after includes and always wins. Each pattern is matched against
// both the full relPath AND its basename, so "*.pdf" matches
// "books/foo.pdf" via the basename check even though it would not match
// the full path directly.
//
// A pattern that fails to compile (path.ErrBadPattern) is treated as a
// non-match rather than propagating an error — callers are expected to
// validate patterns once up front (see InvalidGlobs) and warn the user a
// single time, not re-surface the same parse error for every file checked
// against it.
func MatchGlobs(relPath string, includes, excludes []string) bool {
	if !matchesAny(relPath, includes, true) {
		return false
	}
	if matchesAny(relPath, excludes, false) {
		return false
	}
	return true
}

// matchesAny reports whether relPath matches any pattern in patterns.
// emptyMeans is the result returned when patterns is empty (true for an
// include list — "no include filter" means "match everything" — false for
// an exclude list — "no exclude filter" means "exclude nothing").
func matchesAny(relPath string, patterns []string, emptyMeans bool) bool {
	if len(patterns) == 0 {
		return emptyMeans
	}
	base := path.Base(relPath)
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if ok, err := path.Match(p, relPath); err == nil && ok {
			return true
		}
		if ok, err := path.Match(p, base); err == nil && ok {
			return true
		}
	}
	return false
}

// InvalidGlobs returns the subset of patterns that fail to compile as a
// path.Match pattern. Callers use this once, up front, to print a single
// warning naming the bad pattern(s) rather than letting MatchGlobs' silent
// no-match happen per file with no visible cause.
func InvalidGlobs(patterns []string) []string {
	var bad []string
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if _, err := path.Match(p, ""); err != nil {
			bad = append(bad, p)
		}
	}
	return bad
}

// SafeRelPath validates that p is safe to join onto a local destination
// directory: not absolute, not able to escape the destination root via
// ".." traversal, and free of host-specific volume/drive syntax. On
// Windows any ':' in a segment is rejected (drive-relative paths like
// "C:evil" and NTFS alternate-data-stream syntax like "foo:bar"); on
// Unix hosts ':' is an ordinary filename character (timestamped names
// like "2026-07-10T12:30:00.log" are legitimate) and passes. The
// VolumeName check stays unconditional — it only ever fires under
// Windows path parsing, where a volume-qualified name must never be
// joined. Returns the cleaned slash-separated path on success.
func SafeRelPath(p string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if trimmed == "" {
		return "", fmt.Errorf("empty path")
	}
	cleaned := path.Clean(trimmed)
	if path.IsAbs(cleaned) {
		return "", fmt.Errorf("unsafe path %q: absolute paths are not allowed", p)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("unsafe path %q: directory traversal is not allowed", p)
	}
	if cleaned == "." {
		return "", fmt.Errorf("unsafe path %q: resolves to the destination root itself", p)
	}
	if runtime.GOOS == "windows" && strings.Contains(cleaned, ":") {
		return "", fmt.Errorf("unsafe path %q: ':' is not allowed in path segments on Windows (drive-relative / NTFS stream syntax)", p)
	}
	if filepath.VolumeName(filepath.FromSlash(cleaned)) != "" {
		return "", fmt.Errorf("unsafe path %q: volume-qualified paths are not allowed", p)
	}
	return cleaned, nil
}
