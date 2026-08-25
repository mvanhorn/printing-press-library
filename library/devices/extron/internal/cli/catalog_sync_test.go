// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/store"
)

// letterIndexHTML is a minimal literature index page: one category, one row.
const letterIndexHTML = `<html><body>
<h2>Brochure (X - 1 files)</h2>
<table>
<tr><th>Description</th><th>Rev</th><th>Date</th><th>Size</th><th>Type</th></tr>
<tr>
<td><a id="ctl00_1_idFileUrl" href="/download/files/brochure/%s.pdf" target="download">%s Brochure</a></td>
<td><nobr>A</nobr></td><td><nobr>Jan. 2, 2024</nobr></td><td><nobr>10 KB</nobr></td><td><nobr>PDF</nobr></td>
</tr>
</table>
</body></html>`

// catalogTestServer serves a literature index per letter. Letters named in
// failLetters always return HTTP 500; every other letter returns one document.
func catalogTestServer(t *testing.T, failLetters ...string) (*httptest.Server, func(string) int) {
	t.Helper()
	fail := make(map[string]bool, len(failLetters))
	for _, l := range failLetters {
		fail[l] = true
	}
	var mu sync.Mutex
	hits := make(map[string]int)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		letter := r.URL.Query().Get("id")
		mu.Lock()
		hits[letter]++
		mu.Unlock()
		if fail[letter] {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		body := strings.ReplaceAll(letterIndexHTML, "%s", letter)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return srv, func(letter string) int {
		mu.Lock()
		defer mu.Unlock()
		return hits[letter]
	}
}

// useCatalogTestServer points the catalog crawl at srv for the duration of a test.
func useCatalogTestServer(t *testing.T, srv *httptest.Server) {
	t.Helper()
	prev := newCatalogClient
	newCatalogClient = func() *extron.Client {
		c := extron.New()
		c.BaseURL = srv.URL
		return c
	}
	t.Cleanup(func() { newCatalogClient = prev })
}

// runCatalogSync executes `catalog sync` with the given args and returns the
// decoded JSON summary alongside the command error.
func runCatalogSync(t *testing.T, dbPath string, args ...string) (map[string]any, string, error) {
	t.Helper()
	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	full := append([]string{"catalog", "sync", "--json", "--no-color", "--db", dbPath}, args...)
	cmd.SetArgs(full)
	runErr := cmd.Execute()

	var summary map[string]any
	if body := strings.TrimSpace(out.String()); body != "" {
		if err := json.Unmarshal([]byte(body), &summary); err != nil {
			t.Fatalf("decoding summary %q: %v", body, err)
		}
	}
	return summary, errOut.String(), runErr
}

// TestCatalogSyncContinuesPastFailedLetter is the regression guard for the
// reported failure: one bad letter bucket used to abort the whole 0-9,A-Z
// crawl, so a transient error on letter A discarded the other 35 buckets and
// left the catalog empty. The crawl must now skip the bad bucket, keep the
// good ones, and still exit 0.
func TestCatalogSyncContinuesPastFailedLetter(t *testing.T) {
	srv, _ := catalogTestServer(t, "A")
	useCatalogTestServer(t, srv)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	summary, _, err := runCatalogSync(t, dbPath, "--letters", "A,B,C", "--retries", "0")
	if err != nil {
		t.Fatalf("catalog sync returned error, want nil (one bad letter must not abort the crawl): %v", err)
	}

	if got := summary["letters_fetched"]; got != float64(2) {
		t.Errorf("letters_fetched = %v, want 2 (B and C)", got)
	}
	if got := summary["letters_failed"]; got != float64(1) {
		t.Errorf("letters_failed = %v, want 1 (A)", got)
	}
	if got := summary["docs"]; got != float64(2) {
		t.Errorf("docs = %v, want 2", got)
	}

	errs, ok := summary["errors"].([]any)
	if !ok || len(errs) != 1 {
		t.Fatalf("errors = %v, want one entry", summary["errors"])
	}
	if letter := errs[0].(map[string]any)["letter"]; letter != "A" {
		t.Errorf("errors[0].letter = %v, want A", letter)
	}

	// The partial catalog must be recorded, otherwise every local-read command
	// reports the store as never synced.
	db, err := store.OpenWithContext(t.Context(), dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()
	cursor, _, count, err := db.GetSyncState(catalogResource)
	if err != nil {
		t.Fatalf("reading sync state: %v", err)
	}
	if cursor != "partial" {
		t.Errorf("sync cursor = %q, want %q", cursor, "partial")
	}
	if count != 2 {
		t.Errorf("sync state count = %d, want 2", count)
	}
}

// TestCatalogSyncRetriesBeforeSkipping proves --retries actually re-attempts a
// failing bucket rather than skipping it on the first error.
func TestCatalogSyncRetriesBeforeSkipping(t *testing.T) {
	srv, hits := catalogTestServer(t, "A")
	useCatalogTestServer(t, srv)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	if _, _, err := runCatalogSync(t, dbPath, "--letters", "A,B", "--retries", "1"); err != nil {
		t.Fatalf("catalog sync returned error, want nil: %v", err)
	}

	// One initial attempt plus one retry. The client itself retries WAF resets,
	// not HTTP 500s, so the count is exactly the letter-level attempts.
	if got := hits("A"); got != 2 {
		t.Errorf("letter A request count = %d, want 2 (initial + 1 retry)", got)
	}
}

// TestCatalogSyncStrictFailsOnSkippedLetter keeps an opt-in path to the old
// abort-on-any-error behavior for callers that need it.
func TestCatalogSyncStrictFailsOnSkippedLetter(t *testing.T) {
	srv, _ := catalogTestServer(t, "A")
	useCatalogTestServer(t, srv)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	_, _, err := runCatalogSync(t, dbPath, "--letters", "A,B", "--retries", "0", "--strict")
	if err == nil {
		t.Fatal("catalog sync --strict returned nil, want an error when a letter was skipped")
	}
	if !strings.Contains(err.Error(), "A") {
		t.Errorf("error %q does not name the skipped letter", err)
	}
}

// TestCatalogSyncFailsWhenEveryLetterFails guards the other direction: a crawl
// that stored nothing must not report success.
func TestCatalogSyncFailsWhenEveryLetterFails(t *testing.T) {
	srv, _ := catalogTestServer(t, "A", "B")
	useCatalogTestServer(t, srv)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	_, _, err := runCatalogSync(t, dbPath, "--letters", "A,B", "--retries", "0")
	if err == nil {
		t.Fatal("catalog sync returned nil, want an error when every letter failed")
	}
}

// TestCatalogSyncRunsWithoutFlags guards the documented entry point: SKILL.md,
// README Quick Start, and the command's own Example all say `catalog sync`
// with no flags is how the catalog gets built. A no-flag guard used to make it
// print help and exit 0 without fetching anything, so following the docs
// literally produced an empty catalog.
func TestCatalogSyncRunsWithoutFlags(t *testing.T) {
	// Zero flags, so the DB path has to come from the sandboxed home rather
	// than --db; passing any flag at all would have satisfied the old guard.
	testenv.Isolate(t, cliutil.DataDir)
	t.Setenv(cliutil.DogfoodEnvVar, "1") // one letter bucket keeps the test quick

	srv, hits := catalogTestServer(t)
	useCatalogTestServer(t, srv)

	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"catalog", "sync"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("catalog sync (no flags) returned error: %v", err)
	}

	if hits("0") == 0 {
		t.Fatal("catalog sync with no flags fetched nothing — it printed help instead of syncing")
	}
	if strings.Contains(out.String(), "Usage:") {
		t.Errorf("catalog sync with no flags printed help:\n%s", out.String())
	}
}

// docRow renders one literature table row for the fixture pages.
func docRow(id string) string {
	return fmt.Sprintf(
		`<tr><td><a id="ctl00_1_idFileUrl" href="/download/files/brochure/%s.pdf" target="download">%s Brochure</a></td>`+
			`<td><nobr>A</nobr></td><td><nobr>Jan. 2, 2024</nobr></td><td><nobr>10 KB</nobr></td><td><nobr>PDF</nobr></td></tr>`,
		id, id)
}

// paginatedPage renders a literature page holding the given docs, optionally
// followed by the per-category "Next" link that drives --full pagination.
func paginatedPage(heading string, ids []string, hasNext bool) string {
	var b strings.Builder
	b.WriteString("<html><body>")
	if heading != "" {
		b.WriteString("<h2>" + heading + "</h2>")
	}
	b.WriteString("<table>")
	for _, id := range ids {
		b.WriteString(docRow(id))
	}
	b.WriteString("</table>")
	if hasNext {
		b.WriteString(`<a class="link-next" href="/technology/literature.aspx?filetype=1&amp;tabid=5&amp;page=2">Next</a>`)
	}
	b.WriteString("</body></html>")
	return b.String()
}

// TestCatalogSyncCountsDocsCommittedAcrossRetries is the regression guard for
// the retry-accounting defect: upserts commit as they go, so when one attempt
// stores documents and then fails partway through pagination, a later attempt's
// documents add to the store rather than replacing them. Counting the largest
// single attempt loses whatever only an earlier attempt reached, which makes
// the summary, the sync-state count, and doctor underreport the rows on disk.
//
// The fixture makes the two attempts commit overlapping-but-different sets:
//
//	attempt 1: index a1, then category page 2 -> b1,b2, then page 3 fails  (3 docs)
//	attempt 2: index a1, then category page 2 -> c1,c2, then page 3 -> c3  (4 docs)
//
// Six distinct documents reach the store. Taking the largest attempt reports 4.
func TestCatalogSyncCountsDocsCommittedAcrossRetries(t *testing.T) {
	const wantDocs = 6 // a1, b1, b2, c1, c2, c3

	var mu sync.Mutex
	indexHits := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		w.Header().Set("Content-Type", "text/html")

		// The index page carries no filetype; category pages always do.
		if q.Get("filetype") == "" {
			mu.Lock()
			indexHits++
			mu.Unlock()
			_, _ = w.Write([]byte(paginatedPage("Brochure (A - 1 files)", []string{"a1"}, true)))
			return
		}

		mu.Lock()
		attempt := indexHits
		mu.Unlock()

		switch {
		case attempt == 1 && q.Get("page") == "2":
			_, _ = w.Write([]byte(paginatedPage("", []string{"b1", "b2"}, true)))
		case attempt == 1: // page 3 — the failure that ends attempt 1
			w.WriteHeader(http.StatusInternalServerError)
		case q.Get("page") == "2":
			_, _ = w.Write([]byte(paginatedPage("", []string{"c1", "c2"}, true)))
		default: // page 3 — last page, no Next link
			_, _ = w.Write([]byte(paginatedPage("", []string{"c3"}, false)))
		}
	}))
	t.Cleanup(srv.Close)
	useCatalogTestServer(t, srv)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	summary, _, err := runCatalogSync(t, dbPath, "--letters", "A", "--full", "--retries", "1")
	if err != nil {
		t.Fatalf("catalog sync returned error, want nil: %v", err)
	}

	db, err := store.OpenWithContext(t.Context(), dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()

	rows, err := db.Count(catalogResource)
	if err != nil {
		t.Fatalf("counting catalog rows: %v", err)
	}
	if rows != wantDocs {
		t.Fatalf("catalog rows = %d, want %d — fixture did not commit the expected documents", rows, wantDocs)
	}

	if got := summary["docs"]; got != float64(wantDocs) {
		t.Errorf("summary docs = %v, want %d (rows actually committed across both attempts)", got, wantDocs)
	}
	perLetter, ok := summary["per_letter"].(map[string]any)
	if !ok {
		t.Fatalf("per_letter = %v, want an object", summary["per_letter"])
	}
	if got := perLetter["A"]; got != float64(wantDocs) {
		t.Errorf("per_letter.A = %v, want %d", got, wantDocs)
	}

	_, _, count, err := db.GetSyncState(catalogResource)
	if err != nil {
		t.Fatalf("reading sync state: %v", err)
	}
	if count != wantDocs {
		t.Errorf("sync state count = %d, want %d — doctor and the sync hint read this value", count, wantDocs)
	}
}

