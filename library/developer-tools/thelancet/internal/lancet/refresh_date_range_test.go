// Hand-authored coverage for the refresh date-range bounds. Not generated.

package lancet

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"
)

// recordingFetcher captures the query params of every OpenAlex call and
// returns an empty page so Refresh stops after the first request.
type recordingFetcher struct {
	params []map[string]string
}

func (r *recordingFetcher) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	copied := make(map[string]string, len(params))
	for k, v := range params {
		copied[k] = v
	}
	r.params = append(r.params, copied)
	return json.RawMessage(`{"results":[],"meta":{"next_cursor":""}}`), nil
}

// refreshFilterFor runs Refresh against a throwaway store and returns the
// OpenAlex filter string it built for the single journal it was given.
func refreshFilterFor(t *testing.T, fromYear, toYear int) string {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	f := &recordingFetcher{}
	journ := []Journal{{Slug: "lancet", ISSN: "0140-6736", Display: "The Lancet"}}
	if _, err := Refresh(context.Background(), f, db, journ, fromYear, toYear, 1, nil); err != nil {
		t.Fatalf("Refresh(from=%d, to=%d): %v", fromYear, toYear, err)
	}
	if len(f.params) != 1 {
		t.Fatalf("expected 1 OpenAlex call, got %d", len(f.params))
	}
	return f.params[0]["filter"]
}

func TestRefreshFilterYearBounds(t *testing.T) {
	const base = "primary_location.source.issn:0140-6736"
	cases := []struct {
		name     string
		fromYear int
		toYear   int
		want     string
	}{
		// Regression guard: with no bounds the filter must be exactly what it
		// was before --from-year / --to-year existed.
		{"no bounds", 0, 0, base},
		{"lower bound only", 1990, 0, base + ",from_publication_date:1990-01-01"},
		{"upper bound only", 0, 2000, base + ",to_publication_date:2000-12-31"},
		{"both bounds", 1990, 2000, base + ",from_publication_date:1990-01-01,to_publication_date:2000-12-31"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := refreshFilterFor(t, tc.fromYear, tc.toYear); got != tc.want {
				t.Errorf("filter = %q, want %q", got, tc.want)
			}
		})
	}
}
