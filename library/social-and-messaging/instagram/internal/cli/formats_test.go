package cli

import "testing"

func TestNovelFormatsGroupsByProductType(t *testing.T) {
	dbPath := newTestDB(t)
	// 2 REELS, 1 FEED.
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "r1", productType: "REELS", postedAt: rfcAgo(1 * day), reach: 1000, interactions: 100, reelsWatch: 12.5})
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "r2", productType: "REELS", postedAt: rfcAgo(2 * day), reach: 3000, interactions: 200, reelsWatch: 17.5})
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "f1", productType: "FEED", postedAt: rfcAgo(3 * day), reach: 500, interactions: 50})

	out := execTestJSON(t, newNovelFormatsCmd, dbPath)
	formats := asSlice(t, out, "formats")
	byType := map[string]map[string]any{}
	for _, f := range formats {
		byType[str(t, f, "media_product_type")] = f
	}
	reels, ok := byType["REELS"]
	if !ok {
		t.Fatalf("missing REELS group; got %v", byType)
	}
	feed, ok := byType["FEED"]
	if !ok {
		t.Fatalf("missing FEED group; got %v", byType)
	}
	if p := num(t, reels, "posts"); p != 2 {
		t.Errorf("REELS posts = %v, want 2", p)
	}
	if p := num(t, feed, "posts"); p != 1 {
		t.Errorf("FEED posts = %v, want 1", p)
	}
	// avg reach for REELS = (1000+3000)/2 = 2000.
	if ar := num(t, reels, "avg_reach"); ar != 2000 {
		t.Errorf("REELS avg_reach = %v, want 2000", ar)
	}
	// avg reels watch-time = (12.5+17.5)/2 = 15.
	if w := num(t, reels, "avg_reels_watch_time"); w != 15 {
		t.Errorf("REELS avg_reels_watch_time = %v, want 15", w)
	}
}
