package store

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSearchByType_ReturnsOnlyMatchingType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	hack := []json.RawMessage{
		json.RawMessage(`{"id":"h1","title":"sql injection in checkout"}`),
		json.RawMessage(`{"id":"h2","title":"sql injection in admin panel"}`),
		json.RawMessage(`{"id":"h3","title":"xss in profile"}`),
	}
	if _, _, err := s.UpsertBatch("hacktivity", hack); err != nil {
		t.Fatalf("upsert hack: %v", err)
	}
	user := []json.RawMessage{
		json.RawMessage(`{"id":"u1","title":"sql injection in mobile api"}`),
	}
	if _, _, err := s.UpsertBatch("user-reports", user); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	hits, err := s.SearchByType("injection", "hacktivity", 10)
	if err != nil {
		t.Fatalf("SearchByType hacktivity: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("hacktivity injection hits = %d, want 2", len(hits))
	}
	for _, raw := range hits {
		if !strings.Contains(string(raw), "h") || strings.Contains(string(raw), `"u1"`) {
			t.Errorf("hit leaked across types: %s", string(raw))
		}
	}

	userHits, err := s.SearchByType("injection", "user-reports", 10)
	if err != nil {
		t.Fatalf("SearchByType user: %v", err)
	}
	if len(userHits) != 1 {
		t.Errorf("user-reports injection hits = %d, want 1", len(userHits))
	}
}

func TestSearchByType_EmptyQueryReturnsNil(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	_, _, _ = s.UpsertBatch("hacktivity", []json.RawMessage{
		json.RawMessage(`{"id":"h1","title":"some title"}`),
	})

	for _, q := range []string{"", "  ", "\t\n"} {
		hits, err := s.SearchByType(q, "hacktivity", 10)
		if err != nil {
			t.Errorf("SearchByType(%q) returned error: %v", q, err)
		}
		if hits != nil {
			t.Errorf("SearchByType(%q) = %v, want nil", q, hits)
		}
	}
}

func TestSearchByType_UnknownTypeReturnsEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	_, _, _ = s.UpsertBatch("hacktivity", []json.RawMessage{
		json.RawMessage(`{"id":"h1","title":"sql injection"}`),
	})

	hits, err := s.SearchByType("injection", "nonexistent-type", 10)
	if err != nil {
		t.Fatalf("SearchByType: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("unknown-type hits = %d, want 0", len(hits))
	}
}

// TestSearchByType_FTS5SpecialCharsNoCrash locks the fix for the
// greptile P1 on PR #459: raw user-supplied titles containing FTS5
// syntax characters (unbalanced parens, double quotes, asterisks,
// keywords) must not propagate a SQL parse error through report
// dedupe / submit. The quoteFTS5Phrase helper wraps every query as a
// phrase so any token sequence is parseable.
func TestSearchByType_FTS5SpecialCharsNoCrash(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	_, _, _ = s.UpsertBatch("hacktivity", []json.RawMessage{
		json.RawMessage(`{"id":"h1","title":"SQL injection bypass"}`),
		json.RawMessage(`{"id":"h2","title":"XSS reflected"}`),
	})

	cases := []string{
		"(bypass only)",                // unbalanced subexpression
		`report "wrap" injection`,      // internal double quote
		"title with * in it",           // bare asterisk
		"OR injection",                 // keyword position
		"NOT a valid query AND broken", // keyword combo
		`""`,                           // literal empty quotes
	}
	for _, q := range cases {
		hits, err := s.SearchByType(q, "hacktivity", 10)
		if err != nil {
			t.Errorf("SearchByType(%q) returned error: %v (want no crash)", q, err)
		}
		_ = hits
	}
}
