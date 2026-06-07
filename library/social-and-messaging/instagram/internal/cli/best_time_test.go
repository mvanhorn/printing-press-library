package cli

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

// mediaFixture is a minimal row for ig_brand_media inserts in tests.
type mediaFixture struct {
	slug         string
	mediaID      string
	productType  string
	postedAt     string
	likeCount    int64
	commentCount int64
	reach        int64
	views        int64
	saved        int64
	shares       int64
	interactions int64
	reelsWatch   float64
	caption      string
}

func insertMedia(t *testing.T, dbPath string, m mediaFixture) {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	_, err = db.DB().Exec(
		`INSERT OR REPLACE INTO ig_brand_media(
			slug, ig_user_id, media_id, caption, media_type, media_product_type, permalink, posted_at,
			like_count, comments_count, reach, views, saved, shares, total_interactions, reels_avg_watch_time, captured_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.slug, "ig_"+m.slug, m.mediaID, m.caption, "VIDEO", m.productType,
		"https://instagram.com/p/"+m.mediaID, m.postedAt,
		m.likeCount, m.commentCount, m.reach, m.views, m.saved, m.shares, m.interactions, m.reelsWatch,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("insert media: %v", err)
	}
}

func TestNovelBestTimeSlotAppears(t *testing.T) {
	dbPath := newTestDB(t)
	// Two posts on Monday 09:00 UTC (high interactions), one on Friday 22:00.
	mon0900a := "2026-06-01T09:00:00+0000" // 2026-06-01 is a Monday
	mon0900b := "2026-06-08T09:15:00+0000" // also a Monday, same hour bucket
	fri2200 := "2026-06-05T22:00:00+0000"  // Friday
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "m1", productType: "FEED", postedAt: mon0900a, interactions: 500})
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "m2", productType: "FEED", postedAt: mon0900b, interactions: 700})
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "m3", productType: "FEED", postedAt: fri2200, interactions: 50})

	out := execTestJSON(t, newNovelBestTimeCmd, dbPath)
	slots := asSlice(t, out, "slots")
	if len(slots) == 0 {
		t.Fatalf("expected slots, got none: %v", out)
	}
	// Top slot must be Monday 09:00 with 2 posts and avg (500+700)/2 = 600.
	top := slots[0]
	if wd := str(t, top, "weekday"); wd != "Monday" {
		t.Errorf("top slot weekday = %q, want Monday", wd)
	}
	if h := num(t, top, "hour"); h != 9 {
		t.Errorf("top slot hour = %v, want 9", h)
	}
	if p := num(t, top, "posts"); p != 2 {
		t.Errorf("top slot posts = %v, want 2", p)
	}
	if avg := num(t, top, "avg_interactions"); avg != 600 {
		t.Errorf("top slot avg_interactions = %v, want 600", avg)
	}
}
