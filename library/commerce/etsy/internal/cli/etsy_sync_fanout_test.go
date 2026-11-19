package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type etsyFanoutClient struct{ shopScopeSyncClient }

func (c *etsyFanoutClient) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return c.shopScopeSyncClient.Get(ctx, path+"?"+q.Encode(), nil)
}
func TestEtsySyncFanout(t *testing.T) {
	testenv.Isolate(t)
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	defer db.Close()
	second := false
	requests := 0
	listingPages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/shops/9000001/listings" {
			t.Errorf("unexpected fan-out %s", r.URL.Path)
			io.WriteString(w, `{"results":[]}`)
			return
		}
		listingPages++
		if r.URL.Query().Get("includes") != "Images,Inventory" || r.URL.Query().Get("limit") != "100" {
			t.Errorf("params %v", r.URL.Query())
		}
		if r.URL.Query().Get("state") != "active" {
			io.WriteString(w, `{"results":[],"count":0}`)
			return
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		var rows []map[string]any
		for i := offset; i < offset+100 && i < 250; i++ {
			id := 1000000001 + i
			product := 7000000001 + i
			row := map[string]any{"listing_id": id, "state": "active", "title": "Synthetic", "images": []any{map[string]any{"listing_image_id": id + 500, "url_fullxfull": "synthetic"}}}
			if second && i == 0 {
				product = 7000000999
				row["images"] = []any{}
			}
			if !second || i != 1 {
				row["inventory"] = map[string]any{"products": []any{map[string]any{"product_id": product, "sku": "synthetic"}}, "price_on_property": []any{1}}
			}
			rows = append(rows, row)
		}
		json.NewEncoder(w).Encode(map[string]any{"results": rows, "count": 250})
	}))
	defer srv.Close()
	c := &etsyFanoutClient{shopScopeSyncClient{url: srv.URL, shopID: "9000001", client: srv.Client()}}
	run := func() {
		r := syncResource(context.Background(), c, db, "listings", "", true, 0, false, false, nil, nil)
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	run()
	for _, r := range syncDependentResources(context.Background(), c, db, "", true, 0, false, false, nil, nil, nil, 1) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	if requests != 7 || listingPages != 7 {
		t.Fatalf("requests %d pages %d", requests, listingPages)
	}
	for _, typ := range []string{"listings_inventory", "listing_images"} {
		n, e := db.Count(typ)
		if e != nil || n != 250 {
			t.Fatalf("%s %d %v", typ, n, e)
		}
	}
	second = true
	run()
	n, e := db.Count("listings_inventory")
	if e != nil || n != 250 {
		t.Fatalf("inventory %d %v", n, e)
	}
	n, e = db.Count("listing_images")
	if e != nil || n != 249 {
		t.Fatalf("images %d %v", n, e)
	}
	s2Must(t, db.ListScan("listings_inventory", func(id string, raw json.RawMessage) bool {
		var row map[string]any
		json.Unmarshal(raw, &row)
		if _, ok := row["parent_id"].(string); !ok {
			t.Errorf("parent not string: %s", raw)
		}
		if row["parent_id"] != row["listing_id"] {
			t.Errorf("scope %s", raw)
		}
		if row["parent_id"] == "1000000001" && store.BareResourceID(id) != "7000000999" {
			t.Errorf("ghost %s", id)
		}
		return true
	}))
}

func TestEtsySyncDependentPathsAndExplicitOptIn(t *testing.T) {
	testenv.Isolate(t)
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	defer db.Close()
	for typ, raw := range map[string]string{"receipts": `{"receipt_id":1}`, "shipping_profiles": `{"shipping_profile_id":2}`, "listings": `{"listing_id":3}`} {
		_, _, _, e := db.UpsertBatchDetailed(typ, []json.RawMessage{json.RawMessage(raw)})
		s2Must(t, e)
	}
	paths := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths[r.URL.Path]++
		io.WriteString(w, `{"results":[],"count":0}`)
	}))
	defer srv.Close()
	c := &shopScopeSyncClient{url: srv.URL, shopID: "9000001", client: srv.Client()}
	for _, name := range []string{"receipts_listings", "shipping_profiles_destinations", "shops_receipts_payments", "listings_transactions", "listings_inventory"} {
		for _, r := range syncDependentResources(context.Background(), c, db, "", true, 0, false, false, []string{name}, nil, nil, 1) {
			if r.Err != nil {
				t.Fatal(r.Err)
			}
		}
	}
	for _, path := range []string{"/shops/9000001/receipts/1/listings", "/shops/9000001/shipping-profiles/2/destinations", "/shops/9000001/receipts/1/payments", "/shops/9000001/listings/3/transactions", "/listings/3/inventory"} {
		if paths[path] != 1 {
			t.Errorf("%s: %v", path, paths)
		}
	}
	for _, name := range defaultSyncResources() {
		if name == "listings-active" {
			t.Fatal("duplicate active walk")
		}
	}
	for _, name := range []string{"ledger_entries", "receipts", "transactions", "reviews"} {
		if determinePaginationDefaults(name).limit != 100 {
			t.Fatal(fmt.Sprint(name, " limit"))
		}
	}
	for path := range paths {
		if strings.Contains(path, "{shop_id}") {
			t.Fatal(path)
		}
	}
}

// etsy-ext: Receipt listing fan-out is opt-in even when its parent is selected.
func TestEtsyReceiptListingsDefaultSkip(t *testing.T) {
	testenv.Isolate(t)
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	s2Must(t, err)
	defer db.Close()
	for _, id := range []int{1, 2} {
		_, _, err := db.UpsertBatch("receipts", []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"receipt_id":%d}`, id))})
		s2Must(t, err)
	}
	paths := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths[r.URL.Path]++
		io.WriteString(w, `{"results":[],"count":0}`)
	}))
	defer srv.Close()
	c := &shopScopeSyncClient{url: srv.URL, shopID: "9000001", client: srv.Client()}
	for _, filter := range [][]string{nil, {"receipts"}, {"receipts_listings"}} {
		clear(paths)
		for _, result := range syncDependentResources(context.Background(), c, db, "", true, 0, false, false, filter, nil, nil, 1) {
			s2Must(t, result.Err)
		}
		want := 0
		if len(filter) > 0 && filter[0] == "receipts_listings" {
			want = 1
		}
		for _, id := range []int{1, 2} {
			path := fmt.Sprintf("/shops/9000001/receipts/%d/listings", id)
			if paths[path] != want {
				t.Fatalf("filter=%v paths=%v", filter, paths)
			}
		}
	}
	// Only the payment dependent remains in the default receipt estimate.
	if n := estimateSyncCalls(db, nil, nil, 0, false); n != 2 {
		t.Fatalf("default dependent estimate %d", n)
	}
	if n := estimateSyncCalls(db, nil, []string{"receipts"}, 0, false); n != 2 {
		t.Fatalf("default receipt estimate %d", n)
	}
	if n := estimateSyncCalls(db, nil, []string{"receipts_listings"}, 0, false); n != 2 {
		t.Fatalf("explicit estimate %d", n)
	}
}
