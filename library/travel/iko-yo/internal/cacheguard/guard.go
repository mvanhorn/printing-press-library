// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// Package cacheguard bounds immutable SQLite snapshots and canonical writes.
package cacheguard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Guard pins the selected path and physical database across one operation.
// Identity checks avoid raw database descriptors. Clone opens a pinned read
// descriptor only after refusing active sidecars, and rechecks the source.
type Guard struct {
	selected, canonical string
	anchor              string
	anchorInfo          os.FileInfo
	info                os.FileInfo
	snapshotDir         string
}

// Read refuses sidecars that the generated immutable reader cannot observe.
func Read(path string) (*Guard, error) {
	g, err := Write(path)
	if err == nil {
		err = g.sidecars()
	}
	return g, err
}

// Write resolves aliases before the SQLite writer opens and pins file identity.
// A singly linked symlink target is supported; hard links are ambiguous because
// SQLite sidecars can be attached to another pathname for the same database.
func Write(path string) (*Guard, error) {
	selected, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cache path: %w", err)
	}
	if strings.ContainsAny(selected, "?#%\x00") {
		return nil, fmt.Errorf("cache path contains unsupported SQLite URI characters")
	}
	canonical, anchor, info, err := resolve(selected)
	if err != nil {
		return nil, err
	}
	if strings.ContainsAny(canonical, "?#%\x00") {
		return nil, fmt.Errorf("resolved cache path contains unsupported SQLite URI characters")
	}
	anchorInfo, err := os.Stat(anchor)
	if err != nil || !anchorInfo.IsDir() {
		return nil, fmt.Errorf("cache parent cannot be pinned")
	}
	return &Guard{selected: selected, canonical: canonical, anchor: anchor, anchorInfo: anchorInfo, info: info}, nil
}

func (g *Guard) Path() string { return g.canonical }
func (g *Guard) Exists() bool { return g.info != nil }

// Clone binds lazy SQLite opens to bytes copied through the verified inode,
// rather than reopening the selected pathname during the first SQL query.
// The copy is private and bounded; source sidecars/identity are checked around
// the copy and again after the complete read operation.
func (g *Guard) Clone(ctx context.Context) (path string, err error) {
	if g.info == nil || g.snapshotDir != "" {
		return "", fmt.Errorf("cache snapshot requires one existing database")
	}
	if err = g.Snapshot(); err != nil {
		return "", err
	}
	source, err := os.Open(g.canonical)
	if err != nil {
		return "", fmt.Errorf("opening pinned cache snapshot: %w", err)
	}
	defer func() {
		err = errors.Join(err, source.Close())
		if err != nil {
			err = errors.Join(err, g.Cleanup())
		}
	}()
	opened, err := source.Stat()
	if err != nil || !os.SameFile(opened, g.info) || !opened.Mode().IsRegular() {
		return "", fmt.Errorf("cache descriptor does not match the selected database")
	}
	links, err := linkCount(g.canonical, opened)
	if err != nil || links != 1 {
		return "", fmt.Errorf("pinned cache database has ambiguous hard links")
	}
	const maxSnapshotBytes = 128 << 20
	if opened.Size() < 0 || opened.Size() > maxSnapshotBytes {
		return "", fmt.Errorf("cache snapshot exceeds the 128 MiB read bound")
	}
	g.snapshotDir, err = os.MkdirTemp("", "iko-yo-cache-snapshot-")
	if err != nil {
		return "", fmt.Errorf("creating private cache snapshot: %w", err)
	}
	resolvedTemp, resolveErr := filepath.EvalSymlinks(g.snapshotDir)
	if resolveErr != nil {
		return "", fmt.Errorf("resolving private cache snapshot: %w", resolveErr)
	}
	g.snapshotDir = resolvedTemp
	if strings.ContainsAny(g.snapshotDir, "?#%\x00") {
		return "", fmt.Errorf("private cache snapshot path contains unsupported SQLite URI characters")
	}
	path = filepath.Join(g.snapshotDir, "snapshot")
	dest, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- fixed filename inside this operation's private random 0700 directory.
	if err != nil {
		return "", err
	}
	_, copyErr := io.CopyN(dest, contextReader{ctx: ctx, reader: source}, opened.Size())
	closeErr := dest.Close()
	if err = errors.Join(copyErr, closeErr); err != nil {
		return "", fmt.Errorf("copying pinned cache snapshot: %w", err)
	}
	after, statErr := source.Stat()
	if statErr != nil || !os.SameFile(opened, after) || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return "", fmt.Errorf("pinned cache database changed while copying")
	}
	if err = g.Snapshot(); err != nil {
		return "", err
	}
	return path, nil
}

