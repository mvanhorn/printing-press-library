package store

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"
)

// Refresh semantics for SaveNCCPLDate.
//
// A stored (resource, date) must mirror the snapshot that was last fetched for it.
// The original implementation only upserted, so a revised date -- NCCPL dropping six
// delisted symbols from var-margins, say -- left the six obsolete rows in nccpl_obs
// forever while coverage recorded the smaller count. Every reader (panel, search,
// export, leverage, universe, risk-changes) then served rows the source no longer
// publishes, and the coverage audit disagreed with the data it was auditing.
//
// These tests pin the mirror, the vintage rule that survives it, and the deliberate
// exception for an empty snapshot.

func nccplStoredKeys(t *testing.T, s *Store, resource, date string) []string {
	t.Helper()
	obs, err := NCCPLObservations(context.Background(), s, resource, date, date)
	if err != nil {
		t.Fatalf("observations %s/%s: %v", resource, date, err)
	}
	keys := make([]string, 0, len(obs))
	for _, o := range obs {
		keys = append(keys, o.Key)
	}
	sort.Strings(keys)
	return keys
}

func nccplCoverageFor(t *testing.T, s *Store, resource, date string) NCCPLCoverage {
	t.Helper()
	cov, err := NCCPLCoveredDates(context.Background(), s, resource)
	if err != nil {
		t.Fatalf("coverage %s: %v", resource, err)
	}
	for _, c := range cov {
		if c.Date == date {
			return c
		}
	}
	t.Fatalf("no coverage row for %s/%s (have %d rows)", resource, date, len(cov))
	return NCCPLCoverage{}
}

// assertCoverageMatchesStore pins the invariant SaveNCCPLDate exists to maintain:
// the audit ledger must agree with the observations it audits.
func assertCoverageMatchesStore(t *testing.T, s *Store, resource, date string) {
	t.Helper()
	cov := nccplCoverageFor(t, s, resource, date)
	stored := len(nccplStoredKeys(t, s, resource, date))
	if cov.RowCount != stored {
		t.Errorf("coverage row_count = %d but %d observations are stored for %s/%s; the audit must never disagree with the data it audits",
			cov.RowCount, stored, resource, date)
	}
}

// A shrinking snapshot must take the vanished rows with it. Keeping them would serve
// symbols the source has stopped publishing, which is an observation NCCPL never made.
func TestSaveNCCPLDateRefreshDropsVanishedRows(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)

	first := []NCCPLRow{
		{Key: "HUBC", Payload: `{"symbol":"HUBC","var":"15.5"}`},
		{Key: "OGDC", Payload: `{"symbol":"OGDC","var":"12.0"}`},
		{Key: "DELISTED1", Payload: `{"symbol":"DELISTED1","var":"30.0"}`},
		{Key: "DELISTED2", Payload: `{"symbol":"DELISTED2","var":"30.0"}`},
	}
	if err := SaveNCCPLDate(ctx, s, "var-margins", "2026-09-04", first, time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if got := len(nccplStoredKeys(t, s, "var-margins", "2026-09-04")); got != 4 {
		t.Fatalf("after first save got %d rows, want 4", got)
	}

	// The source revises the date: the two delisted symbols are gone.
	revised := []NCCPLRow{
		{Key: "HUBC", Payload: `{"symbol":"HUBC","var":"15.5"}`},
		{Key: "OGDC", Payload: `{"symbol":"OGDC","var":"12.0"}`},
	}
	if err := SaveNCCPLDate(ctx, s, "var-margins", "2026-09-04", revised, time.Now()); err != nil {
		t.Fatalf("refresh save: %v", err)
	}

	keys := nccplStoredKeys(t, s, "var-margins", "2026-09-04")
	want := []string{"HUBC", "OGDC"}
	if len(keys) != len(want) {
		t.Fatalf("after refresh got keys %v, want %v -- rows absent from the new snapshot must be deleted", keys, want)
	}
	for i, k := range want {
		if keys[i] != k {
			t.Fatalf("after refresh got keys %v, want %v", keys, want)
		}
	}
	assertCoverageMatchesStore(t, s, "var-margins", "2026-09-04")
	if cov := nccplCoverageFor(t, s, "var-margins", "2026-09-04"); cov.RowCount != 2 {
		t.Errorf("coverage row_count = %d, want 2", cov.RowCount)
	}
}

