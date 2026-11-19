package cli

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func s2LowStockFixture(t *testing.T) string {
	return s2FixtureDB(t, func(db *store.Store) {
		s2Upsert(t, db.UpsertListings, s2Listing1)
		s2Upsert(t, db.UpsertListings, s2Listing2)
		inv := `{"listing_id":1000000001,"parent_id":1000000001,"products":[
		 {"product_id":1,"sku":"A-ZERO","is_deleted":false,"offerings":[{"offering_id":11,"quantity":0,"is_enabled":true,"is_deleted":false}]},
		 {"product_id":2,"sku":"B-TWO","is_deleted":false,"offerings":[{"offering_id":12,"quantity":2,"is_enabled":true,"is_deleted":false}]},
		 {"product_id":3,"sku":"C-FIVE","is_deleted":false,"offerings":[{"offering_id":13,"quantity":5,"is_enabled":true,"is_deleted":false}]},
		 {"product_id":4,"sku":"D-GONE","is_deleted":false,"offerings":[{"offering_id":14,"quantity":1,"is_enabled":true,"is_deleted":true}]},
		 {"product_id":5,"sku":"E-OFF","is_deleted":false,"offerings":[{"offering_id":15,"quantity":1,"is_enabled":false,"is_deleted":false}]},
		 {"product_id":6,"sku":"F-DELPROD","is_deleted":true,"offerings":[{"offering_id":16,"quantity":0,"is_enabled":true,"is_deleted":false}]}]}`
		s2Must(t, db.Upsert("listings_inventory", "1000000001", json.RawMessage(inv)))
	})
}

func TestLowStockReturnsOnlyLiveOfferingsAtOrBelowThreshold(t *testing.T) {
	t.Setenv("ETSY_SHOP_ID", "9000001")
	db := s2LowStockFixture(t)
	out, _, err := s2Run(t, "inventory", "low-stock", "--threshold", "3", "--db", db, "--json")
	s2Must(t, err)
	var rows []s2LowStockRow
	etsyDecodeLowStock(t, out, &rows)
	if len(rows) != 2 {
		t.Fatalf("rows = %s", out)
	}
	if rows[0].SKU != "A-ZERO" || rows[0].Quantity != 0 || rows[1].SKU != "B-TWO" || rows[1].Quantity != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Title != "Hand-glazed ceramic mug" || rows[0].ListingID != 1000000001 || rows[0].OfferingID != "11" {
		t.Fatalf("row detail = %+v", rows[0])
	}
	// threshold 0 narrows to the zero offering only
	out, _, _ = s2Run(t, "inventory", "low-stock", "--threshold", "0", "--db", db, "--json")
	etsyDecodeLowStock(t, out, &rows)
	if len(rows) != 1 || rows[0].SKU != "A-ZERO" {
		t.Fatalf("threshold 0 rows = %s", out)
	}
	// a higher threshold picks up the 5 too, still skipping deleted/disabled
	out, _, _ = s2Run(t, "inventory", "low-stock", "--threshold", "5", "--db", db, "--json")
	etsyDecodeLowStock(t, out, &rows)
	if len(rows) != 3 {
		t.Fatalf("threshold 5 rows = %s", out)
	}
}

func TestLowStockEmptyAndMissingStoreReturnEmptyArray(t *testing.T) {
	empty := s2FixtureDB(t, nil)
	out, _, err := s2Run(t, "inventory", "low-stock", "--db", empty, "--json")
	s2Must(t, err)
	if len(etsyLowStockResults(t, out)) != 0 {
		t.Fatalf("empty store output = %q", out)
	}
	missing := filepath.Join(t.TempDir(), "absent.db")
	out, errOut, err := s2Run(t, "inventory", "low-stock", "--db", missing, "--json")
	s2Must(t, err)
	if len(etsyLowStockResults(t, out)) != 0 || !strings.Contains(out, shopxNoteNoStore) {
		t.Fatalf("missing store out=%q err=%q", out, errOut)
	}
	out, _, err = s2Run(t, "inventory", "low-stock", "--dry-run", "--json")
	s2Must(t, err)
	var env map[string]any
	s2Decode(t, out, &env)
	if env["dry_run"] != true {
		t.Fatalf("dry-run = %s", out)
	}
	if _, _, err := s2Run(t, "inventory", "low-stock", "--threshold", "-1", "--db", empty); err == nil || ExitCode(err) != 2 {
		t.Fatalf("negative threshold: %v", err)
	}
}

