// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

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

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
)

// offsetPageClient is a fake sync client that emulates Splitwise's
// /get_expenses endpoint: it paginates by ?offset=N&limit=M and returns a
// bare {"expenses":[...]} envelope with no has_more / next-cursor signal.
// It serves `total` synthetic expenses across as many pages as the offset
// walk requires.
type offsetPageClient struct {
	total          int
	requestedPages int
}

func (c *offsetPageClient) RateLimit() float64 { return 0 }

func (c *offsetPageClient) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	c.requestedPages++

	limit, _ := strconv.Atoi(params["limit"])
	if limit <= 0 {
		limit = 100
	}
	offset, _ := strconv.Atoi(params["offset"])

	remaining := c.total - offset
	if remaining < 0 {
		remaining = 0
	}
	n := limit
	if remaining < limit {
		n = remaining
	}

	var sb strings.Builder
	sb.WriteString(`{"expenses":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		// Each expense needs an extractable integer primary key so the
		// upsert path lands a row.
		fmt.Fprintf(&sb, `{"id":%d,"description":"e%d","deleted_at":null}`, offset+i+1, offset+i+1)
	}
	sb.WriteString(`]}`)
	return json.RawMessage(sb.String()), nil
}

// TestSyncGetExpenses_PaginatesPastFirstPage pins the offset-paginator fix:
// /get_expenses returns no has_more signal, so before the fix the sync loop
// broke after the first 100 rows even under --full --max-pages 0. The fake
// account here has 143 expenses; all 143 must reach the local store.
func TestSyncGetExpenses_PaginatesPastFirstPage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &offsetPageClient{total: 143}

	userParams, err := parseSyncUserParams(nil, nil, nil)
	if err != nil {
		t.Fatalf("parseSyncUserParams: %v", err)
	}

	// full=true clears any resume cursor; maxPages=0 means unlimited.
	res := syncResource(
		context.Background(),
		client,
		db,
		"get-expenses",
		"",    // sinceTS
		true,  // full
		0,     // maxPages (0 = unlimited)
		false, // latestOnly
		userParams,
		io.Discard,
	)
	if res.Err != nil {
		t.Fatalf("syncResource returned error: %v", res.Err)
	}

	if res.Count != 143 {
		t.Fatalf("synced count = %d, want 143 (offset pagination must walk past the 100-row first page)", res.Count)
	}

	// The fake account spans exactly two pages (100 + 43). After the second
	// request, len(items)=43 < pageSize.limit triggers the natural-end check
	// and the loop exits without issuing a third request. Pinning to == 2
	// catches both "stopped too early" (page 1 only) and "never stops"
	// (sticky-offset infinite loop) regressions.
	if client.requestedPages != 2 {
		t.Fatalf("requested pages = %d, want 2 (loop must advance past page 1 and stop after the short second page)", client.requestedPages)
	}

	var rows int
	if err := db.DB().QueryRow(
		`SELECT COUNT(*) FROM resources WHERE resource_type = ?`,
		"get-expenses",
	).Scan(&rows); err != nil {
		t.Fatalf("count stored rows: %v", err)
	}
	if rows != 143 {
		t.Fatalf("stored rows = %d, want 143 (the 43 oldest expenses must reach the local store)", rows)
	}
}
