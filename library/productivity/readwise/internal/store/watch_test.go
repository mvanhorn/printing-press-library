// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSeenSetDiff(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)

	watchID, err := db.AddWatch(ctx, "inbox", "docs", `{}`)
	if err != nil {
		t.Fatalf("AddWatch: %v", err)
	}

	// Initially empty
	seen, err := db.SeenItemIDs(ctx, watchID, "docs")
	if err != nil {
		t.Fatalf("SeenItemIDs initial: %v", err)
	}
	if len(seen) != 0 {
		t.Errorf("expected empty seen set, got %d", len(seen))
	}

	// Mark 2 ids as seen
	if err := db.MarkSeen(ctx, watchID, "docs", []string{"id1", "id2"}); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}
	seen, err = db.SeenItemIDs(ctx, watchID, "docs")
	if err != nil {
		t.Fatalf("SeenItemIDs after mark: %v", err)
	}
	if !seen["id1"] || !seen["id2"] {
		t.Errorf("expected id1 and id2 in seen, got %v", seen)
	}

	// Add id3, only id3 is new
	if err := db.MarkSeen(ctx, watchID, "docs", []string{"id3"}); err != nil {
		t.Fatalf("MarkSeen id3: %v", err)
	}
	seen, err = db.SeenItemIDs(ctx, watchID, "docs")
	if err != nil {
		t.Fatalf("SeenItemIDs after id3: %v", err)
	}
	if !seen["id3"] {
		t.Errorf("expected id3 in seen after second mark")
	}
	if len(seen) != 3 {
		t.Errorf("expected 3 seen ids, got %d", len(seen))
	}
}

func TestListDocItemsInvalidLocation(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	_, err := db.ListDocItems(ctx, []string{"notvalid"}, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid location token, got nil")
	}
}

func TestListDocItemsTagFilter(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	// No data — just test that the query executes without error.
	// Tag filter uses json_each on data JSON; multiple tags = OR semantics.
	items, err := db.ListDocItems(ctx, nil, nil, []string{"tag1", "tag2"})
	if err != nil {
		t.Fatalf("ListDocItems with tag filter: %v", err)
	}
	// Empty store, no items expected.
	if len(items) != 0 {
		t.Errorf("expected 0 items from empty store, got %d", len(items))
	}
}

func TestRemoveWatchCascades(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)

	watchID, err := db.AddWatch(ctx, "cascade-test", "docs", `{}`)
	if err != nil {
		t.Fatalf("AddWatch: %v", err)
	}
	if err := db.MarkSeen(ctx, watchID, "docs", []string{"a", "b"}); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}

	// Verify seen rows exist
	seen, err := db.SeenItemIDs(ctx, watchID, "docs")
	if err != nil {
		t.Fatalf("SeenItemIDs before remove: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("expected 2 seen rows before remove, got %d", len(seen))
	}

	// Remove the watch
	n, err := db.RemoveWatch(ctx, watchID)
	if err != nil {
		t.Fatalf("RemoveWatch: %v", err)
	}
	if n != 1 {
		t.Fatalf("RemoveWatch: expected 1 row affected, got %d", n)
	}

	// Seen rows should be gone (ON DELETE CASCADE)
	seen, err = db.SeenItemIDs(ctx, watchID, "docs")
	if err != nil {
		t.Fatalf("SeenItemIDs after remove: %v", err)
	}
	if len(seen) != 0 {
		t.Errorf("expected watch_seen rows to be cascaded, got %d rows", len(seen))
	}
}

func TestListHighlightItems(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	// Empty store — validate the CAST join compiles and executes without error.
	items, err := db.ListHighlightItems(ctx, "stoic", false)
	if err != nil {
		t.Fatalf("ListHighlightItems: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items from empty store, got %d", len(items))
	}
}

// TestE3HighlightUnseenBeforeLimit ensures a low-ranked new match survives a saturated seen set.
func TestE3HighlightUnseenBeforeLimit(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	id, err := db.AddWatch(ctx, "rank", "highlights", `{"query":"needle"}`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := 0; i < 201; i++ {
		key := fmt.Sprintf("%04d", i)
		text := "needle needle needle"
		if i == 200 {
			text = "needle " + strings.Repeat("other ", 1000)
		}
		if _, err := db.db.ExecContext(ctx, "INSERT INTO highlights(id,text,data) VALUES(?,?,'{}')", key, text); err != nil {
			t.Fatal(err)
		}
		if i < 200 {
			ids = append(ids, key)
		}
	}
	if err := db.MarkSeen(ctx, id, "highlights", ids); err != nil {
		t.Fatal(err)
	}
	items, err := db.ListHighlightItems(ctx, "needle", false, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "0200" {
		t.Fatalf("unseen low ranked match missing: %v", items)
	}
}
