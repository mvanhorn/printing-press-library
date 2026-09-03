// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/store"
)

// designsPageClient serves a paginated designs catalog. A short remainder
// after a full first page makes --max-pages 1 a truncated enumeration.
// When stickyAfter > 0, every page at or past that offset repeats the same
// next cursor so the sticky-cursor detector aborts.
type designsPageClient struct {
	total       int
	requests    int
	stickyAfter int
}

func (c *designsPageClient) RateLimit() float64 { return 0 }

func (c *designsPageClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	if path != "/search-service/select/design2" {
		return nil, fmt.Errorf("unexpected path %q", path)
	}
	c.requests++
	limit, _ := strconv.Atoi(params["limit"])
	if limit <= 0 {
		limit = 100
	}
	offset, _ := strconv.Atoi(params["after"])
	remaining := c.total - offset
	if remaining < 0 {
		remaining = 0
	}
	n := limit
	if remaining < limit {
		n = remaining
	}
	next := offset + n
	hasMore := next < c.total
	if c.stickyAfter > 0 && offset >= c.stickyAfter {
		next = c.stickyAfter
		hasMore = true
	}
	var b strings.Builder
	b.WriteString(`{"has_more":`)
	if hasMore {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString(`,"items":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		id := offset + i + 1
		fmt.Fprintf(&b, `{"id":%d,"title":"d%d"}`, id, id)
	}
	b.WriteByte(']')
	if hasMore {
		fmt.Fprintf(&b, `,"after":"%d"`, next)
	}
	b.WriteByte('}')
	return json.RawMessage(b.String()), nil
}

func TestDesignsSyncMaxPagesDoesNotAdvanceLastSyncedAt(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	seededAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := db.DB().Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count) VALUES (?, ?, ?, ?)`,
		"designs", "", seededAt.Format(time.RFC3339), 0,
	); err != nil {
		t.Fatalf("seed complete watermark: %v", err)
	}

	client := &designsPageClient{total: 105}
	res := syncResource(context.Background(), client, db, "designs", "", false, 1, false, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("capped sync: %v", res.Err)
	}
	if client.requests != 1 {
		t.Fatalf("capped sync requests = %d, want 1", client.requests)
	}

	cursor, afterCap, count, err := db.GetSyncState("designs")
	if err != nil {
		t.Fatalf("get after cap: %v", err)
	}
	if cursor == "" {
		t.Fatal("capped sync should preserve a resume cursor")
	}
	if count != 100 {
		t.Fatalf("capped stored count = %d, want 100", count)
	}
	if !afterCap.Equal(seededAt) {
		t.Fatalf("capped sync advanced last_synced_at from %v to %v", seededAt, afterCap)
	}
	if got := db.GetLastSyncedAt("designs"); got != seededAt.UTC().Format(time.RFC3339) {
		t.Fatalf("GetLastSyncedAt after cap = %q, want seeded watermark", got)
	}

	res = syncResource(context.Background(), client, db, "designs", "", false, 0, false, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("resume sync: %v", res.Err)
	}
	cursor, afterComplete, _, err := db.GetSyncState("designs")
	if err != nil {
		t.Fatalf("get after complete: %v", err)
	}
	if cursor != "" {
		t.Fatalf("complete sync cursor = %q, want empty", cursor)
	}
	stored, err := db.Count("designs")
	if err != nil {
		t.Fatalf("count designs: %v", err)
	}
	if stored != 105 {
		t.Fatalf("complete stored designs = %d, want 105", stored)
	}
	if !afterComplete.After(seededAt) {
		t.Fatalf("complete sync should stamp last_synced_at after %v, got %v", seededAt, afterComplete)
	}
}

func TestDesignsSyncMaxPagesFirstRunLeavesLastSyncedAtUnset(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	client := &designsPageClient{total: 105}
	res := syncResource(context.Background(), client, db, "designs", "", false, 1, false, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("capped sync: %v", res.Err)
	}
	cursor, watermark, _, err := db.GetSyncState("designs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if cursor == "" {
		t.Fatal("capped first sync should preserve a resume cursor")
	}
	if !watermark.IsZero() {
		t.Fatalf("first incomplete sync stamped last_synced_at = %v", watermark)
	}
	if got := db.GetLastSyncedAt("designs"); got != "" {
		t.Fatalf("GetLastSyncedAt after incomplete first sync = %q, want empty", got)
	}
}

func TestDesignsSyncStickyCursorDoesNotAdvanceLastSyncedAt(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	seededAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := db.DB().Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count) VALUES (?, ?, ?, ?)`,
		"designs", "", seededAt.Format(time.RFC3339), 0,
	); err != nil {
		t.Fatalf("seed complete watermark: %v", err)
	}

	client := &designsPageClient{total: 300, stickyAfter: 100}
	res := syncResource(context.Background(), client, db, "designs", "", false, 0, false, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("sticky sync: %v", res.Err)
	}
	if client.requests != 2 {
		t.Fatalf("sticky sync requests = %d, want 2", client.requests)
	}

	cursor, afterSticky, count, err := db.GetSyncState("designs")
	if err != nil {
		t.Fatalf("get after sticky: %v", err)
	}
	if cursor != "100" {
		t.Fatalf("sticky sync cursor = %q, want preserved resume cursor 100", cursor)
	}
	if count != 200 {
		t.Fatalf("sticky stored count = %d, want 200", count)
	}
	if !afterSticky.Equal(seededAt) {
		t.Fatalf("sticky abort advanced last_synced_at from %v to %v", seededAt, afterSticky)
	}
	if got := db.GetLastSyncedAt("designs"); got != seededAt.UTC().Format(time.RFC3339) {
		t.Fatalf("GetLastSyncedAt after sticky abort = %q, want seeded watermark", got)
	}

	client.stickyAfter = 0
	res = syncResource(context.Background(), client, db, "designs", "", false, 0, false, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("unstick resume: %v", res.Err)
	}
	cursor, afterComplete, _, err := db.GetSyncState("designs")
	if err != nil {
		t.Fatalf("get after complete: %v", err)
	}
	if cursor != "" {
		t.Fatalf("complete sync cursor = %q, want empty", cursor)
	}
	stored, err := db.Count("designs")
	if err != nil {
		t.Fatalf("count designs: %v", err)
	}
	if stored != 300 {
		t.Fatalf("complete stored designs = %d, want 300", stored)
	}
	if !afterComplete.After(seededAt) {
		t.Fatalf("complete sync should stamp last_synced_at after %v, got %v", seededAt, afterComplete)
	}
}
