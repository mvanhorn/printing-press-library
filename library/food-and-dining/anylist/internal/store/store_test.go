package store

import (
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
)

func TestGetListsByStoreMatchesStoreIDsExactly(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	if _, err := st.db.Exec(`INSERT INTO lists (id, name) VALUES ('list-1', 'Groceries')`); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO stores (id, list_id, name, sort_index) VALUES
		('abc', 'list-1', 'Short ID Store', 1),
		('xyzabc123', 'list-1', 'Exact Store', 2)`); err != nil {
		t.Fatalf("insert stores: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO items
		(id, list_id, name, checked, manual_sort_index, store_ids)
		VALUES ('item-1', 'list-1', 'Milk', 0, 1, '["xyzabc123"]')`); err != nil {
		t.Fatalf("insert item: %v", err)
	}

	groups, err := st.GetListsByStore("list-1")
	if err != nil {
		t.Fatalf("GetListsByStore returned error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1: %#v", len(groups), groups)
	}
	if groups[0].StoreName != "Exact Store" {
		t.Fatalf("StoreName = %q, want %q", groups[0].StoreName, "Exact Store")
	}
	if len(groups[0].Items) != 1 || groups[0].Items[0].ID != "item-1" {
		t.Fatalf("items = %#v, want only item-1", groups[0].Items)
	}
}
