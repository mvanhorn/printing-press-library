// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression coverage for three review findings that share one theme: a command
// reporting or persisting progress it did not actually make.
//   - find: --page applied to the raw API offset skipped client-side matches.
//   - sync: success output claimed it advanced a cursor it deliberately leaves alone.
//   - new:  a discarded mirror-write error let the saved-search cursor advance.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/store"
)

// jobCorpusServer serves a deterministic search.json corpus honoring offset and
// result_limit. Record i is "job-NN"; only even indices carry the software
// category, so a --category filter halves the stream and any offset applied
// before that filter is observable in the results. A non-nil base shifts every
// id, letting a test hand a later run a set of previously unseen jobs.
func jobCorpusServer(t *testing.T, total int, base *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("result_limit"))
		if limit <= 0 {
			limit = 20
		}
		shift := 0
		if base != nil {
			shift = int(base.Load())
		}
		jobs := make([]map[string]any, 0, limit)
		for i := offset; i < offset+limit && i < total; i++ {
			category := "Operations"
			if i%2 == 0 {
				category = "Software Development"
			}
			id := fmt.Sprintf("job-%02d", shift+i)
			// Both keys matter: the CLI diffs on id_icims while the local
			// store extracts its primary key from id.
			jobs = append(jobs, map[string]any{
				"id":           id,
				"id_icims":     id,
				"title":        fmt.Sprintf("Engineer %02d", shift+i),
				"job_category": category,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"hits": total, "jobs": jobs}); err != nil {
			t.Errorf("encoding fake search response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runCLI executes one root invocation against an isolated home so the test
// never reads or writes the developer's real config, cache, or store.
func runCLI(t *testing.T, home string, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--home", home, "--no-cache", "--no-learn", "--no-input"))
	err := cmd.Execute()
	return out.String(), err
}

// TestFindPagesTheFilteredSetNotTheRawStream pins that --page walks the filtered
// result set when client-side filters are active. The server cannot apply those
// filters, so translating --page into a raw API offset drops matching jobs that
// live in earlier server pages.
func TestFindPagesTheFilteredSetNotTheRawStream(t *testing.T) {
	srv := jobCorpusServer(t, 20, nil)
	t.Setenv("AMAZON_JOBS_BASE_URL", srv.URL)

	out, err := runCLI(t, t.TempDir(), "find", "engineer",
		"--category", "software", "--limit", "2", "--page", "2", "--json")
	if err != nil {
		t.Fatalf("find error = %v\n%s", err, out)
	}

	// Filtered set is job-00, job-02, job-04, ...; page 2 at --limit 2 is {04, 06}.
	for _, want := range []string{"job-04", "job-06"} {
		if !strings.Contains(out, want) {
			t.Fatalf("page 2 of the filtered set is missing %s:\n%s", want, out)
		}
	}
	// job-02 closes page 1. Seeing it again means the offset was applied to the
	// unfiltered stream before the category filter ran.
	for _, leaked := range []string{"job-00", "job-02"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("page 2 repeats page-1 match %s, so --page skipped raw records instead of matches:\n%s", leaked, out)
		}
	}
}

// TestFindWithoutFiltersKeepsServerPaging guards the other half of the contract:
// with no client-side filter, --page must still map straight onto the API offset.
func TestFindWithoutFiltersKeepsServerPaging(t *testing.T) {
	srv := jobCorpusServer(t, 20, nil)
	t.Setenv("AMAZON_JOBS_BASE_URL", srv.URL)

	out, err := runCLI(t, t.TempDir(), "find", "engineer", "--limit", "2", "--page", "2", "--json")
	if err != nil {
		t.Fatalf("find error = %v\n%s", err, out)
	}
	for _, want := range []string{"job-02", "job-03"} {
		if !strings.Contains(out, want) {
			t.Fatalf("server paging should return %s on page 2:\n%s", want, out)
		}
	}
	if strings.Contains(out, "job-00") {
		t.Fatalf("page 2 should not restart at the first record:\n%s", out)
	}
}

// TestSyncSavedDoesNotClaimItAdvancedTheCursor pins that sync --saved stops
// telling the user it moved the new-since cursor. sync mirrors records only; the
// cursor belongs to new, and claiming otherwise made the next new run look like
// it was re-reporting already-baselined jobs.
func TestSyncSavedDoesNotClaimItAdvancedTheCursor(t *testing.T) {
	srv := jobCorpusServer(t, 2, nil)
	t.Setenv("AMAZON_JOBS_BASE_URL", srv.URL)
	home := t.TempDir()
	dbPath := filepath.Join(home, "store.db")

	if out, err := runCLI(t, home, "save", "watch", "engineer", "--db", dbPath); err != nil {
		t.Fatalf("save error = %v\n%s", err, out)
	}
	out, err := runCLI(t, home, "sync", "--saved", "watch", "--db", dbPath, "--plain")
	if err != nil {
		t.Fatalf("sync error = %v\n%s", err, out)
	}
	if strings.Contains(out, "updated new-since cursor") {
		t.Fatalf("sync claims it advanced the cursor it deliberately leaves alone:\n%s", out)
	}
	if !strings.Contains(out, "unchanged") {
		t.Fatalf("sync should state the cursor is unchanged:\n%s", out)
	}
}

// TestNewKeepsCursorWhenMirrorWriteFails pins that a failed mirror write aborts
// new before the cursor moves. Advancing it anyway left stats/skills reading an
// incomplete mirror while the unwritten jobs never surfaced as new again.
func TestNewKeepsCursorWhenMirrorWriteFails(t *testing.T) {
	var base atomic.Int64
	srv := jobCorpusServer(t, 4, &base)
	t.Setenv("AMAZON_JOBS_BASE_URL", srv.URL)
	home := t.TempDir()
	dbPath := filepath.Join(home, "store.db")

	if out, err := runCLI(t, home, "save", "watch", "engineer", "--db", dbPath); err != nil {
		t.Fatalf("save error = %v\n%s", err, out)
	}
	if out, err := runCLI(t, home, "new", "watch", "--db", dbPath, "--json"); err != nil {
		t.Fatalf("baseline new error = %v\n%s", err, out)
	}
	before := savedSearchCursor(t, dbPath, "watch")

	// Hand the next run four ids it has never seen, so the mirror write is a
	// fresh insert rather than a conflict update, then block writes to the
	// mirror. saved_searches is untouched, so the cursor UPDATE would still
	// succeed -- which is exactly how the old code lost these jobs: it
	// discarded the mirror error and advanced the cursor past them.
	base.Store(100)
	blockMirrorWrites(t, dbPath)

	out, err := runCLI(t, home, "new", "watch", "--db", dbPath, "--json")
	if err == nil {
		t.Fatalf("new reported success despite a failed mirror write:\n%s", out)
	}
	if after := savedSearchCursor(t, dbPath, "watch"); after != before {
		t.Fatalf("cursor advanced after a failed mirror write:\nbefore = %s\nafter  = %s", before, after)
	}
}

// savedSearchCursor reads the persisted new-since cursor for a saved search.
func savedSearchCursor(t *testing.T, dbPath, name string) string {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()
	var lastSeen string
	if err := db.DB().QueryRow(`SELECT last_seen FROM saved_searches WHERE name = ?`, name).Scan(&lastSeen); err != nil {
		t.Fatalf("reading saved-search cursor: %v", err)
	}
	return lastSeen
}

// blockMirrorWrites installs a trigger that aborts every insert into the
// generic resource mirror, simulating a mirror-write failure without touching
// saved_searches. The typed postings projection is deliberately non-fatal in
// the store, so the generic table is the layer that surfaces a batch error.
func blockMirrorWrites(t *testing.T, dbPath string) {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()
	if _, err := db.DB().Exec(
		`CREATE TRIGGER block_resources BEFORE INSERT ON "resources" ` +
			`BEGIN SELECT RAISE(ABORT, 'mirror write blocked'); END`); err != nil {
		t.Fatalf("installing mirror write block: %v", err)
	}
}

// TestNewReportsAndPreservesACurtailedScan pins the two halves of the
// page-cap contract. A run stopped by --max-pages must (a) say so instead of
// presenting its partial count as the whole result set, and (b) keep the ids it
// never reached in the cursor, so they are not re-reported as new later.
func TestNewReportsAndPreservesACurtailedScan(t *testing.T) {
	// 150 records: one full 100-record page plus a short second page.
	srv := jobCorpusServer(t, 150, nil)
	t.Setenv("AMAZON_JOBS_BASE_URL", srv.URL)
	home := t.TempDir()
	dbPath := filepath.Join(home, "store.db")

	if out, err := runCLI(t, home, "save", "watch", "engineer", "--db", dbPath); err != nil {
		t.Fatalf("save error = %v\n%s", err, out)
	}
	// Baseline over the complete set, so every id is in the cursor.
	if out, err := runCLI(t, home, "new", "watch", "--db", dbPath, "--max-pages", "5", "--json"); err != nil {
		t.Fatalf("baseline new error = %v\n%s", err, out)
	}
	if before := savedSearchCursor(t, dbPath, "watch"); !strings.Contains(before, "job-149") {
		t.Fatalf("baseline cursor should span the whole corpus, missing job-149:\n%s", before)
	}

	// One page only: reaches job-00..99 and never sees the tail.
	out, err := runCLI(t, home, "new", "watch", "--db", dbPath, "--max-pages", "1", "--json")
	if err != nil {
		t.Fatalf("curtailed new error = %v\n%s", err, out)
	}
	var got struct {
		TotalNow  int    `json:"total_now"`
		TotalHits int    `json:"total_hits"`
		Curtailed bool   `json:"curtailed"`
		Note      string `json:"note"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decoding new output: %v\n%s", err, out)
	}
	if !got.Curtailed {
		t.Fatalf("a page-capped scan must report itself as curtailed:\n%s", out)
	}
	if got.TotalHits != 150 || got.TotalNow != 100 {
		t.Fatalf("partial scan must show 100 scanned of 150 upstream, got %d of %d:\n%s",
			got.TotalNow, got.TotalHits, out)
	}
	if !strings.Contains(got.Note, "max-pages") {
		t.Fatalf("the note must tell the user how to widen the scan, got %q", got.Note)
	}

	// The tail was never scanned, so it must remain tracked. Dropping it would
	// resurface those reqs as "new" the next time they land on an earlier page.
	after := savedSearchCursor(t, dbPath, "watch")
	if !strings.Contains(after, "job-149") {
		t.Fatalf("curtailed scan walked the cursor backwards and dropped unreached ids:\n%s", after)
	}
}
