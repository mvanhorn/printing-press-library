// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
)

// A capped archive sync must leave a resumable bookmark. Without it every
// re-run re-downloads the newest pages and older history is never reached,
// which silently defeats the whole point of the local archive.
func TestResumeCursorRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "resume.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	if got := loadResumeCursor(ctx, db, "C0EXAMPLE01"); got != "" {
		t.Fatalf("cold store should have no bookmark, got %q", got)
	}

	saveResumeCursor(db, "C0EXAMPLE01", "dXNlcjpVMDYxTkZUVDI=")
	if got := loadResumeCursor(ctx, db, "C0EXAMPLE01"); got != "dXNlcjpVMDYxTkZUVDI=" {
		t.Fatalf("bookmark not persisted; got %q", got)
	}

	// Bookmarks are per-channel: one channel's progress must never be read
	// as another's, or a resumed run would skip pages it never fetched.
	if got := loadResumeCursor(ctx, db, "C0EXAMPLE02"); got != "" {
		t.Fatalf("bookmark leaked across channels; got %q", got)
	}

	saveResumeCursor(db, "C0EXAMPLE01", "second")
	if got := loadResumeCursor(ctx, db, "C0EXAMPLE01"); got != "second" {
		t.Fatalf("bookmark not overwritten; got %q", got)
	}

	// Exhausting history clears the bookmark so the next run starts fresh
	// at the newest page rather than resuming from a stale cursor.
	saveResumeCursor(db, "C0EXAMPLE01", "")
	if got := loadResumeCursor(ctx, db, "C0EXAMPLE01"); got != "" {
		t.Fatalf("exhausted history should clear the bookmark; got %q", got)
	}
}
