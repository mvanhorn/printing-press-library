package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
)

func TestDeleteContextAtomicFTSFailureAndCancellation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "delete.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	body := json.RawMessage(`{"id":"videos-singleton","name":"phantomenvelope"}`)
	if err := s.Upsert("videos", "videos-singleton", body); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := s.DeleteContext(ctx, "videos", "videos-singleton"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled delete: %v", err)
	}
	if rows, err := s.Search("phantomenvelope", 10); err != nil || len(rows) != 1 {
		t.Fatalf("canceled delete changed FTS: %s, %v", rows, err)
	}
	// Fail the second statement: the first resource delete must roll back.
	if _, err := s.DB().Exec(`DROP TABLE resources_fts`); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("videos", "videos-singleton"); err == nil {
		t.Fatal("delete succeeded without its FTS table")
	}
	if data, err := s.Get("videos", "videos-singleton"); err != nil || string(data) != string(body) {
		t.Fatalf("failed FTS delete lost resource: %s, %v", data, err)
	}
}
