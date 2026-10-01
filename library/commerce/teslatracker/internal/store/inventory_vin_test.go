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

func TestInventoryLinkUsesVINInsteadOfDescriptiveSlug(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	const vin = "5YJ3E1EA7KF317000"
	link := json.RawMessage(`{"name":"Model 3","slug":"model-3","url":"https://teslatracker.com/inventory/5YJ3E1EA7KF317000?source=list"}`)
	stored, skipped, err := s.UpsertBatch("inventory", []json.RawMessage{link})
	if err != nil || stored != 1 || skipped != 0 {
		t.Fatalf("stored=%d skipped=%d err=%v", stored, skipped, err)
	}
	got, err := s.Get("inventory", vin)
	if err != nil || string(got) != string(link) {
		t.Fatalf("VIN lookup after link sync: %s, %v", got, err)
	}
	if _, err := s.Get("inventory", "model-3"); err == nil {
		t.Fatal("descriptive slug unexpectedly used as inventory ID")
	}
}

func TestInventoryLinkVINRequiresCompletePath(t *testing.T) {
	for _, raw := range []string{
		"https://teslatracker.com/inventory/5YJ3E1EA7KF3170000",
		"https://teslatracker.com/inventory/5YJ3E1EA7KF317000/extra",
		"https://teslatracker.com/other/5YJ3E1EA7KF317000",
	} {
		if got := inventoryLinkVIN(map[string]any{"url": raw}); got != "" {
			t.Errorf("inventoryLinkVIN(%q) = %q, want empty", raw, got)
		}
	}
}
