package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func insertHashtagSnapshot(t *testing.T, dbPath, slug, hashtag, hashtagID string, reach, engagement, count int64, capturedAt string) {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	_, err = db.DB().Exec(
		`INSERT INTO ig_hashtag_snapshots(slug, hashtag, hashtag_id, top_media_reach, top_media_engagement, top_media_count, captured_at)
		 VALUES (?,?,?,?,?,?,?)`,
		slug, hashtag, hashtagID, reach, engagement, count, capturedAt)
	if err != nil {
		t.Fatalf("insert hashtag snapshot: %v", err)
	}
}

func TestNovelHashtagPerfRankedByEngagement(t *testing.T) {
	dbPath := newTestDB(t)
	// coffee engagement 900 > latte 300.
	insertHashtagSnapshot(t, dbPath, "alpha", "coffee", "ht_coffee", 0, 900, 9, rfcAgo(1*day))
	insertHashtagSnapshot(t, dbPath, "alpha", "latte", "ht_latte", 0, 300, 9, rfcAgo(1*day))

	out := execTestJSON(t, newNovelHashtagPerfCmd, dbPath)
	hashtags := asSlice(t, out, "hashtags")
	if len(hashtags) != 2 {
		t.Fatalf("want 2 hashtags, got %d", len(hashtags))
	}
	if got := str(t, hashtags[0], "hashtag"); got != "coffee" {
		t.Errorf("top hashtag = %q, want coffee (highest engagement)", got)
	}
	if got := num(t, hashtags[0], "top_media_engagement"); got != 900 {
		t.Errorf("top engagement = %v, want 900", got)
	}
}

func TestNovelHashtagPerfEmptyNote(t *testing.T) {
	dbPath := newTestDB(t)
	out := execTestJSON(t, newNovelHashtagPerfCmd, dbPath)
	hashtags := asSlice(t, out, "hashtags")
	if len(hashtags) != 0 {
		t.Fatalf("want 0 hashtags, got %d", len(hashtags))
	}
	if _, ok := out["note"]; !ok {
		t.Errorf("expected a note pointing to brands track-hashtag")
	}
}
