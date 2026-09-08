// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"testing"

	_ "modernc.org/sqlite"
)

// newCoverageDB builds just the tables these regressions touch.
//
// store.EnsurePBSSchema guards its migrations with a package-level sync.Once,
// so it applies the schema to whichever *sql.DB is passed first in a process
// and is a no-op afterwards. That is fine for the CLI, which opens one panel
// per process, but it means a test that wants a fresh database has to issue
// the DDL itself.
func newCoverageDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, stmt := range []string{
		`CREATE TABLE pbs_coverage (
			as_of TEXT NOT NULL, kind TEXT NOT NULL, role TEXT NOT NULL,
			url TEXT, state TEXT NOT NULL,
			PRIMARY KEY (as_of, kind, role))`,
		`CREATE TABLE pbs_price (
			as_of TEXT NOT NULL, surface TEXT NOT NULL, city TEXT NOT NULL,
			item_desc TEXT NOT NULL, stat TEXT NOT NULL, value REAL,
			value_state TEXT NOT NULL, source TEXT NOT NULL,
			PRIMARY KEY (as_of, surface, city, item_desc, stat))`,
		`CREATE TABLE pbs_national (as_of TEXT NOT NULL, item_desc TEXT NOT NULL)`,
		`CREATE TABLE pbs_release_file (
			as_of TEXT NOT NULL, kind TEXT NOT NULL, role TEXT NOT NULL, url TEXT)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	return db
}

// putIndexFile records that the scraped index lists a file of this role, which
// is what tells resume whether a report row is owed at all.
func putIndexFile(t *testing.T, db *sql.DB, asOf, role string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO pbs_release_file (as_of, kind, role, url) VALUES (?,?,?,?)`,
		asOf, "spi", role, "https://example.invalid/"+asOf+"-"+role); err != nil {
		t.Fatalf("insert release file: %v", err)
	}
}

