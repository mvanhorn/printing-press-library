package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateReadOnlyQuery(t *testing.T) {
	allowed := []string{"SELECT 1", "WITH x AS (SELECT 1) SELECT * FROM x", "SELECT 'a;b'; -- trailing"}
	for _, query := range allowed {
		if err := ValidateReadOnlyQuery(query); err != nil {
			t.Errorf("allow %q: %v", query, err)
		}
	}
	rejected := []string{"EXPLAIN SELECT 1", "SELECT 1; ATTACH DATABASE '/tmp/x' AS x", "SELECT 1; VACUUM INTO '/tmp/x'", "/*x*/ ATTACH DATABASE '/tmp/x' AS x"}
	for _, query := range rejected {
		if err := ValidateReadOnlyQuery(query); err == nil {
			t.Errorf("accepted %q", query)
		}
	}
}

func TestBoundedReadOnlyQueryRejectsLargeQueryAndCell(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "bounded.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, _, _, err := BoundedReadOnlyQuery(context.Background(), db.DB(), "/*"+strings.Repeat("x", 1000)+"*/ SELECT 1", 10, 1000, time.Second); err == nil {
		t.Fatal("oversized query accepted")
	}
	if _, _, _, err := BoundedReadOnlyQuery(context.Background(), db.DB(), "SELECT hex(zeroblob(2000))", 10, 1000, time.Second); err == nil {
		t.Fatal("oversized cell accepted")
	}
}
