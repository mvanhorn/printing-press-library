// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestAgenticSnapshotAllowsUnavailableScore(t *testing.T) {
	db, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	item, err := db.SaveAgenticSnapshot(context.Background(), []byte(`{"target":"https://example.com","score":null,"issues":[]}`), time.Now())
	if err != nil {
		t.Fatalf("save nullable-score report: %v", err)
	}
	if item.Score != nil {
		t.Fatalf("saved score = %v, want nil", *item.Score)
	}
}
