package cli

import (
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
)

// TestLoadGroupsDedupesSameGroupStoredUnderTwoKeys guards against the
// duplicated "Non-group expenses" row: a store synced by two binary
// generations holds the id-0 group under key "0" and under its name, and
// loadGroups must collapse them to one Group so `balances --by-group` (and
// every other consumer) sees each group once.
func TestLoadGroupsDedupesSameGroupStoredUnderTwoKeys(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	nonGroup := []byte(`{"id":0,"name":"Non-group expenses","members":[{"id":42,"balance":[{"currency_code":"USD","amount":"33.48"}]}]}`)
	if err := s.Upsert("get-groups", "0", nonGroup); err != nil {
		t.Fatalf("seed key 0: %v", err)
	}
	if err := s.Upsert("get-groups", "Non-group expenses", nonGroup); err != nil {
		t.Fatalf("seed name key: %v", err)
	}
	if err := s.Upsert("get-groups", "100", []byte(`{"id":100,"name":"Tahoe","members":[{"id":42,"balance":[{"currency_code":"USD","amount":"25.00"}]}]}`)); err != nil {
		t.Fatalf("seed group 100: %v", err)
	}

	groups, err := loadGroups(s)
	if err != nil {
		t.Fatalf("loadGroups: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 distinct groups (0 and 100), got %d: %+v", len(groups), groups)
	}
	rows := groupBalances(groups, 42)
	if len(rows) != 2 {
		t.Fatalf("expected one by-group row per group, got %d: %+v", len(rows), rows)
	}
	seen := map[int]int{}
	for _, r := range rows {
		seen[r.GroupID]++
	}
	if seen[0] != 1 || seen[100] != 1 {
		t.Fatalf("expected exactly one row for group 0 and one for group 100, got %v", seen)
	}
}