// TestCatalogSyncCountsRepeatedDocOnce is the other half of the accounting
// contract: a retry that re-commits the same documents must not inflate the
// count, which is what summing per-attempt totals would do.
func TestCatalogSyncCountsRepeatedDocOnce(t *testing.T) {
	var mu sync.Mutex
	indexHits := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		w.Header().Set("Content-Type", "text/html")
		if q.Get("filetype") == "" {
			mu.Lock()
			indexHits++
			mu.Unlock()
			_, _ = w.Write([]byte(paginatedPage("Brochure (A - 2 files)", []string{"a1", "a2"}, true)))
			return
		}
		mu.Lock()
		attempt := indexHits
		mu.Unlock()
		if attempt == 1 {
			// Attempt 1 fails at the first category page, after the index
			// documents have already been committed.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Attempt 2 re-serves the same two documents and finishes.
		_, _ = w.Write([]byte(paginatedPage("", []string{"a1", "a2"}, false)))
	}))
	t.Cleanup(srv.Close)
	useCatalogTestServer(t, srv)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	summary, _, err := runCatalogSync(t, dbPath, "--letters", "A", "--full", "--retries", "1")
	if err != nil {
		t.Fatalf("catalog sync returned error, want nil: %v", err)
	}
	if got := summary["docs"]; got != float64(2) {
		t.Errorf("summary docs = %v, want 2 (the same document committed twice counts once)", got)
	}
}

