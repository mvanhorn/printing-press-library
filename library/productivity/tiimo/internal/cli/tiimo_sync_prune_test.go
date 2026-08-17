package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/store"
)

func mkOccurrence(t *testing.T, st *store.Store, id, day, title string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"activityId": id,
		"title":      title,
		"startTime":  day + "T00:00:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	keyed, key, err := occurrenceID(raw, day)
	if err != nil {
		t.Fatalf("occurrenceID: %v", err)
	}
	if err := st.UpsertActivities(keyed); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return key
}

func countRows(t *testing.T, st *store.Store, q string) int {
	t.Helper()
	var n int
	if err := st.DB().QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("count (%s): %v", q, err)
	}
	return n
}

func TestPruneStaleOccurrences(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	from := time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 8, 20, 0, 0, 0, 0, time.Local)

	keepInWindow := mkOccurrence(t, st, "act-1", "2026-08-12", "kept")
	staleInWindow := mkOccurrence(t, st, "act-2", "2026-08-13", "stale")
	// Same activity, a second occurrence: proves occurrence-level granularity.
	staleSameActivity := mkOccurrence(t, st, "act-1", "2026-08-14", "moved away")
	outsideWindow := mkOccurrence(t, st, "act-3", "2026-09-01", "untouched")

	if got := countRows(t, st, `SELECT COUNT(*) FROM activities`); got != 4 {
		t.Fatalf("setup: want 4 rows, got %d", got)
	}

	seen := map[string]bool{keepInWindow: true}
	removed, err := pruneStaleOccurrences(st, from, to, seen, true)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 2 {
		t.Fatalf("want 2 removed (%q, %q), got %d", staleInWindow, staleSameActivity, removed)
	}

	if got := countRows(t, st, `SELECT COUNT(*) FROM activities`); got != 2 {
		t.Fatalf("typed table: want 2 survivors, got %d", got)
	}
	// The generic mirror backs `backup` and `export`; leaving it stale was the
	// reported symptom, so assert it too.
	if got := countRows(t, st, `SELECT COUNT(*) FROM resources WHERE resource_type='activities'`); got != 2 {
		t.Fatalf("generic table: want 2 survivors, got %d", got)
	}

	survivors := map[string]bool{}
	rows, err := st.DB().Query(`SELECT id FROM activities`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		survivors[id] = true
	}
	if !survivors[keepInWindow] {
		t.Error("deleted an occurrence this run saw")
	}
	if !survivors[outsideWindow] {
		t.Error("deleted an occurrence outside the synced window")
	}
	if survivors[staleInWindow] || survivors[staleSameActivity] {
		t.Error("kept an occurrence that is gone upstream")
	}
}

func TestPruneRefusesWithoutEvidence(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	mkOccurrence(t, st, "act-1", "2026-08-12", "kept")

	// An empty seen-set means the run recorded nothing; treating that as
	// "everything is stale" would erase the mirror.
	removed, err := pruneStaleOccurrences(st, time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local),
		time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local), map[string]bool{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("empty seen-set must delete nothing, deleted %d", removed)
	}
	if got := countRows(t, st, `SELECT COUNT(*) FROM activities`); got != 1 {
		t.Fatalf("row was destroyed: %d", got)
	}
}

