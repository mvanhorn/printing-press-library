// Hand-authored tests for the Lancet analytics API (serve command). Not generated.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/store"
)

// newServeTestDB opens a temp-dir SQLite store and seeds the lancet analytics
// tables with a tiny fixture, mirroring internal/lancet/lancet_test.go's
// newTestDB shape (Alice + Bob at Oxford) but seeded via SQL because the
// lancet package's decoded types are unexported. Years are chosen so the
// default affiliation-growth window (5 years back from MAX(pub_year)=2025,
// i.e. recent 2021-2025 vs prior 2016-2020) shows Oxford with recent=2,
// prior=1, growth=+1.
func newServeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	db := st.DB()
	db.SetMaxOpenConns(1)
	if err := lancet.EnsureSchema(ctx, db); err != nil {
		t.Fatalf("schema: %v", err)
	}

	works := []struct {
		id, title, issn string
		year, cited     int
	}{
		{"W1", "Cancer trial", "0140-6736", 2025, 100},
		{"W2", "Cardio study", "0140-6736", 2024, 50},
		{"W3", "Neuro study", "0140-6736", 2019, 10},
	}
	for _, w := range works {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO lancet_works (work_id, doi, title, journal_issn, journal_name, pub_year, pub_date, cited_count, is_oa, topic, synced_at)
			VALUES (?, ?, ?, ?, 'The Lancet', ?, ?, ?, 1, 'Medicine', '2026-07-04T00:00:00Z')`,
			w.id, "10.1/"+w.id, w.title, w.issn, w.year, "2024-01-01", w.cited); err != nil {
			t.Fatalf("seed work %s: %v", w.id, err)
		}
	}
	authorships := []struct {
		work, author, name string
	}{
		{"W1", "A1", "Alice"},
		{"W1", "A2", "Bob"},
		{"W2", "A1", "Alice"},
		{"W3", "A1", "Alice"},
	}
	for _, a := range authorships {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO lancet_authorships (work_id, author_id, author_name, seq) VALUES (?, ?, ?, 0)`,
			a.work, a.author, a.name); err != nil {
			t.Fatalf("seed authorship %s/%s: %v", a.work, a.author, err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO lancet_affiliations (work_id, author_id, institution_id, institution_name, country)
			VALUES (?, ?, 'I1', 'Oxford', 'GB')`,
			a.work, a.author); err != nil {
			t.Fatalf("seed affiliation %s/%s: %v", a.work, a.author, err)
		}
	}
	return db
}

func newServeTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return newServeMux(newServeTestDB(t), 5*time.Second, io.Discard)
}

func serveDo(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServeAffiliationsSuccess(t *testing.T) {
	h := newServeTestHandler(t)
	rec := serveDo(t, h, http.MethodGet, "/affiliations")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var rows []lancet.InstGrowth
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("body is not a JSON array of InstGrowth: %v (body: %s)", err, rec.Body.String())
	}
	if len(rows) != 1 {
		t.Fatalf("got %d institutions, want 1: %+v", len(rows), rows)
	}
	got := rows[0]
	if got.Institution != "Oxford" || got.RecentCount != 2 || got.PriorCount != 1 || got.Growth != 1 {
		t.Errorf("row = %+v, want Oxford recent=2 prior=1 growth=1", got)
	}
	// JSON field-name contract must match the CLI's --json output.
	var raw []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	for _, field := range []string{"institution", "recent_count", "prior_count", "growth"} {
		if _, ok := raw[0][field]; !ok {
			t.Errorf("response missing JSON field %q: %v", field, raw[0])
		}
	}
}

func TestServeAffiliationsThresholdFiltersOut(t *testing.T) {
	h := newServeTestHandler(t)
	rec := serveDo(t, h, http.MethodGet, "/affiliations?threshold=3")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	// Oxford has recent=2 < threshold 3; result must be [] (not null).
	if body := rec.Body.String(); body != "[]\n" {
		t.Errorf("body = %q, want empty JSON array \"[]\\n\"", body)
	}
}

func TestServeAuthorsSuccess(t *testing.T) {
	h := newServeTestHandler(t)
	rec := serveDo(t, h, http.MethodGet, "/authors?institution=Oxford")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var rows []lancet.AuthorRank
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("body is not a JSON array of AuthorRank: %v (body: %s)", err, rec.Body.String())
	}
	if len(rows) != 2 {
		t.Fatalf("got %d authors, want 2: %+v", len(rows), rows)
	}
	// Alice: W1(100) + W2(50) + W3(10) = 160 citations across 3 works; ranks first.
	if rows[0].AuthorName != "Alice" || rows[0].Works != 3 || rows[0].TotalCitations != 160 {
		t.Errorf("top author = %+v, want Alice works=3 total_citations=160", rows[0])
	}
	var raw []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	for _, field := range []string{"author_id", "author_name", "works", "total_citations", "avg_citations"} {
		if _, ok := raw[0][field]; !ok {
			t.Errorf("response missing JSON field %q: %v", field, raw[0])
		}
	}
}

func TestServeAuthorsLimit(t *testing.T) {
	h := newServeTestHandler(t)
	rec := serveDo(t, h, http.MethodGet, "/authors?limit=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var rows []lancet.AuthorRank
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 1 || rows[0].AuthorName != "Alice" {
		t.Errorf("rows = %+v, want just Alice", rows)
	}
}

func TestServeParamValidation400(t *testing.T) {
	h := newServeTestHandler(t)
	cases := []struct {
		name, target string
	}{
		{"affiliations non-integer years", "/affiliations?years=abc"},
		{"affiliations non-integer threshold", "/affiliations?threshold=x"},
		{"affiliations non-integer limit", "/affiliations?limit=1.5"},
		{"affiliations unknown journal", "/affiliations?journal=not-a-journal"},
		{"authors non-integer limit", "/authors?limit=abc"},
		{"authors unknown journal", "/authors?journal=not-a-journal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serveDo(t, h, http.MethodGet, tc.target)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
			}
			var body serveErrorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error == "" {
				t.Errorf("400 body must be structured JSON with non-empty error, got %q (err=%v)", rec.Body.String(), err)
			}
		})
	}
}

func TestServeDefaultsMirrorCLICoercion(t *testing.T) {
	h := newServeTestHandler(t)
	// The CLI coerces years < 1 back to 5 instead of erroring; the API must
	// mirror that, so years=0 behaves exactly like the default request.
	rec := serveDo(t, h, http.MethodGet, "/affiliations?years=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var rows []lancet.InstGrowth
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 1 || rows[0].RecentCount != 2 {
		t.Errorf("rows = %+v, want the same Oxford row as the default 5-year window", rows)
	}
}

func TestServeMethodNotAllowedAndUnknownPath(t *testing.T) {
	h := newServeTestHandler(t)
	if rec := serveDo(t, h, http.MethodPost, "/authors"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /authors status = %d, want 405", rec.Code)
	}
	if rec := serveDo(t, h, http.MethodDelete, "/affiliations"); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /affiliations status = %d, want 405", rec.Code)
	}
	if rec := serveDo(t, h, http.MethodGet, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /nope status = %d, want 404", rec.Code)
	}
}