// TestCatalogSyncBareStoresOnlyFirstPage pins the behavior the first-run
// documentation now describes: bare `catalog sync` stores only the first index
// page per letter bucket, so a category with more pages is truncated, and
// --full is what produces the complete catalog. Live numbers behind the docs:
// bare sync yields ~1,200 documents with buckets capped at the page-1 ceiling,
// while --full yields ~3,600 and up.
//
// If the default ever changes to paginate, this test fails and the "not the
// complete catalog" wording in SKILL.md and README.md has to change with it.
func TestCatalogSyncBareStoresOnlyFirstPage(t *testing.T) {
	// Page 1 carries two documents and a Next link; page 2 carries two more.
	newServer := func() *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			if r.URL.Query().Get("filetype") == "" {
				_, _ = w.Write([]byte(paginatedPage("Brochure (A - 2 files)", []string{"p1a", "p1b"}, true)))
				return
			}
			_, _ = w.Write([]byte(paginatedPage("", []string{"p2a", "p2b"}, false)))
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	countRows := func(t *testing.T, dbPath string) int {
		t.Helper()
		db, err := store.OpenWithContext(t.Context(), dbPath)
		if err != nil {
			t.Fatalf("opening store: %v", err)
		}
		defer db.Close()
		n, err := db.Count(catalogResource)
		if err != nil {
			t.Fatalf("counting catalog rows: %v", err)
		}
		return n
	}

	t.Run("bare sync truncates at page 1", func(t *testing.T) {
		useCatalogTestServer(t, newServer())
		dbPath := filepath.Join(t.TempDir(), "data.db")
		summary, _, err := runCatalogSync(t, dbPath, "--letters", "A")
		if err != nil {
			t.Fatalf("catalog sync returned error: %v", err)
		}
		if got := summary["docs"]; got != float64(2) {
			t.Errorf("bare sync docs = %v, want 2 (page 1 only)", got)
		}
		if rows := countRows(t, dbPath); rows != 2 {
			t.Errorf("bare sync stored %d rows, want 2 — page 2 must not be fetched without --full", rows)
		}
	})

	t.Run("--full follows pagination", func(t *testing.T) {
		useCatalogTestServer(t, newServer())
		dbPath := filepath.Join(t.TempDir(), "data.db")
		summary, _, err := runCatalogSync(t, dbPath, "--letters", "A", "--full")
		if err != nil {
			t.Fatalf("catalog sync --full returned error: %v", err)
		}
		if got := summary["docs"]; got != float64(4) {
			t.Errorf("--full docs = %v, want 4 (both pages)", got)
		}
		if rows := countRows(t, dbPath); rows != 4 {
			t.Errorf("--full stored %d rows, want 4", rows)
		}
	})
}

