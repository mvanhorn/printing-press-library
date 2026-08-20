package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func insertCompetitorSnapshot(t *testing.T, dbPath, owner, username string, followers, mediaCount int64, recentEng float64, capturedAt string) {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	_, err = db.DB().Exec(
		`INSERT INTO ig_competitor_snapshots(owner_slug, username, followers_count, media_count, recent_avg_engagement, captured_at)
		 VALUES (?,?,?,?,?,?)`,
		owner, username, followers, mediaCount, recentEng, capturedAt)
	if err != nil {
		t.Fatalf("insert competitor snapshot: %v", err)
	}
}

func TestNovelRivalsFollowerChange(t *testing.T) {
	dbPath := newTestDB(t)
	// earliest 8000, latest 8600 -> change 600.
	insertCompetitorSnapshot(t, dbPath, "alpha", "rivalco", 8000, 200, 120.0, rfcAgo(20*day))
	insertCompetitorSnapshot(t, dbPath, "alpha", "rivalco", 8600, 205, 150.0, rfcAgo(1*day))

	out := execTestJSON(t, newNovelRivalsCmd, dbPath, "--since", "30d")
	rivals := asSlice(t, out, "rivals")
	if len(rivals) != 1 {
		t.Fatalf("want 1 rival, got %d", len(rivals))
	}
	r := rivals[0]
	if got := num(t, r, "start_followers"); got != 8000 {
		t.Errorf("start_followers = %v, want 8000", got)
	}
	if got := num(t, r, "end_followers"); got != 8600 {
		t.Errorf("end_followers = %v, want 8600", got)
	}
	if got := num(t, r, "follower_change"); got != 600 {
		t.Errorf("follower_change = %v, want 600 (latest-earliest)", got)
	}
	if got := num(t, r, "recent_avg_engagement"); got != 150 {
		t.Errorf("recent_avg_engagement = %v, want 150 (latest)", got)
	}
}

func TestNovelRivalsEmptyNote(t *testing.T) {
	dbPath := newTestDB(t)
	out := execTestJSON(t, newNovelRivalsCmd, dbPath)
	rivals := asSlice(t, out, "rivals")
	if len(rivals) != 0 {
		t.Fatalf("want 0 rivals, got %d", len(rivals))
	}
	if _, ok := out["note"]; !ok {
		t.Errorf("expected a note pointing to brands track-rival")
	}
}
