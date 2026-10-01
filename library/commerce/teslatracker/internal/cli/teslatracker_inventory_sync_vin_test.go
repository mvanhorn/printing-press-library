package cli

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/teslatracker/internal/store"
)

type inventoryHTMLClient struct{}

func (inventoryHTMLClient) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	return json.RawMessage(`<html><body><a href="/inventory/5YJ3E1EA7KF317000">Model 3</a></body></html>`), nil
}

func (inventoryHTMLClient) RequestBaseURL() string  { return "https://teslatracker.com" }
func (inventoryHTMLClient) LastContentType() string { return "text/html" }
func (inventoryHTMLClient) RateLimit() float64      { return 0 }

func TestSyncInventoryHTMLLinkCanBeReadByVIN(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "inventory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result := syncResource(context.Background(), inventoryHTMLClient{}, db,
		"inventory", "", true, 1, false, false, nil, io.Discard)
	if result.Err != nil || result.Count != 1 {
		t.Fatalf("sync result: count=%d err=%v warn=%v", result.Count, result.Err, result.Warn)
	}
	item, err := db.Get("inventory", "5YJ3E1EA7KF317000")
	if err != nil {
		t.Fatalf("VIN lookup after normal HTML sync: %v", err)
	}
	var link struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(item, &link); err != nil || link.URL != "https://teslatracker.com/inventory/5YJ3E1EA7KF317000" {
		t.Fatalf("stored link: %s, %v", item, err)
	}
}
