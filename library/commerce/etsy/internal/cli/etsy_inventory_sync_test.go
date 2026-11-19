package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

type inventorySyncClient struct {
	url    string
	client *http.Client
}

func (c inventorySyncClient) RateLimit() float64 { return 1000 }
func (c inventorySyncClient) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.url+path, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}
func TestEtsyInventoryDependentSync(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("ETSY_SHOP_ID", "9000001")
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	s2Must(t, err)
	defer db.Close()
	s2Upsert(t, db.UpsertListings, s2Listing1)
	s2Upsert(t, db.UpsertListings, s2Listing2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := int64(7000000001)
		if strings.Contains(r.URL.Path, "1000000002") {
			base += 2
		}
		fmt.Fprintf(w, `{"products":[{"product_id":%d,"sku":"DUP-SKU","is_deleted":false,"offerings":[{"offering_id":%d,"quantity":0,"is_enabled":true,"is_deleted":false,"price":{"amount":1000,"divisor":100,"currency_code":"USD"}}]},{"product_id":%d,"sku":"OTHER","offerings":[{"offering_id":%d,"quantity":2,"is_enabled":true,"price":{"amount":1000,"divisor":100,"currency_code":"USD"}}]}],"price_on_property":[],"quantity_on_property":[],"sku_on_property":[]}`, base, base+100, base+1, base+101)
	}))
	defer srv.Close()
	var dep dependentResourceDef
	for _, d := range dependentResourceDefs() {
		if d.Name == "listings_inventory" {
			dep = d
		}
	}
	result := syncDependentResource(context.Background(), inventorySyncClient{srv.URL, srv.Client()}, db, dep, "", true, 0, false, false, nil, nil, 1)
	if result.Err != nil || result.Count != 4 {
		t.Fatalf("sync result: %+v", result)
	}
	n := 0
	s2Must(t, db.ListScan("listings_inventory", func(id string, raw json.RawMessage) bool {
		m, e := s2DecodeObject(raw)
		s2Must(t, e)
		lid := int64(1000000001)
		if s2Int(m["product_id"]) >= 7000000003 {
			lid++
		}
		if store.BareResourceID(id) != s2Str(m["product_id"]) || s2InventoryID(m["listing_id"]) != lid || s2InventoryID(m["parent_id"]) != lid || len(m["offerings"].([]any)) == 0 {
			t.Errorf("row %s: %s", id, raw)
		}
		n++
		return true
	}))
	if n != 4 {
		t.Fatalf("stored %d rows", n)
	}
	for _, lid := range []int64{1000000001, 1000000002} {
		m, e := s2DecodeObject([]byte(s2StoredInventory(db, lid)))
		s2Must(t, e)
		if len(m["products"].([]any)) != 2 {
			t.Fatalf("snapshot inventory: %v", m)
		}
	}
	s2Must(t, db.Close())
	out, _, err := s2Run(t, "inventory", "low-stock", "--threshold", "3", "--db", path, "--json")
	s2Must(t, err)
	var rows []s2LowStockRow
	etsyDecodeLowStock(t, out, &rows)
	if len(rows) != 4 || rows[0].Quantity != 0 || rows[0].ListingID != 1000000001 || rows[0].Title != "Hand-glazed ceramic mug" {
		t.Fatalf("low stock: %s", out)
	}
	out, _, err = s2Run(t, "inventory", "audit", "--db", path, "--agent")
	s2Must(t, err)
	var envelope struct {
		Results struct {
			Results []map[string]any `json:"results"`
		} `json:"results"`
	}
	s2Decode(t, out, &envelope)
	findings := envelope.Results.Results
	zero, dup := false, false
	for _, f := range findings {
		zero = zero || f["type"] == "zero_quantity"
		dup = dup || (f["type"] == "duplicate_sku" && f["sku"] == "DUP-SKU")
	}
	if !zero || !dup {
		t.Fatalf("audit: %s", out)
	}
}

// Mixed mirrors can contain the same product under both inventory aliases.
// Preserve the first copy without collapsing distinct products on a listing.
func TestInventoryProductsNormalizeMixedRows(t *testing.T) {
	testenv.Isolate(t)
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	defer db.Close()
	legacy := json.RawMessage(`{"listing_id":1000000001,"products":[{"product_id":1,"sku":"FIRST","offerings":[{"offering_id":11,"quantity":0}]}]}`)
	s2Must(t, db.Upsert("listings_inventory", "1000000001", legacy))
	s2Must(t, db.Upsert("listing_inventory", "1", json.RawMessage(`{"parent_id":1000000001,"product_id":1,"sku":"SECOND","offerings":[{"offering_id":11,"quantity":2}]}`)))
	s2Must(t, db.Upsert("listing_inventory", "2", json.RawMessage(`{"parent_id":1000000001,"product_id":2,"offerings":[{"offering_id":12,"quantity":2}]}`)))
	products, err := s2InventoryProducts(db)
	s2Must(t, err)
	if len(products) != 2 {
		t.Fatalf("products: %+v", products)
	}
	var first map[string]any
	for _, p := range products {
		if s2Int(p.Product["product_id"]) == 1 {
			first = p.Product
		}
	}
	// The resource-type index scans the singular alias first in this fixture.
	if first["sku"] != "SECOND" {
		t.Fatalf("deduplication: %+v", first)
	}
	if got := s2StoredInventory(db, 1000000001); got != string(legacy) {
		t.Fatalf("legacy inventory changed: %s", got)
	}
	if got := s2StoredInventory(db, 999); got != "" {
		t.Fatalf("missing inventory: %s", got)
	}
}
