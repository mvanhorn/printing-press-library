// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const wheelogSnapshotLimit = 64 << 20

type wheelogSnapshotReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r wheelogSnapshotReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// Pin the verified source descriptor before copying. SQL opens only our private
// snapshot, so swapping/restoring the selected pathname cannot substitute rows
// between validation and the driver's lazy first query.
func pinWheelogSnapshot(ctx context.Context, target string, guard *wheelogReadGuard) (string, func(), error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	source, err := os.Open(target) // #nosec G304 -- caller-selected read-only cache; descriptor identity is verified below.
	if err != nil {
		return "", nil, err
	}
	before, err := source.Stat()
	if err != nil || !sameWheelogFile(guard.main, before) {
		_ = source.Close() // Preserve the descriptor verification failure.
		if err != nil {
			return "", nil, err
		}
		return "", nil, &WheelogSnapshotError{Reason: "the opened saved database is not the verified inode"}
	}
	if before.Size() > wheelogSnapshotLimit {
		_ = source.Close()
		return "", nil, &WheelogSnapshotError{Reason: "the saved database exceeds the 64 MiB private snapshot limit"}
	}
	dir, err := os.MkdirTemp("", "wheelog-saved-snapshot-")
	if err != nil {
		_ = source.Close()
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) } // Owned private temporary snapshot only.
	path := filepath.Join(dir, "snapshot")
	snapshot, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600) // #nosec G304 -- fixed filename under a newly-created private directory.
	if err != nil {
		_ = source.Close()
		cleanup()
		return "", nil, err
	}
	size, copyErr := io.Copy(snapshot, wheelogSnapshotReader{ctx: ctx, reader: io.LimitReader(source, wheelogSnapshotLimit+1)})
	closeSnapshotErr := snapshot.Close()
	after, statErr := source.Stat()
	closeSourceErr := source.Close()
	if copyErr == nil && size > wheelogSnapshotLimit {
		copyErr = fmt.Errorf("saved database exceeds the 64 MiB private snapshot limit")
	}
	if copyErr == nil && statErr != nil {
		copyErr = statErr
	}
	if copyErr == nil && !sameWheelogFile(before, after) {
		copyErr = &WheelogSnapshotError{Reason: "the pinned saved database changed while it was copied"}
	}
	if copyErr == nil && closeSnapshotErr != nil {
		copyErr = closeSnapshotErr
	}
	if copyErr == nil && closeSourceErr != nil {
		copyErr = closeSourceErr
	}
	if copyErr == nil {
		copyErr = guard.check(wheelogFileInfo)
	}
	if copyErr != nil {
		cleanup()
		return "", nil, copyErr
	}
	return path, cleanup, nil
}
