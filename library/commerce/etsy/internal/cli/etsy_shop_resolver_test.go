package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func etsyConfigShop(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[template_vars]\nshop_id = \"9000001\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ETSY_CONFIG", path)
	t.Setenv("ETSY_SHOP_ID", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
}

func TestEtsyShopResolverConfigAndWriteThrough(t *testing.T) {
	db := etsyWriteThroughFixture(t)
	etsyConfigShop(t)
	if shopxShopID(nil) != "9000001" || bulkShopID("") != "9000001" || bulkShopID("555001") != "555001" {
		t.Fatal("config/flag resolution")
	}
	writeThroughCache(context.Background(), "listings", json.RawMessage(`[{"listing_id":1000000001,"shop_id":9000001},{"listing_id":1000000002,"shop_id":555001}]`))
	etsyWriteThroughCount(t, db, "listings", 1)
	etsyWriteThroughCount(t, db, "marketplace_listings", 1)
	t.Setenv("ETSY_SHOP_ID", "555001")
	if shopxShopID(nil) != "555001" || bulkShopID("") != "555001" {
		t.Fatal("env must win")
	}
}

func TestEtsyShopResolverSingleEnvReader(t *testing.T) {
	pattern := regexp.MustCompile(`(?:EnvOverride|Getenv)\("ETSY_SHOP_ID"\)`)
	for _, file := range []string{"shopx_helpers.go", "listings_bulk_edit.go", "bulk_plan.go", "etsy_writethrough.go", "inventory_low_stock.go", "market_snapshot.go"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if file == "shopx_helpers.go" {
			want = 1
		}
		if n := len(pattern.FindAll(data, -1)); n != want {
			t.Fatalf("%s: %d direct readers, want %d", file, n, want)
		}
	}
}

func TestEtsyShopResolverBulkAndReaders(t *testing.T) {
	f := shopxNewFixture(t)
	etsyConfigShop(t)
	for i, state := range []any{"active", "active", nil, "draft", ""} {
		shop := 9000001
		if i == 1 {
			shop = 555001
		}
		f.put("listings", map[string]any{"listing_id": 1000000001 + i, "shop_id": shop, "state": state, "title": "Juniper Ceramics", "ending_timestamp": shopxIn(shopxDay)})
	}
	f.put("listings-active", map[string]any{"listing_id": 1000000009, "shop_id": 9000001, "state": "active", "ending_timestamp": shopxIn(shopxDay)})
	f.put("shops_listings", map[string]any{"listing_id": 1000000010, "shop_id": 9000001, "state": "active"})
	if n := len(shopxRows(f.db, shopxListingTypes...)); n != 5 {
		t.Fatalf("legacy rows read: %d", n)
	}
	items, err := bulkItemsFromFlags("", true, f.db, " 9000001 ", "tags", nil, []string{"linen apron"}, nil, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	_, err = bulkItemsFromFlags("", true, nil, "", "tags", nil, []string{"linen apron"}, nil, 0)
	if err == nil || !strings.Contains(err.Error(), shopxNoteShopUnset) || !strings.Contains(err.Error(), "shop-scoped") {
		t.Fatal(err)
	}
	// Exercise the command preview as well as its selector.
	preview, _, previewErr := s2Run(t, "listings", "bulk-edit", "--all", "--field", "tags", "--add", "linen apron", "--db", f.path, "--json")
	if previewErr != nil || !strings.Contains(preview, "1000000001") || strings.Contains(preview, "1000000002") || strings.Contains(preview, "1000000003") || strings.Contains(preview, "1000000004") {
		t.Fatalf("preview=%s err=%v", preview, previewErr)
	}
	out, _, err := shopxRun(t, f, "listings", "expiring")
	if err != nil || strings.Contains(out, "1000000002") || strings.Contains(out, "1000000009") || !strings.Contains(out, "1000000001") {
		t.Fatalf("%s %v", out, err)
	}
}

func TestEtsyShopResolverRefreshConfig(t *testing.T) {
	s2FastThrottle(t, 1)
	calls := 0
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
		if r.URL.Path != "/listings/batch/inventory" {
			t.Errorf("unexpected request %s", r.URL.Path)
			return false
		}
		calls++
		if ids := r.URL.Query().Get("listing_ids"); ids != "1000000001" {
			t.Errorf("ids=%s", ids)
		}
		_, _ = w.Write([]byte(`{"results":[]}`))
		return true
	})
	etsyConfigShop(t)
	path := s2FixtureDB(t, func(db *store.Store) {
		_, _, err := db.UpsertBatch("listings", []json.RawMessage{
			json.RawMessage(`{"listing_id":1000000001,"shop_id":9000001,"state":"active"}`),
			json.RawMessage(`{"listing_id":1000000002,"shop_id":555001,"state":"active"}`),
			json.RawMessage(`{"listing_id":1000000003,"shop_id":9000001}`),
			json.RawMessage(`{"listing_id":1000000004,"shop_id":9000001,"state":""}`),
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	_, _, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json", "--no-cache", "--rate-limit", "1000")
	if err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestEtsyShopResolverVariationPreview(t *testing.T) {
	for _, variation := range []bool{false, true} {
		t.Run(fmt.Sprint(variation), func(t *testing.T) {
			f := shopxNewFixture(t)
			inventory := map[string]any{"products": []any{
				map[string]any{"product_id": 1, "offerings": []any{map[string]any{"offering_id": 11, "price": shopxMoneyObj(2500, "USD"), "quantity": 5, "is_enabled": true}}},
				map[string]any{"product_id": 2, "offerings": []any{map[string]any{"offering_id": 12, "price": shopxMoneyObj(2500, "USD"), "quantity": 5, "is_enabled": true}}},
			}}
			if variation {
				for _, key := range []string{"price_on_property", "quantity_on_property", "sku_on_property", "readiness_state_on_property"} {
					inventory[key] = []int{513}
				}
			}
			listing := map[string]any{"listing_id": 1000000001, "shop_id": 555001, "state": "active", "price": shopxMoneyObj(2500, "USD"), "inventory": inventory}
			raw, err := json.Marshal(listing)
			if err != nil {
				t.Fatal(err)
			}
			f.put("listings", listing)
			etsyReplaceEmbeddedListingRows(f.db, []json.RawMessage{raw})
			reconstructed := s2StoredInventory(f.db, 1000000001)
			if variation {
				for _, key := range []string{"price_on_property", "quantity_on_property", "sku_on_property", "readiness_state_on_property"} {
					if !strings.Contains(reconstructed, key) {
						t.Fatalf("missing %s: %s", key, reconstructed)
					}
				}
			}
			out, _, _ := shopxRun(t, f, "listings", "bulk-edit", "--listings", "1000000001", "--field", "price", "--set", "30")
			if strings.Contains(out, "per-variation prices") != variation {
				t.Fatalf("variation=%v: %s", variation, out)
			}
		})
	}
}

func TestEtsyShopResolverBulkUnsetMakesNoRequests(t *testing.T) {
	calls := 0
	s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool { calls++; return false })
	t.Setenv("ETSY_SHOP_ID", "")
	t.Setenv("ETSY_CONFIG", filepath.Join(t.TempDir(), "absent.toml"))
	path := s2FixtureDB(t, func(db *store.Store) {})
	_, _, err := s2Run(t, "listings", "bulk-edit", "--all", "--field", "tags", "--add", "linen apron", "--db", path, "--json")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "ETSY_SHOP_ID") || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
