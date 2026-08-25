// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestReconcileResourcePrunesOnlyUnseenRows(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	items := []json.RawMessage{
		json.RawMessage(`{"_id":1,"title":"keep","link":"https://example.com/1"}`),
		json.RawMessage(`{"_id":2,"title":"remove","link":"https://example.com/2"}`),
	}
	if _, failed, err := s.UpsertBatch("raindrops", items); err != nil || failed != 0 {
		t.Fatalf("UpsertBatch() failed=%d err=%v", failed, err)
	}
	deleted, err := s.ReconcileResource("raindrops", []string{"1"}, "raindrops", nil)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	var ids []string
	rows, err := s.DB().Query(`SELECT id FROM resources WHERE resource_type='raindrops' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if len(ids) != 1 || ids[0] != "1" {
		t.Fatalf("remaining ids = %v, want [1]", ids)
	}
}
