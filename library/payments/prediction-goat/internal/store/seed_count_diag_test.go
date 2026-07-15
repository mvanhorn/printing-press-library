// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"
)

// TestSeedRowsInserted_Diagnostic reports the per-kind seed counts.
// Pure diagnostic; never fails. Useful when changing the seed
// payload to see exactly what got committed.
func TestSeedRowsInserted_Diagnostic(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "diag.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM entity_lookups`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	t.Logf("total seeded entity_lookup rows: %d", n)

	rows, err := s.DB().Query(`SELECT kind, COUNT(*) FROM entity_lookups GROUP BY kind ORDER BY kind`)
	if err != nil {
		t.Fatalf("breakdown: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var k int
		_ = rows.Scan(&kind, &k)
		t.Logf("  %s: %d", kind, k)
	}
}
