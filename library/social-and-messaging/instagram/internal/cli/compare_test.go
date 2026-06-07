package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func insertAccountSnapshot(t *testing.T, dbPath, slug string, followers, reach, interactions, views int64, capturedAt string) {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	_, err = db.DB().Exec(
		`INSERT INTO ig_account_snapshots(slug, ig_user_id, followers_count, follows_count, media_count, reach, total_interactions, accounts_engaged, views, captured_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		slug, "ig_"+slug, followers, 100, 50, reach, interactions, 0, views, capturedAt)
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
}

func TestNovelCompareRanksByReach(t *testing.T) {
	dbPath := newTestDB(t)
	// reach: bravo > alpha > charlie. alpha has known ER = 200/1000 = 0.2.
	insertAccountSnapshot(t, dbPath, "alpha", 5000, 1000, 200, 800, rfcAgo(1*hour))
	insertAccountSnapshot(t, dbPath, "bravo", 9000, 4000, 300, 2000, rfcAgo(2*hour))
	insertAccountSnapshot(t, dbPath, "charlie", 1000, 200, 80, 100, rfcAgo(3*hour))

	out := execTestJSON(t, newNovelCompareCmd, dbPath, "--metric", "reach", "--since", "30d")
	brands := asSlice(t, out, "brands")
	if len(brands) != 3 {
		t.Fatalf("want 3 brands, got %d", len(brands))
	}
	wantOrder := []string{"bravo", "alpha", "charlie"}
	for i, w := range wantOrder {
		if got := str(t, brands[i], "slug"); got != w {
			t.Errorf("position %d: got %q, want %q (reach order)", i, got, w)
		}
	}
	// engagement_rate for alpha (position 1) = 200/1000 = 0.2.
	if er := num(t, brands[1], "engagement_rate"); er < 0.1999 || er > 0.2001 {
		t.Errorf("alpha engagement_rate = %v, want 0.2", er)
	}
}

func TestNovelCompareEmptyStore(t *testing.T) {
	dbPath := newTestDB(t)
	out := execTestJSON(t, newNovelCompareCmd, dbPath)
	brands := asSlice(t, out, "brands")
	if len(brands) != 0 {
		t.Fatalf("want 0 brands on empty store, got %d", len(brands))
	}
	if _, ok := out["note"]; !ok {
		t.Errorf("expected a note on empty store, got %v", out)
	}
}
