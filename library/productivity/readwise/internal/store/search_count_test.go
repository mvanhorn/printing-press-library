package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSearchCountMatchesShortTokenAndOffsetPages(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, id := range []string{"one", "two"} {
		if err := s.Upsert("documents", id, json.RawMessage(`{"title":"營造 phase 2"}`)); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{"營造", "phase-2", "phase"} {
		count, err := s.SearchCountContext(context.Background(), query)
		if err != nil || count != 2 {
			t.Fatalf("count(%q) = %d, %v; want 2", query, count, err)
		}
		page, err := s.SearchContextOffset(context.Background(), query, 1, 1)
		if err != nil || len(page) != 1 {
			t.Fatalf("second page(%q) = %v, %v; want one row", query, page, err)
		}
	}
}