// readSyncState returns the catalog cursor and recorded total from a store.
func readSyncState(t *testing.T, dbPath string) (string, int, int) {
	t.Helper()
	db, err := store.OpenWithContext(t.Context(), dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()
	cursor, _, count, err := db.GetSyncState(catalogResource)
	if err != nil {
		t.Fatalf("reading sync state: %v", err)
	}
	rows, err := db.Count(catalogResource)
	if err != nil {
		t.Fatalf("counting catalog rows: %v", err)
	}
	return cursor, count, rows
}

// TestCatalogSyncNarrowRerunKeepsStoreWideState is the regression guard for
// narrowed runs overwriting store-wide state. sync_state describes the whole
// catalog, but `catalog sync --letters A,Q` only knows the buckets it fetched.
// Writing that run's own tally made doctor read a handful of documents against
// a store holding thousands — observed live as total_count=31 against 4,968
// actual rows, with doctor still reporting the cache fresh.
//
// This matters because retry-then-skip makes "rerun just the failed buckets
// with --letters" the documented recovery path, so the guidance leads users
// straight into it.
func TestCatalogSyncNarrowRerunKeepsStoreWideState(t *testing.T) {
	t.Run("narrow rerun keeps the store-wide total", func(t *testing.T) {
		srv, _ := catalogTestServer(t)
		useCatalogTestServer(t, srv)
		dbPath := filepath.Join(t.TempDir(), "data.db")

		// Broad pass: three buckets, one document each.
		if _, _, err := runCatalogSync(t, dbPath, "--letters", "A,B,C"); err != nil {
			t.Fatalf("broad sync returned error: %v", err)
		}
		if _, count, rows := readSyncState(t, dbPath); count != 3 || rows != 3 {
			t.Fatalf("after broad sync: sync_state=%d rows=%d, want 3 and 3", count, rows)
		}

		// Narrow rerun of a single bucket — the documented recovery path.
		if _, _, err := runCatalogSync(t, dbPath, "--letters", "A"); err != nil {
			t.Fatalf("narrow rerun returned error: %v", err)
		}
		_, count, rows := readSyncState(t, dbPath)
		if rows != 3 {
			t.Fatalf("catalog rows = %d, want 3 — the rerun must not drop other buckets' rows", rows)
		}
		if count != rows {
			t.Errorf("sync_state total = %d, want %d (the store's actual row count) — doctor reads this value", count, rows)
		}
	})

	t.Run("narrow rerun does not downgrade a complete catalog", func(t *testing.T) {
		srv, _ := catalogTestServer(t)
		useCatalogTestServer(t, srv)
		dbPath := filepath.Join(t.TempDir(), "data.db")

		if _, _, err := runCatalogSync(t, dbPath, "--letters", "A,B,C"); err != nil {
			t.Fatalf("seed sync returned error: %v", err)
		}
		// Stand in for a prior complete crawl.
		func() {
			db, err := store.OpenWithContext(t.Context(), dbPath)
			if err != nil {
				t.Fatalf("opening store: %v", err)
			}
			defer db.Close()
			if err := db.SaveSyncState(catalogResource, "full", 3); err != nil {
				t.Fatalf("seeding full cursor: %v", err)
			}
		}()

		if _, _, err := runCatalogSync(t, dbPath, "--letters", "A"); err != nil {
			t.Fatalf("narrow rerun returned error: %v", err)
		}
		if cursor, _, _ := readSyncState(t, dbPath); cursor != "full" {
			t.Errorf("cursor = %q after a narrow rerun, want %q — a --letters run knows nothing about the buckets it skipped, so it must not downgrade the catalog", cursor, "full")
		}
	})

	t.Run("narrow run never claims the catalog is complete", func(t *testing.T) {
		srv, _ := catalogTestServer(t)
		useCatalogTestServer(t, srv)
		dbPath := filepath.Join(t.TempDir(), "data.db")

		// --full on a fresh store, but only one bucket: cannot be "full".
		if _, _, err := runCatalogSync(t, dbPath, "--letters", "A", "--full"); err != nil {
			t.Fatalf("narrow --full returned error: %v", err)
		}
		if cursor, _, _ := readSyncState(t, dbPath); cursor == "full" {
			t.Errorf("cursor = %q, want %q — one bucket is not the whole catalog, and \"full\" silences the partial-catalog hint", cursor, "partial")
		}
	})
}
