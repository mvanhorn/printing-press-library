// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestVectorStorageIdentityIncludesIndexAndNamespace(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "vectors.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	items := []json.RawMessage{
		json.RawMessage(`{"id":"shared","index_name":"index-a","namespace":"one"}`),
		json.RawMessage(`{"id":"shared","index_name":"index-a","namespace":"two"}`),
		json.RawMessage(`{"id":"shared","index_name":"index-b","namespace":"one"}`),
	}
	stored, failed, err := s.UpsertBatch("vectors", items)
	if err != nil {
		t.Fatalf("upsert vectors: %v", err)
	}
	if stored != 3 || failed != 0 {
		t.Fatalf("stored=%d failed=%d, want 3/0", stored, failed)
	}
	var count int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = 'vectors'`).Scan(&count); err != nil {
		t.Fatalf("count vectors: %v", err)
	}
	if count != 3 {
		t.Fatalf("scoped vector count = %d, want 3", count)
	}
}
