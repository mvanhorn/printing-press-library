package store

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestOpenReadOnly_BlocksDirectWrite is the simple half — a plain DELETE
// must fail.
func TestOpenReadOnly_BlocksDirectWrite(t *testing.T) {
	path := newTempDB(t)

	roDB, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer roDB.Close()

	_, err = roDB.ExecContext(context.Background(), `DELETE FROM audit`)
	if err == nil {
		t.Fatal("expected DELETE to fail against read-only DB; it succeeded")
	}
	if !looksLikeReadOnlyErr(err) {
		t.Errorf("expected a read-only / write-blocked error, got: %v", err)
	}
}

// TestOpenReadOnly_BlocksCTEDelete is the load-bearing test — a write
// smuggled inside a CTE (`WITH x AS (SELECT 1) DELETE FROM audit`) was the
// exact bypass the prefix-only heuristic missed. It must fail at the engine
// level even though the statement starts with WITH.
func TestOpenReadOnly_BlocksCTEDelete(t *testing.T) {
	path := newTempDB(t)

	// Seed an audit row via the read/write handle so the DELETE has something
	// to chew on.
	rw, err := OpenAndMigrate(path)
	if err != nil {
		t.Fatalf("OpenAndMigrate: %v", err)
	}
	if _, err := rw.ExecContext(context.Background(), `
		INSERT INTO audit(command, request, hash, confirmed)
		VALUES ('test', '{}', 'deadbeef', 0)`); err != nil {
		t.Fatalf("seed audit row: %v", err)
	}
	rw.Close()

	roDB, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer roDB.Close()

	// The bypass: starts with WITH, so a prefix-only check would let it
	// through. SQLite read-only mode (mode=ro + query_only=1) must refuse it.
	for _, q := range []string{
		`WITH x AS (SELECT 1) DELETE FROM audit`,
		`with cte as (select 1) delete from audit`, // lowercase
		`WITH a AS (SELECT 1), b AS (SELECT 2) UPDATE audit SET hash='x'`,
		`WITH x AS (SELECT 1) INSERT INTO audit(command,request,hash,confirmed) VALUES ('x','{}','y',1)`,
	} {
		_, err := roDB.ExecContext(context.Background(), q)
		if err == nil {
			// Even worse — confirm the row is still there by selecting from
			// a fresh handle.
			rw2, _ := OpenAndMigrate(path)
			var n int
			_ = rw2.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM audit`).Scan(&n)
			rw2.Close()
			t.Errorf("BYPASS: %q did not error. audit row count now %d", q, n)
			continue
		}
		if !looksLikeReadOnlyErr(err) {
			t.Errorf("unexpected error for %q: %v", q, err)
		}
	}

	// Confirm the seeded row survived all bypass attempts.
	var n int
	if err := roDB.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM audit`).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Errorf("audit row count = %d; expected 1 (no rows should have been deleted)", n)
	}
}

// TestOpenReadOnly_AllowsSelect verifies the read-only handle actually works
// for the legitimate SELECT case — we'd be sad if we tightened so hard nothing
// could read.
func TestOpenReadOnly_AllowsSelect(t *testing.T) {
	path := newTempDB(t)

	roDB, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer roDB.Close()

	var n int
	if err := roDB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("select against ro: %v", err)
	}
	if n < 1 {
		t.Errorf("schema_migrations should have at least 1 row, got %d", n)
	}
}

// TestOpenReadOnly_ErrorsIfMissing — the explicit "you forgot to sync" UX.
func TestOpenReadOnly_ErrorsIfMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such.db")
	_, err := OpenReadOnly(missing)
	if err == nil {
		t.Fatal("expected error on missing file")
	}
	if !strings.Contains(err.Error(), "sync") {
		t.Errorf("error should mention sync remediation: %v", err)
	}
}

func TestOpenAndMigrateConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, err := OpenAndMigrate(path)
			if err != nil {
				errs <- err
				return
			}
			errs <- db.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("OpenAndMigrate concurrent error: %v", err)
		}
	}

	db, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly after concurrent migrate: %v", err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	migrations, err := listMigrations()
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if n != len(migrations) {
		t.Fatalf("schema_migrations count = %d, want %d", n, len(migrations))
	}
}

// newTempDB returns a path to a freshly-migrated empty DB. Caller doesn't
// have to clean up — t.TempDir handles it.
func newTempDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := OpenAndMigrate(path)
	if err != nil {
		t.Fatalf("OpenAndMigrate: %v", err)
	}
	db.Close()
	return path
}

// looksLikeReadOnlyErr accepts any of the variants SQLite produces when a
// read-only handle refuses a write. We don't pin to one string because the
// driver layer wraps differently across versions.
func looksLikeReadOnlyErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, sub := range []string{
		"readonly",
		"read-only",
		"read only",
		"attempt to write",
		"cannot modify",
		"query_only",
	} {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
