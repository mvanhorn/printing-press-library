package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestImportKeywordSnapshotAndHistory(t *testing.T) {
	t.Parallel()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	rows := []json.RawMessage{
		json.RawMessage(`{"keyword":"self journal","campaign":"Core","ad_group":"Exact","bid":1.25,"spend":10,"sales":50,"orders":2,"clicks":20}`),
	}
	meta, err := s.ImportKeywordSnapshot(context.Background(), "", "morning", "keywords.csv", when, rows)
	if err != nil {
		t.Fatalf("ImportKeywordSnapshot returned error: %v", err)
	}
	if meta.ID == "" || meta.RowCount != 1 {
		t.Fatalf("meta = %+v", meta)
	}

	list, err := s.ListKeywordSnapshots(context.Background())
	if err != nil {
		t.Fatalf("ListKeywordSnapshots returned error: %v", err)
	}
	if len(list) != 1 || list[0].Name != "morning" {
		t.Fatalf("list = %+v", list)
	}

	history, err := s.KeywordHistory(context.Background(), "self journal")
	if err != nil {
		t.Fatalf("KeywordHistory returned error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("len(history) = %d, want 1", len(history))
	}
	var got map[string]any
	if err := json.Unmarshal(history[0], &got); err != nil {
		t.Fatalf("unmarshal history: %v", err)
	}
	if got["date"] == "" {
		t.Fatalf("history row missing date: %s", history[0])
	}
}