// Mirroring must not disturb the vintage of a row the source still publishes: a key
// present in both snapshots keeps the observed_at of the fetch that first served it,
// while its payload is refreshed to the current value.
func TestSaveNCCPLDateRefreshKeepsVintageOfSurvivors(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)

	firstFetch := time.Now().Add(-72 * time.Hour)
	if err := SaveNCCPLDate(ctx, s, "var-margins", "2026-09-04", []NCCPLRow{
		{Key: "HUBC", Payload: `{"symbol":"HUBC","var":"15.5"}`},
		{Key: "DELISTED", Payload: `{"symbol":"DELISTED","var":"30.0"}`},
	}, firstFetch); err != nil {
		t.Fatalf("first save: %v", err)
	}
	before, err := NCCPLObservations(ctx, s, "var-margins", "", "")
	if err != nil || len(before) != 2 {
		t.Fatalf("read back: %v (%d rows)", err, len(before))
	}
	var firstVintage string
	for _, o := range before {
		if o.Key == "HUBC" {
			firstVintage = o.ObservedAt
		}
	}
	if firstVintage == "" {
		t.Fatal("HUBC missing from first snapshot")
	}

	if err := SaveNCCPLDate(ctx, s, "var-margins", "2026-09-04", []NCCPLRow{
		{Key: "HUBC", Payload: `{"symbol":"HUBC","var":"18.25"}`},
	}, time.Now()); err != nil {
		t.Fatalf("refresh save: %v", err)
	}

	after, err := NCCPLObservations(ctx, s, "var-margins", "", "")
	if err != nil {
		t.Fatalf("read back 2: %v", err)
	}
	if len(after) != 1 || after[0].Key != "HUBC" {
		t.Fatalf("after refresh got %d rows (%v), want only HUBC", len(after), nccplStoredKeys(t, s, "var-margins", "2026-09-04"))
	}
	if after[0].Payload != `{"symbol":"HUBC","var":"18.25"}` {
		t.Errorf("payload = %s, want the refreshed value", after[0].Payload)
	}
	if after[0].ObservedAt != firstVintage {
		t.Errorf("observed_at moved from %q to %q; deleting retired keys must not restamp a surviving row's vintage",
			firstVintage, after[0].ObservedAt)
	}
}

// The zero-row decision, pinned.
//
// An empty snapshot over a date that already holds rows does NOT delete them. From the
// store, a genuinely empty upstream response is indistinguishable from a transient
// failure that decodes to zero rows (expired session, Cloudflare interstitial, a HAR
// entry saved without its body). Deleting on that evidence is irreversible and destroys
// a real observation; keeping costs nothing, because the next non-empty snapshot
// mirrors the date correctly. fetched_at stays at the vintage of the fetch that
// actually produced the rows -- moving it forward would claim the source served those
// rows at a moment when it served nothing.
func TestSaveNCCPLDateEmptySnapshotKeepsExistingRows(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)

	realFetch := time.Now().Add(-48 * time.Hour)
	if err := SaveNCCPLDate(ctx, s, "slb", "2026-09-04", []NCCPLRow{
		{Key: "HUBC", Payload: `{"symbol":"HUBC","qty":1000}`},
		{Key: "OGDC", Payload: `{"symbol":"OGDC","qty":500}`},
	}, realFetch); err != nil {
		t.Fatalf("first save: %v", err)
	}
	covBefore := nccplCoverageFor(t, s, "slb", "2026-09-04")

	if err := SaveNCCPLDate(ctx, s, "slb", "2026-09-04", nil, time.Now()); err != nil {
		t.Fatalf("empty save: %v", err)
	}

	keys := nccplStoredKeys(t, s, "slb", "2026-09-04")
	if len(keys) != 2 {
		t.Fatalf("after an empty refresh got %d rows (%v), want the 2 already-observed rows kept -- an empty response is indistinguishable from a transient failure",
			len(keys), keys)
	}
	covAfter := nccplCoverageFor(t, s, "slb", "2026-09-04")
	if covAfter.RowCount != 2 {
		t.Errorf("coverage row_count = %d, want 2 -- it must report what is actually stored", covAfter.RowCount)
	}
	if covAfter.FetchedAt != covBefore.FetchedAt {
		t.Errorf("fetched_at moved from %q to %q; an empty response must not date the kept rows to a fetch that served nothing",
			covBefore.FetchedAt, covAfter.FetchedAt)
	}
	assertCoverageMatchesStore(t, s, "slb", "2026-09-04")
}

// The other half of the zero-row decision: a date with nothing stored still records the
// intentional zero, and re-confirming it moves fetched_at forward, because "the source
// served nothing at this later time" is a true statement about the source.
func TestSaveNCCPLDateEmptySnapshotOnUnseenDateRecordsZero(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)

	firstAttempt := time.Now().Add(-48 * time.Hour)
	if err := SaveNCCPLDate(ctx, s, "slb", "2026-09-04", nil, firstAttempt); err != nil {
		t.Fatalf("empty save: %v", err)
	}
	cov := nccplCoverageFor(t, s, "slb", "2026-09-04")
	if cov.RowCount != 0 {
		t.Errorf("row_count = %d, want 0", cov.RowCount)
	}
	if cov.FetchedAt == "" {
		t.Error("an intentional zero must still record when it was fetched")
	}

	if err := SaveNCCPLDate(ctx, s, "slb", "2026-09-04", nil, time.Now()); err != nil {
		t.Fatalf("second empty save: %v", err)
	}
	again := nccplCoverageFor(t, s, "slb", "2026-09-04")
	if again.RowCount != 0 {
		t.Errorf("row_count = %d after re-confirming empty, want 0", again.RowCount)
	}
	if again.FetchedAt == cov.FetchedAt {
		t.Errorf("fetched_at = %q unchanged; re-confirming a still-empty date is a real new observation of nothing", again.FetchedAt)
	}
	assertCoverageMatchesStore(t, s, "slb", "2026-09-04")
}