func TestCanPruneOccurrencesGuards(t *testing.T) {
	all := func(string) bool { return true }
	ok := []syncResourceResult{{Resource: "activities", Status: "ok"}, {Resource: "calendar_events", Status: "ok"}}

	if can, why := canPruneOccurrences(ok, all, 0, false); !can {
		t.Fatalf("clean run should prune, refused: %s", why)
	}
	if can, _ := canPruneOccurrences(ok, all, 5, false); can {
		t.Error("--max-pages truncation must block pruning")
	}
	failed := []syncResourceResult{{Resource: "activities", Status: "error"}, {Resource: "calendar_events", Status: "ok"}}
	if can, _ := canPruneOccurrences(failed, all, 0, false); can {
		t.Error("a failed resource must block pruning")
	}
	calFailed := []syncResourceResult{{Resource: "activities", Status: "ok"}, {Resource: "calendar_events", Status: "error"}}
	if can, _ := canPruneOccurrences(calFailed, all, 0, false); can {
		t.Error("a failed calendar sync must block pruning: its occurrences are missing from the seen set")
	}
	onlyActivities := func(n string) bool { return n == "activities" }
	if can, _ := canPruneOccurrences(ok, onlyActivities, 0, false); can {
		t.Error("calendar events share the table; skipping them must block pruning")
	}
	missing := []syncResourceResult{{Resource: "activities", Status: "ok"}}
	if can, _ := canPruneOccurrences(missing, all, 0, false); can {
		t.Error("a resource with no result must block pruning")
	}
	// --profile syncs one profile, so the seen set cannot account for the
	// others -- and the sweep cannot be narrowed, because activity rows carry
	// no profile column. Pruning here would delete other profiles' data.
	if can, why := canPruneOccurrences(ok, all, 0, true); can {
		t.Error("--profile makes the seen set partial and must block pruning")
	} else if !strings.Contains(why, "--profile") {
		t.Errorf("refusal should name --profile, got %q", why)
	}
}

func mkCalendarOccurrence(t *testing.T, st *store.Store, id, day, calID string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"activityId": id,
		"title":      "imported event",
		"startTime":  day + "T09:00:00",
		"calendarId": calID,
	})
	if err != nil {
		t.Fatal(err)
	}
	keyed, key, err := occurrenceID(raw, day)
	if err != nil {
		t.Fatalf("occurrenceID: %v", err)
	}
	if err := st.UpsertActivities(keyed); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	return key
}

// A hidden or disconnected calendar is skipped by syncCalendarEvents, so its
// occurrences never reach the seen set. Without protection, pruning would read
// that absence as deletion and destroy the mirrored history of a calendar the
// user merely hid.
func TestHiddenCalendarRowsSurvivePruning(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	from := time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	to := time.Date(2026, 8, 20, 0, 0, 0, 0, time.Local)

	hiddenA := mkCalendarOccurrence(t, st, "ev-1", "2026-08-12", "cal-hidden")
	hiddenB := mkCalendarOccurrence(t, st, "ev-2", "2026-08-15", "cal-hidden")
	otherCal := mkCalendarOccurrence(t, st, "ev-3", "2026-08-13", "cal-active")
	native := mkOccurrence(t, st, "act-1", "2026-08-14", "native, gone upstream")

	seen := map[string]bool{}
	if err := retainCalendarRows(st, "cal-hidden", seen); err != nil {
		t.Fatalf("retainCalendarRows: %v", err)
	}
	if !seen[hiddenA] || !seen[hiddenB] {
		t.Fatalf("hidden calendar rows were not retained: %v", seen)
	}
	if seen[otherCal] || seen[native] {
		t.Error("retained rows belonging to a different calendar")
	}

	removed, err := pruneStaleOccurrences(st, from, to, seen, true)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	// Only the two rows this run neither fetched nor retained should go.
	if removed != 2 {
		t.Fatalf("want 2 removed (the active-calendar and native rows), got %d", removed)
	}
	surv := map[string]bool{}
	rows, err := st.DB().Query(`SELECT id FROM activities`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		surv[id] = true
	}
	if !surv[hiddenA] || !surv[hiddenB] {
		t.Error("hiding a calendar destroyed its mirrored events")
	}
}

// Deleting the last activity in a window legitimately yields zero keys. When
// the caller has verified the window was fully enumerated, that empty set is
// evidence of deletion and must prune -- otherwise the final removed
// occurrence stays visible in every mirror-backed command forever.
func TestVerifiedEmptyWindowPrunes(t *testing.T) {
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	mkOccurrence(t, st, "act-1", "2026-08-12", "deleted upstream")
	mkOccurrence(t, st, "act-2", "2026-09-30", "outside the window")

	removed, err := pruneStaleOccurrences(st,
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local),
		time.Date(2026, 8, 31, 0, 0, 0, 0, time.Local),
		map[string]bool{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("verified empty window should prune the in-window row, removed %d", removed)
	}
	if got := countRows(t, st, `SELECT COUNT(*) FROM activities`); got != 1 {
		t.Fatalf("out-of-window row must survive, rows=%d", got)
	}
}
