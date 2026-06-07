package cli

import "testing"

func TestNovelTopPostsRanksByMetric(t *testing.T) {
	dbPath := newTestDB(t)
	// All within the default 30d window (posted_at recent).
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "hi", productType: "FEED", postedAt: rfcAgo(2 * day), reach: 9000, interactions: 100, likeCount: 50, commentCount: 5})
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "mid", productType: "FEED", postedAt: rfcAgo(3 * day), reach: 4000, interactions: 300, likeCount: 40, commentCount: 4})
	insertMedia(t, dbPath, mediaFixture{slug: "alpha", mediaID: "lo", productType: "FEED", postedAt: rfcAgo(4 * day), reach: 100, interactions: 10, likeCount: 1, commentCount: 0})

	// Rank by reach: "hi" first, "lo" must NOT be first.
	out := execTestJSON(t, newNovelTopPostsCmd, dbPath, "--metric", "reach")
	posts := asSlice(t, out, "posts")
	if len(posts) != 3 {
		t.Fatalf("want 3 posts, got %d", len(posts))
	}
	if got := str(t, posts[0], "media_id"); got != "hi" {
		t.Errorf("top by reach = %q, want hi", got)
	}
	if got := str(t, posts[0], "media_id"); got == "lo" {
		t.Errorf("low-reach post must not rank first")
	}
	if m := num(t, posts[0], "metric"); m != 9000 {
		t.Errorf("top metric value = %v, want 9000", m)
	}

	// --limit respected.
	out2 := execTestJSON(t, newNovelTopPostsCmd, dbPath, "--metric", "reach", "--limit", "1")
	posts2 := asSlice(t, out2, "posts")
	if len(posts2) != 1 {
		t.Fatalf("--limit 1: want 1 post, got %d", len(posts2))
	}

	// Different metric flips ranking: by interactions "mid" wins.
	out3 := execTestJSON(t, newNovelTopPostsCmd, dbPath, "--metric", "interactions")
	posts3 := asSlice(t, out3, "posts")
	if got := str(t, posts3[0], "media_id"); got != "mid" {
		t.Errorf("top by interactions = %q, want mid", got)
	}
}
