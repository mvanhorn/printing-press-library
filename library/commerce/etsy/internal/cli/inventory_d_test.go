package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func dEnv(t *testing.T) {
	t.Helper()
	restore, err := cliutil.SetHomeOverride("")
	s2Must(t, err)
	t.Cleanup(restore)
	testenv.Isolate(t, cliutil.StateDir)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	s2FastThrottle(t, time.Millisecond)
}
func dListings(t *testing.T, n int) string {
	return s2FixtureDB(t, func(db *store.Store) {
		for i := 1; i <= n+5; i++ {
			shop := 9000001
			if i > n {
				shop = 9999999
			}
			s2Upsert(t, db.UpsertListings, fmt.Sprintf(`{"listing_id":%d,"shop_id":%d,"state":"active"}`, 2000000000+i, shop))
		}
		s2Must(t, db.Upsert("listings_inventory", "stale", json.RawMessage(`{"parent_id":2000000001,"product_id":99,"offerings":[]}`)))
	})
}
func dBatch(w http.ResponseWriter, r *http.Request, qty int64) {
	ids := strings.Split(r.URL.Query().Get("listing_ids"), ",")
	results := []any{}
	for _, id := range ids {
		n, _ := strconv.ParseInt(id, 10, 64)
		results = append(results, map[string]any{"listing_id": id, "inventory": map[string]any{"products": []any{map[string]any{"product_id": n + 1, "offerings": []any{map[string]any{"quantity": qty, "is_enabled": true}}}}}})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
}
func TestDBatchRefresh(t *testing.T) {
	dEnv(t)
	var qty atomic.Int64
	qty.Store(2)
	sizes := []int{}
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		if r.URL.Path != "/listings/batch/inventory" {
			t.Errorf("unexpected path %s", r.URL.Path)
			return true
		}
		ids := strings.Split(r.URL.Query().Get("listing_ids"), ",")
		sizes = append(sizes, len(ids))
		for _, id := range ids {
			n, _ := strconv.ParseInt(id, 10, 64)
			if n > 2000000250 {
				t.Errorf("foreign listing %s", id)
			}
		}
		dBatch(w, r, qty.Load())
		return true
	})
	path := dListings(t, 250)
	for run := 0; run < 2; run++ {
		qty.Store(int64(2 - run*2))
		out, _, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--limit", "0", "--json", "--rate-limit", "1000")
		s2Must(t, err)
		var rows []s2LowStockRow
		etsyDecodeLowStock(t, out, &rows)
		if len(rows) != 250 || rows[0].Quantity != qty.Load() {
			t.Fatalf("rows=%d first=%+v", len(rows), rows[0])
		}
	}
	if fmt.Sprint(sizes) != "[100 100 50 100 100 50]" || len(srv.all()) != 6 {
		t.Fatalf("sizes=%v", sizes)
	}
	db, err := store.OpenReadOnly(path)
	s2Must(t, err)
	defer db.Close()
	assertInventoryProductRows(t, db, 250)
	var count int
	s2Must(t, db.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type='listings_inventory' AND json_extract(data, '$.product_id')=99`).Scan(&count))
	if count != 0 {
		t.Fatal("stale row survived")
	}
	s2Must(t, db.ListScan("listings_inventory", func(id string, data json.RawMessage) bool {
		obj, err := s2DecodeObject(data)
		s2Must(t, err)
		if id != s2Str(obj["product_id"])+"\x00"+s2Str(obj["parent_id"]) {
			t.Errorf("bad key %q", id)
		}
		return true
	}))
}
func TestDFallbackAndMissingShop(t *testing.T) {
	dEnv(t)
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		if strings.Contains(r.URL.Query().Get("listing_ids"), "2000000002") {
			w.WriteHeader(404)
			return true
		}
		dBatch(w, r, 2)
		return true
	})
	path := dListings(t, 3)
	_, stderr, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json", "--rate-limit", "1000")
	s2Must(t, err)
	if !strings.Contains(stderr, "1 of 3 inventory reads failed") || len(srv.all()) != 5 {
		t.Fatalf("stderr=%s requests=%v", stderr, srv.all())
	}
	t.Setenv("ETSY_SHOP_ID", "")
	_, stderr, err = s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json")
	s2Must(t, err)
	if !strings.Contains(stderr, "ETSY_SHOP_ID not set") || len(srv.all()) != 5 {
		t.Fatal(stderr)
	}
}
func TestDBatchBudget(t *testing.T) {
	for _, n := range []int{150, 250} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			dEnv(t)
			srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool { dBatch(w, r, 1); return true })
			h := make(http.Header)
			h.Set("x-limit-per-day", "20")
			h.Set("x-remaining-today", "2")
			client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
			path := dListings(t, n)
			_, _, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json", "--rate-limit", "1000")
			if n == 150 {
				s2Must(t, err)
				if len(srv.all()) != 2 {
					t.Fatal("expected two batches")
				}
			} else {
				if ExitCode(err) != 7 || len(srv.all()) != 0 {
					t.Fatalf("err=%v requests=%d", err, len(srv.all()))
				}
				_, _, err = s2Run(t, "inventory", "low-stock", "--refresh", "--ignore-budget", "--db", path, "--json", "--rate-limit", "1000")
				s2Must(t, err)
				if len(srv.all()) != 3 {
					t.Fatal("override batches")
				}
			}
		})
	}
}
func TestDSetQtyLiveRead(t *testing.T) {
	dEnv(t)
	var sold atomic.Bool
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
		if r.Method == "GET" {
			inv := s2Inventory
			if sold.Load() {
				inv = strings.ReplaceAll(inv, `"quantity":2`, `"quantity":1`)
			}
			fmt.Fprint(w, inv)
			return true
		}
		if r.Method == "PUT" {
			obj, decodeErr := s2DecodeObject([]byte(body))
			s2Must(t, decodeErr)
			products := obj["products"].([]any)
			for _, p := range products {
				p := p.(map[string]any)
				sku := s2Str(p["sku"])
				for _, o := range p["offerings"].([]any) {
					q := s2Int(o.(map[string]any)["quantity"])
					if sku == "MUG-SPECKLE-L" && q != 1 {
						t.Errorf("sale overwritten: %s", body)
					}
					if sku == "MUG-SPECKLE-S" && q != 9 {
						t.Errorf("target unchanged: %s", body)
					}
				}
			}
			fmt.Fprint(w, `{}`)
			return true
		}
		return false
	})
	args := []string{"inventory", "set-qty", "1000000001", "--product", "4000000001", "--qty", "9", "--json"}
	_, _, err := s2Run(t, args...)
	s2Must(t, err)
	sold.Store(true)
	_, _, err = s2Run(t, append(args, "--yes")...)
	s2Must(t, err)
	if srv.count("GET", "/listings/1000000001/inventory") != 2 {
		t.Fatal("read used cache")
	}
}

func TestDSnapshotsLive(t *testing.T) {
	for _, mode := range []string{"snapshot", "bulk"} {
		t.Run(mode, func(t *testing.T) {
			dEnv(t)
			var changed atomic.Bool
			base := s2ShopHandler(nil)
			srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
				if r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/inventory") {
					inv := s2Inventory
					if changed.Load() {
						inv = strings.ReplaceAll(inv, `"quantity":2`, `"quantity":1`)
					}
					fmt.Fprint(w, inv)
					return true
				}
				return base(w, r, body)
			})
			path := s2SeedShop(t)
			// Prime the ordinary response cache, so either command's missing bypass
			// independently replays stale inventory.
			flags := rootFlags{timeout: time.Second, rateLimit: 1000}
			c, err := flags.newClient()
			s2Must(t, err)
			_, err = c.Get(context.Background(), "/listings/1000000001/inventory", nil)
			s2Must(t, err)
			changed.Store(true)
			if mode == "snapshot" {
				_, _, err = s2Run(t, "listings", "snapshot", "1000000001", "--db", path, "--json")
			} else {
				_, _, err = s2Run(t, "listings", "bulk-edit", "--listings", "1000000001", "--field", "title", "--set", "Fresh title", "--apply", "--yes", "--db", path, "--json")
			}
			s2Must(t, err)
			db, err := store.OpenReadOnly(path)
			s2Must(t, err)
			defer db.Close()
			rows, err := db.ListListingSnapshots(1000000001, 10, true)
			s2Must(t, err)
			if len(rows) != 1 || !strings.Contains(rows[0].InventoryJSON, `"quantity":1`) {
				t.Fatalf("snapshots=%+v", rows)
			}
			if srv.count("GET", "/listings/1000000001/inventory") != 2 {
				t.Fatal("snapshot read used cache")
			}
		})
	}
}

func TestDBatchDailyQuotaPreservesRows(t *testing.T) {
	dEnv(t)
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, _ string) bool {
		if r.URL.Path != "/listings/batch/inventory" {
			t.Errorf("fallback request: %s", r.URL.Path)
		}
		w.Header().Set("x-remaining-today", "0")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":"daily quota"}`)
		return true
	})
	path := dListings(t, 3)
	_, _, err := s2Run(t, "inventory", "low-stock", "--refresh", "--db", path, "--json", "--rate-limit", "1000")
	if ExitCode(err) != 7 || len(srv.all()) != 1 {
		t.Fatalf("err=%v calls=%v", err, srv.all())
	}
	db, err := store.OpenReadOnly(path)
	s2Must(t, err)
	defer db.Close()
	var n int
	s2Must(t, db.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type='listings_inventory' AND json_extract(data, '$.product_id')=99`).Scan(&n))
	if n != 1 {
		t.Fatal("quota wiped previous inventory")
	}
}

func TestDLowStockShopPartition(t *testing.T) {
	dEnv(t)
	t.Setenv("ETSY_SHOP_ID", "9000001")
	path := s2FixtureDB(t, func(db *store.Store) {
		for i, shop := range []int{9000001, 9999999} {
			id := 2000000001 + i
			s2Upsert(t, db.UpsertListings, fmt.Sprintf(`{"listing_id":%d,"shop_id":%d}`, id, shop))
			s2Must(t, db.Upsert("listing_inventory", fmt.Sprint(id), json.RawMessage(fmt.Sprintf(`{"listing_id":%d,"products":[{"product_id":%d,"offerings":[{"quantity":0}]}]}`, id, id))))
		}
	})
	out, _, err := s2Run(t, "inventory", "low-stock", "--db", path, "--json")
	s2Must(t, err)
	var env struct {
		Results []s2LowStockRow `json:"results"`
		Notes   []string        `json:"notes"`
	}
	s2Decode(t, out, &env)
	if len(env.Results) != 1 || env.Results[0].ListingID != 2000000001 || !strings.Contains(strings.Join(env.Notes, " "), "1 listing(s) from other shops ignored") {
		t.Fatal(out)
	}
	out, _, err = s2Run(t, "inventory", "audit", "--db", path, "--json")
	s2Must(t, err)
	if strings.Contains(out, `"listing_id":2000000002`) || !strings.Contains(out, `"zero_quantity"`) {
		t.Fatal(out)
	}
}