func putCoverage(t *testing.T, db *sql.DB, asOf, role, state string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO pbs_coverage (as_of, kind, role, state) VALUES (?,?,?,?)`,
		asOf, "spi", role, state); err != nil {
		t.Fatalf("insert coverage: %v", err)
	}
}

// TestLoadCompletedRequiresTerminalReport pins the resume contract.
//
// A release is TWO files: the annexure carries the city x item price grid, the
// report carries the weight vector, weight totals and quintile indices. Resume
// previously keyed on the annexure row alone, so a release whose annexure
// committed and whose report then failed was skipped by every later ordinary
// sync — it kept its prices and lost its weights permanently, reachable only
// via --refetch.
func TestLoadCompletedRequiresTerminalReport(t *testing.T) {
	db := newCoverageDB(t)

	// Every release below lists BOTH files in the scraped index, except the
	// no-report-file case added at the end.
	for _, asOf := range []string{"2026-09-03", "2026-08-27", "2026-08-20", "2026-08-13", "2026-08-06", "2026-07-30"} {
		putIndexFile(t, db, asOf, "annexure")
		putIndexFile(t, db, asOf, "report")
	}

	// Complete: both files terminal.
	putCoverage(t, db, "2026-09-03", "annexure", covFetched)
	putCoverage(t, db, "2026-09-03", "report", covFetched)

	// Report gone upstream. Terminal — re-running cannot bring it back.
	putCoverage(t, db, "2026-08-27", "annexure", covFetched)
	putCoverage(t, db, "2026-08-27", "report", covRot)

	// Report present but unparseable (the PDF-only case). Terminal: re-fetching
	// the same bytes cannot make them parse.
	putCoverage(t, db, "2026-08-20", "annexure", covFetched)
	putCoverage(t, db, "2026-08-20", "report", covUnparsed)

	// Report failed in transport. NOT terminal — must be retried.
	putCoverage(t, db, "2026-08-13", "annexure", covFetched)
	putCoverage(t, db, "2026-08-13", "report", covError)

	// No report row at all — the shape a run leaves when it dies between the
	// two files. Must be retried.
	putCoverage(t, db, "2026-08-06", "annexure", covFetched)

	// Annexure itself incomplete. Must be retried regardless of the report.
	putCoverage(t, db, "2026-07-30", "annexure", covEmpty)
	putCoverage(t, db, "2026-07-30", "report", covFetched)

	// Report present upstream but in a format no parser accepts, so it was
	// deliberately never fetched. Terminal: re-running cannot help.
	putIndexFile(t, db, "2026-07-23", "annexure")
	putIndexFile(t, db, "2026-07-23", "report")
	putCoverage(t, db, "2026-07-23", "annexure", covFetched)
	putCoverage(t, db, "2026-07-23", "report", covNotFetched)

	// The index lists NO report file for this release — the shape of every CPI
	// month. No report row is owed, so a fetched annexure alone completes it.
	// Requiring a placeholder row here would mean recording an observation
	// about a file that does not exist.
	putIndexFile(t, db, "2026-07-16", "annexure")
	putCoverage(t, db, "2026-07-16", "annexure", covFetched)

	got, err := loadCompleted(context.Background(), db)
	if err != nil {
		t.Fatalf("loadCompleted: %v", err)
	}

	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	want := []string{"2026-07-16|spi", "2026-07-23|spi", "2026-08-20|spi", "2026-08-27|spi", "2026-09-03|spi"}
	if len(keys) != len(want) {
		t.Fatalf("completed = %v, want %v", keys, want)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("completed = %v, want %v", keys, want)
		}
	}

	for _, mustRetry := range []string{"2026-08-13|spi", "2026-08-06|spi", "2026-07-30|spi"} {
		if got[mustRetry] {
			t.Errorf("%s must stay eligible for another attempt", mustRetry)
		}
	}
}

// TestDeleteReleaseRowsDropsStaleObservations pins replace-semantics. PBS
// rewrites releases, and the item basket is not constant, so a rewrite that
// drops an item must not leave that item alive at the same as_of to be summed
// into a later basket or national average.
func TestDeleteReleaseRowsDropsStaleObservations(t *testing.T) {
	db := newCoverageDB(t)

	ins := func(asOf, item string) {
		t.Helper()
		if _, err := db.Exec(
			`INSERT INTO pbs_price (as_of, surface, city, item_desc, stat, value, value_state, source)
			 VALUES (?,?,?,?,?,?,?,?)`,
			asOf, "urban", "Lahore", item, "avg", 100.0, "present", "xlsx"); err != nil {
			t.Fatalf("insert price: %v", err)
		}
	}
	// Only the weekly series publishes on these dates, so the delete is
	// unambiguous and must run.
	putIndexFile(t, db, "2026-09-03", "annexure")
	putIndexFile(t, db, "2026-08-27", "annexure")

	// The release being rewritten, plus a neighbour that must survive.
	ins("2026-09-03", "Wheat Flour")
	ins("2026-09-03", "Gas Charges for Q1")
	ins("2026-08-27", "Wheat Flour")

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := deleteReleaseRows(context.Background(), tx, "2026-09-03", "spi", "pbs_price", "pbs_national"); err != nil {
		t.Fatalf("deleteReleaseRows: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pbs_price WHERE as_of = '2026-09-03'`).Scan(&n); err != nil {
		t.Fatalf("count rewritten: %v", err)
	}
	if n != 0 {
		t.Errorf("rewritten release still holds %d stale rows", n)
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM pbs_price WHERE as_of = '2026-08-27'`).Scan(&n); err != nil {
		t.Fatalf("count neighbour: %v", err)
	}
	if n != 1 {
		t.Errorf("neighbouring release rows = %d, want 1 — the delete must be scoped to one as_of", n)
	}
}

// TestDeleteReleaseRowsDeclinesOnSeriesCollision pins the case that makes an
// as_of-scoped delete unsafe.
//
// The observation tables key on as_of and carry no `kind`, but PBS publishes a
// weekly AND a monthly release on the same date three times in the live index
// (2024-02-01, 2024-08-01, 2026-01-01). Deleting by as_of alone on such a date
// would destroy whichever series is not being synced, so the delete must
// decline and leave the upsert to do the work.
func TestDeleteReleaseRowsDeclinesOnSeriesCollision(t *testing.T) {
	db := newCoverageDB(t)

	if _, err := db.Exec(
		`INSERT INTO pbs_price (as_of, surface, city, item_desc, stat, value, value_state, source)
		 VALUES (?,?,?,?,?,?,?,?)`,
		"2026-01-01", "appendix-a", "Lahore", "Wheat Flour", "avg", 100.0, "present", "pdf"); err != nil {
		t.Fatalf("insert price: %v", err)
	}

	// Both series publish on this date, exactly as the live index does.
	putIndexFile(t, db, "2026-01-01", "annexure")
	if _, err := db.Exec(
		`INSERT INTO pbs_release_file (as_of, kind, role, url) VALUES (?,?,?,?)`,
		"2026-01-01", "cpi", "annexure", "https://example.invalid/cpi"); err != nil {
		t.Fatalf("insert cpi index row: %v", err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := deleteReleaseRows(context.Background(), tx, "2026-01-01", "spi", "pbs_price", "pbs_national"); err != nil {
		t.Fatalf("deleteReleaseRows: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pbs_price WHERE as_of = '2026-01-01'`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("rows at a colliding date = %d, want 1 — the other series must not be deleted", n)
	}
}