// Mirroring is scoped to exactly one (resource, date). A row_key shared with a
// neighbouring date or another resource must be untouched.
func TestSaveNCCPLDateMirrorIsScopedToResourceAndDate(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)

	shared := []NCCPLRow{
		{Key: "HUBC", Payload: `{"symbol":"HUBC"}`},
		{Key: "OGDC", Payload: `{"symbol":"OGDC"}`},
	}
	for _, tgt := range []struct{ resource, date string }{
		{"var-margins", "2026-09-03"},
		{"var-margins", "2026-09-04"},
		{"slb", "2026-09-04"},
	} {
		if err := SaveNCCPLDate(ctx, s, tgt.resource, tgt.date, shared, time.Now()); err != nil {
			t.Fatalf("seed %s/%s: %v", tgt.resource, tgt.date, err)
		}
	}

	if err := SaveNCCPLDate(ctx, s, "var-margins", "2026-09-04",
		[]NCCPLRow{{Key: "HUBC", Payload: `{"symbol":"HUBC"}`}}, time.Now()); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if got := nccplStoredKeys(t, s, "var-margins", "2026-09-04"); len(got) != 1 {
		t.Errorf("mirrored date has %d rows (%v), want 1", len(got), got)
	}
	if got := nccplStoredKeys(t, s, "var-margins", "2026-09-03"); len(got) != 2 {
		t.Errorf("neighbouring date has %d rows (%v), want 2 -- mirroring must be scoped to one date", len(got), got)
	}
	if got := nccplStoredKeys(t, s, "slb", "2026-09-04"); len(got) != 2 {
		t.Errorf("other resource has %d rows (%v), want 2 -- mirroring must be scoped to one resource", len(got), got)
	}
	assertCoverageMatchesStore(t, s, "var-margins", "2026-09-03")
	assertCoverageMatchesStore(t, s, "var-margins", "2026-09-04")
	assertCoverageMatchesStore(t, s, "slb", "2026-09-04")
}

// A real var-margins date carries ~1,100 symbols. The retired-key set is differenced in
// Go rather than pushed into one DELETE ... NOT IN (?, ?, ...), so a wide date cannot
// fail to mirror by exceeding SQLite's bound-parameter limit.
func TestSaveNCCPLDateMirrorsWideSnapshot(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)

	wide := make([]NCCPLRow, 0, 1200)
	for i := 0; i < 1200; i++ {
		wide = append(wide, NCCPLRow{Key: fmt.Sprintf("SYM%04d", i), Payload: fmt.Sprintf(`{"i":%d}`, i)})
	}
	if err := SaveNCCPLDate(ctx, s, "var-margins", "2026-09-04", wide, time.Now()); err != nil {
		t.Fatalf("wide save: %v", err)
	}
	if err := SaveNCCPLDate(ctx, s, "var-margins", "2026-09-04", wide[:1150], time.Now()); err != nil {
		t.Fatalf("wide refresh: %v", err)
	}
	if got := len(nccplStoredKeys(t, s, "var-margins", "2026-09-04")); got != 1150 {
		t.Errorf("after wide refresh got %d rows, want 1150", got)
	}
	assertCoverageMatchesStore(t, s, "var-margins", "2026-09-04")
}

// Coverage counts what is stored, not what was handed in: a snapshot repeating a key
// stores one row, so row_count must be the distinct count or the audit lies by one.
func TestSaveNCCPLDateCoverageCountsDistinctKeys(t *testing.T) {
	ctx := context.Background()
	s := newNCCPLTestStore(t)

	if err := SaveNCCPLDate(ctx, s, "fipi", "2026-09-04", []NCCPLRow{
		{Key: "FI|EQUITY", Payload: `{"net":1}`},
		{Key: "FI|EQUITY", Payload: `{"net":2}`},
		{Key: "FC|EQUITY", Payload: `{"net":3}`},
	}, time.Now()); err != nil {
		t.Fatalf("save: %v", err)
	}
	assertCoverageMatchesStore(t, s, "fipi", "2026-09-04")
	if cov := nccplCoverageFor(t, s, "fipi", "2026-09-04"); cov.RowCount != 2 {
		t.Errorf("row_count = %d, want 2 distinct keys", cov.RowCount)
	}
}
