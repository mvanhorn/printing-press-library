// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
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
