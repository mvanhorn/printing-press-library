package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/drudge"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/drudgereport/internal/store"
	"github.com/spf13/cobra"
)

func openDrudgeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := store.EnsureDrudgeSchema(context.Background(), s.DB()); err != nil {
		t.Fatalf("ensure drudge schema: %v", err)
	}
	return s.DB()
}

func newDrudgeTestCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

func insertDrudgeSnapshot(t *testing.T, db *sql.DB, snapshotID string, capturedAt time.Time) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO drudge_snapshot (snapshot_id, captured_at, source_url, body_hash, story_count)
		 VALUES (?, ?, ?, ?, ?)`,
		snapshotID, capturedAt.UTC().Format(time.RFC3339Nano), "https://example.test", snapshotID+"hash", 1,
	)
	if err != nil {
		t.Fatalf("insert snapshot %s: %v", snapshotID, err)
	}
}

func insertDrudgeStory(t *testing.T, db *sql.DB, snapshotID, storyID, title, url string, slot drudge.Slot, isRed bool, domain string, capturedAt time.Time) {
	t.Helper()
	red := 0
	if isRed {
		red = 1
	}
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO drudge_story (snapshot_id, story_id, title, url, slot, slot_index, is_red, has_image, image_url, outbound_domain, captured_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshotID, storyID, title, url, string(slot), 0, red, 0, nil, domain, capturedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert story %s/%s: %v", snapshotID, storyID, err)
	}
}

func insertDrudgeEvent(t *testing.T, db *sql.DB, eventID, snapshotID, storyID string, eventType drudge.SlotEventType, fromSlot, toSlot drudge.Slot, capturedAt time.Time) {
	t.Helper()
	var from any
	if fromSlot != "" {
		from = string(fromSlot)
	}
	var to any
	if toSlot != "" {
		to = string(toSlot)
	}
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO drudge_slot_event (event_id, snapshot_id, story_id, event_type, from_slot, to_slot, captured_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		eventID, snapshotID, storyID, string(eventType), from, to, capturedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert event %s: %v", eventID, err)
	}
}

func TestQueryDigestTopDomainsFiltersEmptyDomain(t *testing.T) {
	db := openDrudgeTestDB(t)
	cmd := newDrudgeTestCmd()
	now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	insertDrudgeSnapshot(t, db, "snap1", now)
	insertDrudgeStory(t, db, "snap1", "story1", "Domain", "https://example.com/a", drudge.SlotColumn1, false, "example.com", now)
	insertDrudgeStory(t, db, "snap1", "story2", "Empty", "not a url", drudge.SlotColumn1, false, "", now)

	rows, err := queryDigestTopDomains(cmd, db, now.Add(-time.Hour).Format(time.RFC3339Nano), now.Add(time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("queryDigestTopDomains: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("top domains length = %d, want 1 (%+v)", len(rows), rows)
	}
	if got := rows[0]["outbound_domain"]; got != "example.com" {
		t.Fatalf("outbound_domain = %v, want example.com", got)
	}
}

func TestQueryDigestRedSurgesReportsTotalTenureSeparately(t *testing.T) {
	db := openDrudgeTestDB(t)
	cmd := newDrudgeTestCmd()
	t0 := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	t2 := t0.Add(2 * time.Hour)
	t3 := t0.Add(3 * time.Hour)
	for i, ts := range []time.Time{t0, t1, t2, t3} {
		snapshotID := fmt.Sprintf("snap-red-%d", i)
		insertDrudgeSnapshot(t, db, snapshotID, ts)
		insertDrudgeStory(t, db, snapshotID, "story-red", "Red Story", "https://example.com/red", drudge.SlotColumn1, ts == t1 || ts == t2, "example.com", ts)
	}

	rows, err := queryDigestRedSurges(cmd, db, t0.Format(time.RFC3339Nano), t3.Add(time.Second).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("queryDigestRedSurges: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("red surges length = %d, want 1 (%+v)", len(rows), rows)
	}
	if got, want := rows[0]["red_tenure_seconds"], int64(3600); got != want {
		t.Fatalf("red_tenure_seconds = %v, want %v", got, want)
	}
	if got, want := rows[0]["total_tenure_seconds"], int64(10800); got != want {
		t.Fatalf("total_tenure_seconds = %v, want %v", got, want)
	}
}

func TestQuerySplashTenureHistoryUsesMostRecentPromotion(t *testing.T) {
	db := openDrudgeTestDB(t)
	cmd := newDrudgeTestCmd()
	t1 := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t1.Add(2 * time.Hour)
	t4 := t1.Add(3 * time.Hour)

	insertDrudgeSnapshot(t, db, "snap-tenure-1", t1)
	insertDrudgeStory(t, db, "snap-tenure-1", "story-tenure", "Splash", "https://example.com/splash", drudge.SlotSplash, false, "example.com", t1)
	insertDrudgeEvent(t, db, "event-tenure-1", "snap-tenure-1", "story-tenure", drudge.EventAppeared, "", drudge.SlotSplash, t1)
	insertDrudgeSnapshot(t, db, "snap-tenure-2", t2)
	insertDrudgeStory(t, db, "snap-tenure-2", "story-tenure", "Splash", "https://example.com/splash", drudge.SlotColumn1, false, "example.com", t2)
	insertDrudgeEvent(t, db, "event-tenure-2", "snap-tenure-2", "story-tenure", drudge.EventDemotedFromSplash, drudge.SlotSplash, drudge.SlotColumn1, t2)
	insertDrudgeSnapshot(t, db, "snap-tenure-3", t3)
	insertDrudgeStory(t, db, "snap-tenure-3", "story-tenure", "Splash", "https://example.com/splash", drudge.SlotSplash, false, "example.com", t3)
	insertDrudgeEvent(t, db, "event-tenure-3", "snap-tenure-3", "story-tenure", drudge.EventPromotedToSplash, drudge.SlotColumn1, drudge.SlotSplash, t3)
	insertDrudgeSnapshot(t, db, "snap-tenure-4", t4)
	insertDrudgeStory(t, db, "snap-tenure-4", "story-tenure", "Splash", "https://example.com/splash", drudge.SlotSplash, false, "example.com", t4)

	rows, err := querySplashTenureHistory(cmd, db, 10)
	if err != nil {
		t.Fatalf("querySplashTenureHistory: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("history length = %d, want 1 (%+v)", len(rows), rows)
	}
	if got, want := rows[0]["first_seen_at"], t3.Format(time.RFC3339Nano); got != want {
		t.Fatalf("first_seen_at = %v, want %v", got, want)
	}
	if got, want := rows[0]["splash_tenure_seconds"], int64(3600); got != want {
		t.Fatalf("splash_tenure_seconds = %v, want %v", got, want)
	}
}

func TestSplashTenureSecondsIgnoresPriorColumnAppearance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ctx := context.Background()
	s, err := store.OpenWithContext(ctx, defaultDBPath("drudgereport-pp-cli"))
	if err != nil {
		t.Fatalf("open default store: %v", err)
	}
	if err := store.EnsureDrudgeSchema(ctx, s.DB()); err != nil {
		t.Fatalf("ensure drudge schema: %v", err)
	}
	t1 := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t1.Add(2 * time.Hour)
	insertDrudgeSnapshot(t, s.DB(), "snap-splash-1", t1)
	insertDrudgeStory(t, s.DB(), "snap-splash-1", "story-splash", "Story", "https://example.com/story", drudge.SlotColumn1, false, "example.com", t1)
	insertDrudgeSnapshot(t, s.DB(), "snap-splash-2", t2)
	insertDrudgeStory(t, s.DB(), "snap-splash-2", "story-splash", "Story", "https://example.com/story", drudge.SlotSplash, false, "example.com", t2)
	if err := s.Close(); err != nil {
		t.Fatalf("close default store: %v", err)
	}

	got, err := splashTenureSeconds(ctx, "story-splash", t3)
	if err != nil {
		t.Fatalf("splashTenureSeconds: %v", err)
	}
	if got != 3600 {
		t.Fatalf("splashTenureSeconds = %d, want 3600", got)
	}
}

func TestQueryTailEventsUsesJoinedLatestStory(t *testing.T) {
	db := openDrudgeTestDB(t)
	cmd := newDrudgeTestCmd()
	t1 := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	insertDrudgeSnapshot(t, db, "snap-tail-1", t1)
	insertDrudgeStory(t, db, "snap-tail-1", "story-tail", "Old Title", "https://example.com/old", drudge.SlotColumn1, false, "example.com", t1)
	insertDrudgeSnapshot(t, db, "snap-tail-2", t2)
	insertDrudgeStory(t, db, "snap-tail-2", "story-tail", "Latest Title", "https://example.com/latest", drudge.SlotSplash, false, "example.com", t2)
	insertDrudgeEvent(t, db, "event-tail-1", "snap-tail-1", "story-tail", drudge.EventAppeared, "", drudge.SlotColumn1, t1)

	rows, err := queryTailEvents(cmd, db, `SELECT e.event_id, e.snapshot_id, e.story_id, e.event_type, e.from_slot, e.to_slot, e.captured_at,
		COALESCE(s.title, ''), COALESCE(s.url, '')
		FROM drudge_slot_event e
		LEFT JOIN (
			SELECT story_id, title, url,
				ROW_NUMBER() OVER (PARTITION BY story_id ORDER BY captured_at DESC) AS rn
			FROM drudge_story
		) s ON s.story_id = e.story_id AND s.rn = 1
		WHERE e.captured_at >= ?
		ORDER BY e.captured_at DESC, e.event_id`, t1.Add(-time.Minute).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("queryTailEvents: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("tail rows length = %d, want 1 (%+v)", len(rows), rows)
	}
	if got := rows[0]["title"]; got != "Latest Title" {
		t.Fatalf("title = %v, want Latest Title", got)
	}
}

func TestQueryOnDateEmptyStoreUsesNotFoundExitCode(t *testing.T) {
	db := openDrudgeTestDB(t)
	_, err := queryOnDate(newDrudgeTestCmd(), db, time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("queryOnDate returned nil error, want not found")
	}
	var cliErr *cliError
	if !errors.As(err, &cliErr) {
		t.Fatalf("queryOnDate error type = %T, want *cliError", err)
	}
	if cliErr.code == 4 {
		t.Fatalf("queryOnDate exit code = 4, want non-auth code")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode(queryOnDate error) = %d, want 3", got)
	}
}
