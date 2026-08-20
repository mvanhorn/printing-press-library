package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/printify/internal/store"
)

func TestResolveLocalStripsPrintifyJSONSuffix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dbPath := defaultDBPath("printify-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.Upsert("products", "456", json.RawMessage(`{"id":"456","title":"Synced"}`)); err != nil {
		t.Fatalf("upsert product: %v", err)
	}
	db.Close()

	data, _, err := resolveLocal(context.Background(), nil, nil, "products", false, "/v1/shops/123/products/456.json", nil, "test")
	if err != nil {
		t.Fatalf("resolve local: %v", err)
	}
	if !strings.Contains(string(data), `"id":"456"`) {
		t.Fatalf("unexpected local data: %s", data)
	}
}