func (g *Guard) Cleanup() error {
	if g.snapshotDir == "" {
		return nil
	}
	path := g.snapshotDir
	g.snapshotDir = ""
	return os.RemoveAll(path)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// BindCreated pins a newly created DB after open/migration, before fact writes.
func (g *Guard) BindCreated() error {
	current, err := g.identity()
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("cache database did not exist after writer open")
	}
	if g.info == nil {
		g.info = current
	}
	return nil
}

// Identity validates the chosen path around save and after the writer closes.
func (g *Guard) Identity() error {
	_, err := g.identity()
	return err
}

// Snapshot runs after all queries/close, before any successful facts are emitted.
func (g *Guard) Snapshot() error {
	current, err := g.identity()
	if err != nil {
		return err
	}
	if g.info == nil && current != nil || g.info != nil && current == nil {
		return fmt.Errorf("cache database appeared or disappeared during read; retry")
	}
	if current != nil && (current.Size() != g.info.Size() || !current.ModTime().Equal(g.info.ModTime()) || current.Mode() != g.info.Mode()) {
		return fmt.Errorf("cache database changed during read; retry after writers close")
	}
	return g.sidecars()
}

func (g *Guard) identity() (os.FileInfo, error) {
	parent, err := os.Stat(g.anchor)
	if err != nil || !parent.IsDir() || !os.SameFile(parent, g.anchorInfo) {
		return nil, fmt.Errorf("cache parent identity changed during operation")
	}
	canonical, _, info, err := resolve(g.selected)
	if err != nil {
		return nil, err
	}
	if canonical != g.canonical {
		return nil, fmt.Errorf("selected cache path changed its resolved target")
	}
	if g.info != nil && (info == nil || !os.SameFile(info, g.info)) {
		return nil, fmt.Errorf("cache database identity changed during operation")
	}
	return info, nil
}

func (g *Guard) sidecars() error {
	for _, base := range []string{g.canonical, g.selected} {
		for _, suffix := range []string{"-wal", "-shm", "-journal"} {
			info, err := os.Lstat(base + suffix)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return fmt.Errorf("cache SQLite sidecar cannot be checked: %w", err)
			}
			if !info.Mode().IsRegular() || info.Size() != 0 {
				return fmt.Errorf("cache has an active or ambiguous SQLite %s sidecar; close writers and retry", strings.TrimPrefix(suffix, "-"))
			}
		}
	}
	return nil
}

func resolve(selected string) (canonical, anchor string, info os.FileInfo, err error) {
	canonical, err = filepath.EvalSymlinks(selected)
	if err == nil {
		info, err = os.Stat(canonical)
		if err != nil {
			return "", "", nil, fmt.Errorf("cache database cannot be inspected: %w", err)
		}
		chosen, statErr := os.Stat(selected)
		if statErr != nil || !os.SameFile(info, chosen) || !info.Mode().IsRegular() {
			return "", "", nil, fmt.Errorf("selected cache path does not identify the regular resolved database")
		}
		links, countErr := linkCount(canonical, info)
		if countErr != nil || links != 1 {
			return "", "", nil, fmt.Errorf("cache database must have exactly one hard link; aliases are ambiguous")
		}
		return canonical, filepath.Dir(canonical), info, nil
	}
	if !os.IsNotExist(err) {
		return "", "", nil, fmt.Errorf("cache path cannot be resolved: %w", err)
	}
	// Missing stores stay empty on reads and may be created on writes. Resolve
	// the existing parent first; dangling symlinks are never treated as absence.
	if _, lerr := os.Lstat(selected); lerr == nil || !os.IsNotExist(lerr) {
		return "", "", nil, fmt.Errorf("cache path is unresolved rather than absent")
	}
	parts := []string{filepath.Base(selected)}
	parent := filepath.Dir(selected)
	for {
		resolved, resolveErr := filepath.EvalSymlinks(parent)
		if resolveErr == nil {
			anchor := resolved
			parentInfo, statErr := os.Stat(resolved)
			if statErr != nil || !parentInfo.IsDir() {
				return "", "", nil, fmt.Errorf("cache parent is not an accessible directory")
			}
			for i := len(parts) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, parts[i])
			}
			return resolved, anchor, nil, nil
		}
		if !os.IsNotExist(resolveErr) {
			return "", "", nil, fmt.Errorf("cache parent cannot be resolved: %w", resolveErr)
		}
		if _, lerr := os.Lstat(parent); lerr == nil || !os.IsNotExist(lerr) {
			return "", "", nil, fmt.Errorf("cache parent contains an unresolved alias")
		}
		parts = append(parts, filepath.Base(parent))
		next := filepath.Dir(parent)
		if next == parent {
			return "", "", nil, fmt.Errorf("cache parent cannot be resolved")
		}
		parent = next
	}
}