func TestLowStockHintsWhenListingsSyncedButNoInventory(t *testing.T) {
	db := s2FixtureDB(t, func(db *store.Store) { s2Upsert(t, db.UpsertListings, s2Listing1) })
	out, errOut, err := s2Run(t, "inventory", "low-stock", "--db", db, "--json")
	s2Must(t, err)
	if len(etsyLowStockResults(t, out)) != 0 || !strings.Contains(errOut, "--refresh") {
		t.Fatalf("out=%q err=%q", out, errOut)
	}
}

func TestLowStockRefreshCachesInventoryFromAPI(t *testing.T) {
	s2FastThrottle(t, 1)
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
		if r.URL.Path == "/listings/batch/inventory" {
			results := []any{}
			for _, id := range strings.Split(r.URL.Query().Get("listing_ids"), ",") {
				inventory, err := s2DecodeObject([]byte(s2Inventory))
				s2Must(t, err)
				offset := int64(0)
				if id == "1000000002" {
					offset = 1000
				}
				for _, value := range inventory["products"].([]any) {
					product := value.(map[string]any)
					product["product_id"] = s2Int(product["product_id"]) + offset
				}
				results = append(results, map[string]any{"listing_id": id, "inventory": inventory})
			}
			s2Must(t, json.NewEncoder(w).Encode(map[string]any{"results": results}))
			return true
		}
		return false
	})
	db := s2FixtureDB(t, func(db *store.Store) {
		s2Upsert(t, db.UpsertListings, s2Listing1)
		s2Upsert(t, db.UpsertListings, s2Listing2)
	})
	out, _, err := s2Run(t, "inventory", "low-stock", "--threshold", "3", "--refresh", "--db", db, "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	var rows []s2LowStockRow
	etsyDecodeLowStock(t, out, &rows)
	// both listings returned distinct products with the same quantities: the qty-2 large mug, once per listing
	if len(rows) != 2 || rows[0].Quantity != 2 || rows[0].SKU != "MUG-SPECKLE-L" {
		t.Fatalf("rows = %s", out)
	}
	if srv.count("GET", "/listings/batch/inventory") != 1 || len(srv.all()) != 1 {
		t.Fatalf("requests = %+v", srv.all())
	}
	cached, openErr := store.OpenReadOnly(db)
	s2Must(t, openErr)
	assertInventoryProductRows(t, cached, 6)
	s2Must(t, cached.Close())
	// a second plain run is served from the cache with no further calls
	before := len(srv.all())
	_, _, err = s2Run(t, "inventory", "low-stock", "--db", db, "--json")
	s2Must(t, err)
	if len(srv.all()) != before {
		t.Fatal("cached read hit the API")
	}
}

func TestLowStockRefreshReplacesLegacyInventory(t *testing.T) {
	s2FastThrottle(t, 1)
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
		if r.URL.Path == "/listings/batch/inventory" {
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"listing_id": "1000000001", "inventory": json.RawMessage(s2Inventory)}}})
			return true
		}
		if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/inventory") {
			_, _ = w.Write([]byte(s2Inventory))
			return true
		}
		return false
	})
	path := s2FixtureDB(t, func(db *store.Store) {
		s2Upsert(t, db.UpsertListings, s2Listing1)
		legacy := `{"parent_id":"1000000001","listing_id":1000000001,"products":[{"product_id":99,"sku":"STALE","offerings":[{"offering_id":99,"quantity":0,"is_enabled":true}]}]}`
		s2Must(t, db.Upsert("listings_inventory", "1000000001", json.RawMessage(legacy)))
		s2Must(t, db.Upsert("listings_inventory", "98", json.RawMessage(`{"product_id":98,"listing_id":1000000001,"parent_id":1000000001,"offerings":[{"offering_id":98,"quantity":0}]}`)))
	})
	out, _, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	var rows []s2LowStockRow
	etsyDecodeLowStock(t, out, &rows)
	if len(rows) != 1 || rows[0].SKU != "MUG-SPECKLE-L" {
		t.Fatalf("refresh rows: %s", out)
	}
	db, err := store.OpenReadOnly(path)
	s2Must(t, err)
	defer db.Close()
	assertInventoryProductRows(t, db, 3)
}

func assertInventoryProductRows(t *testing.T, db *store.Store, want int) {
	t.Helper()
	count := 0
	s2Must(t, db.ListScan("listings_inventory", func(id string, data json.RawMessage) bool {
		m, err := s2DecodeObject(data)
		s2Must(t, err)
		if s2Str(m["product_id"]) == "" || s2InventoryID(m["listing_id"]) == 0 || s2InventoryID(m["parent_id"]) != s2InventoryID(m["listing_id"]) {
			t.Fatalf("not a product row: %s", data)
		}
		if _, legacy := m["products"]; legacy {
			t.Fatalf("legacy row: %s", data)
		}
		count++
		return true
	}))
	if count != want {
		t.Fatalf("stored products = %d, want %d", count, want)
	}
}
