// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

// TestOpenAppliesWAL verifies the connection hook actually puts the
// database into WAL journal mode. Before this fix, the DSN-level
// `_journal_mode=WAL` was silently ignored by modernc.org/sqlite v1.37.0
// and the live store ran with the default `delete` journal — which is
// the root cause of the SQLITE_BUSY failures we saw under concurrent
// reads.
func TestOpenAppliesWAL(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	s, err := OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer s.Close()

	var mode string
	if err := s.DB().QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q (DSN pragma path is unreliable; connection hook must apply WAL)", mode, "wal")
	}
}

// TestOpenAppliesBusyTimeout verifies every pooled connection carries
// the 5-second busy_timeout. busy_timeout is connection-level (not
// database-level), so the hook has to fire on each new connection in
// the pool. We exercise both pool slots by holding one connection open
// while querying busy_timeout on another.
func TestOpenAppliesBusyTimeout(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	s, err := OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	conn1, err := s.DB().Conn(ctx)
	if err != nil {
		t.Fatalf("Conn 1: %v", err)
	}
	defer conn1.Close()

	conn2, err := s.DB().Conn(ctx)
	if err != nil {
		t.Fatalf("Conn 2: %v", err)
	}
	defer conn2.Close()

	// Both pooled conns should report busy_timeout=5000.
	var bt1, bt2 int
	if err := conn1.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&bt1); err != nil {
		t.Fatalf("conn1 busy_timeout: %v", err)
	}
	if err := conn2.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&bt2); err != nil {
		t.Fatalf("conn2 busy_timeout: %v", err)
	}
	if bt1 != 5000 {
		t.Errorf("conn1 busy_timeout = %d, want 5000", bt1)
	}
	if bt2 != 5000 {
		t.Errorf("conn2 busy_timeout = %d, want 5000", bt2)
	}
}

// TestConcurrentReadsDoNotFail seeds the store and fires four parallel
// reads. With WAL + busy_timeout in place, all four must succeed.
// Pre-fix this would intermittently fail with "database is locked".
func TestConcurrentReadsDoNotFail(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	s, err := OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer s.Close()

	if _, err := s.DB().Exec(`INSERT INTO resources(id, resource_type, data) VALUES ('seed', 'test', '{}')`); err != nil {
		t.Fatalf("seed insert: %v", err)
	}

	const readers = 4
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var n int
			if err := s.DB().QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n); err != nil {
				errs <- err
				return
			}
			if n != 1 {
				errs <- &countMismatchErr{got: n, want: 1}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent read: %v", err)
	}
}

// TestReadDuringWriteDoesNotFail starts a write transaction in one
// goroutine and a read in another. Under WAL the read should not block
// on the write; pre-fix it would fail with SQLITE_BUSY.
func TestReadDuringWriteDoesNotFail(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	s, err := OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	writeConn, err := s.DB().Conn(ctx)
	if err != nil {
		t.Fatalf("write conn: %v", err)
	}
	defer writeConn.Close()

	tx, err := writeConn.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO resources(id, resource_type, data) VALUES ('w', 'test', '{}')`); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert in tx: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		var n int
		done <- s.DB().QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n)
	}()

	if err := <-done; err != nil {
		_ = tx.Rollback()
		t.Fatalf("read during open write tx: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

type countMismatchErr struct {
	got, want int
}

func (e *countMismatchErr) Error() string {
	return "row count mismatch"
}
