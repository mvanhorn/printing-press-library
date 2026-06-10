// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"testing"
)

// seedMakeModelRows inserts two listings that expose the make-vs-model
// column-binding bug:
//   - hc: make=Honda, model=Civic  (a genuine Honda)
//   - fh: make=Ford,  model=Honda  (model literally "Honda" — must NOT match --make honda)
func seedMakeModelRows(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	insertListingMM(t, sqlDB, "hc", "Honda", "Civic", 2000000)
	insertListingMM(t, sqlDB, "fh", "Ford", "Honda", 2100000)
}

// rowSet collects a string field across result rows into a set for assertion.
func rowSet(rows []map[string]any, field string) map[string]bool {
	out := map[string]bool{}
	for _, r := range rows {
		if v, ok := r[field].(string); ok {
			out[v] = true
		}
	}
	return out
}

func TestSpreadMakeBindsMakeColumnOnly(t *testing.T) {
	ctx := context.Background()
	sqlDB, cleanup := openTestStore(t)
	defer cleanup()
	seedMakeModelRows(t, sqlDB)

	// --make honda: MAKE column only -> only the Honda/Civic row's source.
	// spread groups by source; both rows use source "te", so distinguish by n.
	mkRows, err := spreadRows(ctx, sqlDB, "honda", "", "")
	if err != nil {
		t.Fatalf("spreadRows --make: %v", err)
	}
	if got := totalN(mkRows); got != 1 {
		t.Errorf("--make honda matched %d listings, want 1 (Honda/Civic only, not Ford/Honda)", got)
	}

	// positional "honda": ambiguous -> OR both columns -> both rows.
	termRows, err := spreadRows(ctx, sqlDB, "", "", "honda")
	if err != nil {
		t.Fatalf("spreadRows term: %v", err)
	}
	if got := totalN(termRows); got != 2 {
		t.Errorf("positional honda matched %d listings, want 2 (both make and model)", got)
	}

	// --model honda: MODEL column only -> only the Ford/Honda row.
	modelRows, err := spreadRows(ctx, sqlDB, "", "honda", "")
	if err != nil {
		t.Fatalf("spreadRows --model: %v", err)
	}
	if got := totalN(modelRows); got != 1 {
		t.Errorf("--model honda matched %d listings, want 1 (Ford/Honda only)", got)
	}
}

// totalN sums the "n" counts across spread source rows.
func totalN(rows []map[string]any) int {
	total := 0
	for _, r := range rows {
		if n, ok := r["n"].(int); ok {
			total += n
		}
	}
	return total
}

func TestDealMakeBindsMakeColumnOnly(t *testing.T) {
	ctx := context.Background()
	sqlDB, cleanup := openTestStore(t)
	defer cleanup()
	seedMakeModelRows(t, sqlDB)

	// deal needs >=3 listings in a (make,model,year,mileageBand) bucket to score,
	// so the row count alone is the cleanest signal. Use a raw count via the same
	// filter by checking the SELECT the filter produces through dealRows' bucketing
	// is overkill; instead assert which titles survive the WHERE via a probe query.
	assertFilterMatches(t, ctx, sqlDB, "deal --make honda", "honda", "", "", map[string]bool{"hc": true})
	assertFilterMatches(t, ctx, sqlDB, "deal --model honda", "", "honda", "", map[string]bool{"fh": true})
	assertFilterMatches(t, ctx, sqlDB, "deal positional honda", "", "", "honda", map[string]bool{"hc": true, "fh": true})
}

func TestDedupeMakeBindsMakeColumnOnly(t *testing.T) {
	ctx := context.Background()
	sqlDB, cleanup := openTestStore(t)
	defer cleanup()
	seedMakeModelRows(t, sqlDB)

	// dedupe returns one row per VIN; the seeded rows have distinct VINs.
	mkRows, err := dedupeRows(ctx, sqlDB, "honda", "", "", 0)
	if err != nil {
		t.Fatalf("dedupeRows --make: %v", err)
	}
	if got := rowSet(mkRows, "make"); !(len(mkRows) == 1 && got["Honda"] && !got["Ford"]) {
		t.Errorf("--make honda returned %d rows %v, want exactly the Honda row", len(mkRows), got)
	}

	termRows, err := dedupeRows(ctx, sqlDB, "", "", "honda", 0)
	if err != nil {
		t.Fatalf("dedupeRows term: %v", err)
	}
	if len(termRows) != 2 {
		t.Errorf("positional honda returned %d rows, want 2 (Honda/Civic and Ford/Honda)", len(termRows))
	}

	modelRows, err := dedupeRows(ctx, sqlDB, "", "honda", "", 0)
	if err != nil {
		t.Fatalf("dedupeRows --model: %v", err)
	}
	if got := rowSet(modelRows, "make"); !(len(modelRows) == 1 && got["Ford"] && !got["Honda"]) {
		t.Errorf("--model honda returned %d rows %v, want exactly the Ford/Honda row", len(modelRows), got)
	}
}

// assertFilterMatches runs the shared make/model/term filter against at_listings
// and asserts the surviving listing_id set, isolating the WHERE binding from
// each command's downstream aggregation (deal's bucketing, etc.).
func assertFilterMatches(t *testing.T, ctx context.Context, sqlDB *sql.DB, label, mk, model, term string, want map[string]bool) {
	t.Helper()
	where := []string{"1=1"}
	var args []any
	where, args = appendMakeModelTermFilter(where, args, mk, model, term)
	q := "SELECT listing_id FROM at_listings WHERE " + joinAnd(where)
	rows, err := sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		t.Fatalf("%s query: %v", label, err)
	}
	defer rows.Close()
	got := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("%s scan: %v", label, err)
		}
		got[id] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s rows: %v", label, err)
	}
	if len(got) != len(want) {
		t.Errorf("%s: matched %v, want %v", label, keysOf(got), keysOf(want))
		return
	}
	for id := range want {
		if !got[id] {
			t.Errorf("%s: missing expected id %q (got %v)", label, id, keysOf(got))
		}
	}
}

func joinAnd(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " AND "
		}
		out += p
	}
	return out
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
