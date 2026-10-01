package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestInventoryPersistsVINOnlyRecords(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	item := json.RawMessage(`{"vin":"TESTVIN12345678901","price":12345}`)
	stored, skipped, err := s.UpsertBatch("inventory", []json.RawMessage{item})
	if err != nil || stored != 1 || skipped != 0 {
		t.Fatalf("stored=%d skipped=%d err=%v", stored, skipped, err)
	}
	got, err := s.Get("inventory", "TESTVIN12345678901")
	if err != nil || string(got) != string(item) {
		t.Fatalf("read after sync: %s, %v", got, err)
	}
}
