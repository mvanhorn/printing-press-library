package cli

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/teslatracker/internal/store"
)

type inventoryHTMLClient struct{ html string }

func (c inventoryHTMLClient) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	if c.html != "" {
		return json.RawMessage(c.html), nil
	}
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

	full := json.RawMessage(`{"vin":"5YJ3E1EA7KF317000","model":"Model 3","mileage":27000,"image":"detail.png"}`)
	if err := db.Upsert("inventory", "5YJ3E1EA7KF317000", full); err != nil {
		t.Fatal(err)
	}
	result = syncResource(context.Background(), inventoryHTMLClient{html: `<html><body><a href="/inventory/5YJ3E1EA7KF317000">Updated Model 3</a></body></html>`}, db,
		"inventory", "", true, 1, false, false, nil, io.Discard)
	if result.Err != nil || result.Count != 1 {
		t.Fatalf("second sync result: count=%d err=%v", result.Count, result.Err)
	}
	item, err = db.Get("inventory", "5YJ3E1EA7KF317000")
	if err != nil {
		t.Fatal(err)
	}
	var detail struct {
		VIN     string `json:"vin"`
		Name    string `json:"name"`
		Mileage int    `json:"mileage"`
		URL     string `json:"url"`
	}
	if err := json.Unmarshal(item, &detail); err != nil || detail.VIN != "5YJ3E1EA7KF317000" || detail.Name != "Updated Model 3" || detail.Mileage != 27000 || detail.URL != "https://teslatracker.com/inventory/5YJ3E1EA7KF317000" {
		t.Fatalf("full detail after link sync: %s, %v", item, err)
	}
	if err := db.Upsert("inventory", "5YJ3E1EA7KF317000", full); err != nil {
		t.Fatal(err)
	}
	updatedLink := json.RawMessage(`{"url":"https://teslatracker.com/inventory/5YJ3E1EA7KF317000","name":"Newest Model 3","slug":"newest-model-3","image":""}`)
	if _, _, err := db.UpsertBatch("inventory", []json.RawMessage{updatedLink}); err != nil {
		t.Fatal(err)
	}
	item, err = db.Get("inventory", "5YJ3E1EA7KF317000")
	var current struct {
		VIN     string `json:"vin"`
		Name    string `json:"name"`
		Slug    string `json:"slug"`
		Image   string `json:"image"`
		Mileage int    `json:"mileage"`
	}
	if err != nil || json.Unmarshal(item, &current) != nil || current.VIN != "5YJ3E1EA7KF317000" || current.Name != "Newest Model 3" || current.Slug != "newest-model-3" || current.Image != "detail.png" || current.Mileage != 27000 {
		t.Fatalf("updated listing metadata with detail preserved: %s, %v", item, err)
	}
	clearedLink := json.RawMessage(`{"url":"https://teslatracker.com/inventory/5YJ3E1EA7KF317000","name":"","text":"","image":""}`)
	if _, _, err := db.UpsertBatch("inventory", []json.RawMessage{clearedLink}); err != nil {
		t.Fatal(err)
	}
	item, err = db.Get("inventory", "5YJ3E1EA7KF317000")
	var cleared struct {
		Name, Text, Image string
		Mileage           int
	}
	if err != nil || json.Unmarshal(item, &cleared) != nil || cleared.Name != "" || cleared.Text != "" || cleared.Image != "detail.png" || cleared.Mileage != 27000 {
		t.Fatalf("cleared listing text with detail image preserved: %s, %v", item, err)
	}
	vins, err := vinsFromLinks(context.Background(), db.DB())
	if err != nil || len(vins) != 1 || vins[0] != "5YJ3E1EA7KF317000" {
		t.Fatalf("VINs for hydrate after detail-preserving sync: %v, %v", vins, err)
	}
}
