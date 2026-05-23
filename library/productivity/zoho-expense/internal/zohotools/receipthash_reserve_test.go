package zohotools

import (
	"database/sql"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Each test opens its own in-memory SQLite and provisions the
// receipt_hashes table. This avoids depending on the store package's
// migration runner so the zohotools tests stay package-pure.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// File-backed DB instead of :memory: so concurrent goroutines see the
	// same schema. modernc.org/sqlite gives each connection a fresh
	// in-memory database when the DSN is ":memory:", which breaks the
	// parallel-claim regression test.
	// Match the production store's DSN flags — WAL + busy_timeout are what
	// let concurrent goroutines serialize cleanly on the same DB without
	// SQLITE_BUSY errors.
	dbPath := t.TempDir() + "/test.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// modernc.org/sqlite ignores the `?_journal_mode=...` DSN flags; set
	// them via PRAGMA statements explicitly. Single connection prevents
	// the parallel-write SQLITE_BUSY case that's unrelated to what we're
	// testing (we want to exercise the application-level claim race, not
	// SQLite's own concurrency model).
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		t.Fatalf("set busy_timeout: %v", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("set journal_mode: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE receipt_hashes (
		hash TEXT PRIMARY KEY,
		expense_id TEXT NOT NULL DEFAULT '',
		original_filename TEXT NOT NULL DEFAULT '',
		uploaded_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestReserveHash_FirstCallClaims(t *testing.T) {
	db := openTestDB(t)
	claimed, existing, err := ReserveHash(db, "abc123", "a.pdf")
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if !claimed {
		t.Errorf("first call should claim; got claimed=false existing=%q", existing)
	}
	if existing != "" {
		t.Errorf("first call should report empty existing; got %q", existing)
	}
}

func TestReserveHash_SecondCallSeesSentinel(t *testing.T) {
	db := openTestDB(t)
	_, _, _ = ReserveHash(db, "abc123", "a.pdf")
	claimed, existing, err := ReserveHash(db, "abc123", "b.pdf")
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if claimed {
		t.Errorf("second call should not claim")
	}
	if existing != "" {
		t.Errorf("second call before Confirm should see empty sentinel; got %q", existing)
	}
}

func TestReserveHash_AfterConfirmIsRealDuplicate(t *testing.T) {
	db := openTestDB(t)
	_, _, _ = ReserveHash(db, "abc123", "a.pdf")
	if err := ConfirmHash(db, "abc123", "exp_42"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	claimed, existing, err := ReserveHash(db, "abc123", "b.pdf")
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if claimed {
		t.Errorf("post-Confirm reservation should not claim")
	}
	if existing != "exp_42" {
		t.Errorf("post-Confirm reservation should report real expense_id; got %q", existing)
	}
}

func TestReleaseHash_RemovesUnconfirmedRow(t *testing.T) {
	db := openTestDB(t)
	_, _, _ = ReserveHash(db, "abc123", "a.pdf")
	if err := ReleaseHash(db, "abc123"); err != nil {
		t.Fatalf("release: %v", err)
	}
	// After release, the row is gone — next ReserveHash claims again.
	claimed, _, err := ReserveHash(db, "abc123", "retry.pdf")
	if err != nil {
		t.Fatalf("reserve after release: %v", err)
	}
	if !claimed {
		t.Errorf("after Release, the slot should be claimable again")
	}
}

func TestReleaseHash_DoesNotRemoveConfirmedRow(t *testing.T) {
	db := openTestDB(t)
	_, _, _ = ReserveHash(db, "abc123", "a.pdf")
	if err := ConfirmHash(db, "abc123", "exp_42"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := ReleaseHash(db, "abc123"); err != nil {
		t.Fatalf("release (should be no-op): %v", err)
	}
	// Row must still exist with the real expense_id.
	id, found, err := LookupHash(db, "abc123")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if !found || id != "exp_42" {
		t.Errorf("Release must not touch confirmed rows; got found=%v id=%q", found, id)
	}
}

// Crash-recovery regression. If a prior run reserved a hash and then
// crashed before Confirm or Release, the sentinel sits in the table
// forever. Without a TTL reclaim, every subsequent retry returns
// race-skipped and the file stays unuploaded. With the reclaim,
// ReserveHash sees the stale sentinel and claims it.
func TestReserveHash_ReclaimsStaleSentinel(t *testing.T) {
	db := openTestDB(t)
	defer func(orig time.Duration) { ReservationTTL = orig }(ReservationTTL)
	ReservationTTL = 1 * time.Second // shrink for the test

	// Simulate a crashed reservation by inserting a sentinel row with
	// an old timestamp (2 seconds ago).
	_, err := db.Exec(
		`INSERT INTO receipt_hashes (hash, expense_id, original_filename, uploaded_at)
		 VALUES (?, '', ?, datetime('now', '-2 seconds'))`,
		"stalehash", "crashed.pdf",
	)
	if err != nil {
		t.Fatalf("seed stale sentinel: %v", err)
	}

	// A fresh ReserveHash call should reclaim the stale sentinel.
	claimed, existing, err := ReserveHash(db, "stalehash", "retry.pdf")
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if !claimed {
		t.Errorf("expected to reclaim stale sentinel; got claimed=false existing=%q", existing)
	}
}

// Counter-test: a fresh sentinel (within TTL) must NOT be reclaimed.
// Otherwise the parallel-claim race regression would break.
func TestReserveHash_DoesNotReclaimFreshSentinel(t *testing.T) {
	db := openTestDB(t)
	// Reservation TTL defaults to 30 min; the just-inserted sentinel
	// is well within that window.
	_, _, _ = ReserveHash(db, "freshhash", "first.pdf")
	claimed, existing, err := ReserveHash(db, "freshhash", "second.pdf")
	if err != nil {
		t.Fatalf("second reserve: %v", err)
	}
	if claimed {
		t.Errorf("fresh sentinel must not be reclaimed; got claimed=true")
	}
	if existing != "" {
		t.Errorf("sibling reservation must report empty existing; got %q", existing)
	}
}

// Race regression — the exact pattern that motivated the patch. N
// goroutines hashing the same content all race ReserveHash; exactly one
// should claim. SQLite's ON CONFLICT DO NOTHING + RowsAffected provides
// the atomicity.
func TestReserveHash_ParallelDuplicateClaimsExactlyOne(t *testing.T) {
	db := openTestDB(t)
	const n = 20
	var wg sync.WaitGroup
	claims := make([]bool, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, _, err := ReserveHash(db, "racehash", "file.pdf")
			if err != nil {
				t.Errorf("goroutine %d: %v", i, err)
				return
			}
			claims[i] = claimed
		}()
	}
	wg.Wait()
	count := 0
	for _, c := range claims {
		if c {
			count++
		}
	}
	if count != 1 {
		t.Errorf("exactly one goroutine must claim; got %d", count)
	}
}
