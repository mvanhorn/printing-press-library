package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Synthetic routes exercise batching independently of the shared small fixture.
func TestMarketDBatches(t *testing.T) {
	for _, tc := range []struct{ n, shops, status, batches int }{{100, 78, 200, 1}, {150, 150, 200, 2}, {4, 4, 400, 1}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			mktTestEnv(t)
			pages, batches, shopGets := 0, 0, 0
			sizes := []int{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/listings/active":
					pages++
					offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
					res := []map[string]any{}
					for i := offset; i < min(offset+100, tc.n); i++ {
						res = append(res, map[string]any{"listing_id": i + 1, "shop_id": 5000 + i%tc.shops})
					}
					json.NewEncoder(w).Encode(map[string]any{"results": res})
				case r.URL.Path == "/listings/batch":
					batches++
					ids := strings.Split(r.URL.Query().Get("listing_ids"), ",")
					sizes = append(sizes, len(ids))
					if tc.status != 200 {
						w.WriteHeader(tc.status)
						return
					}
					if r.URL.Query().Get("includes") != "Shop" {
						t.Error("missing includes")
					}
					res := []map[string]any{}
					for _, id := range ids {
						n, _ := strconv.Atoi(id)
						shop := 5000 + (n-1)%tc.shops
						res = append(res, map[string]any{"listing_id": n, "shop_id": shop, "shop": map[string]any{"transaction_sold_count": shop + 1000}})
					}
					json.NewEncoder(w).Encode(map[string]any{"results": res})
				case strings.HasPrefix(r.URL.Path, "/shops/"):
					shopGets++
					n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/shops/"))
					json.NewEncoder(w).Encode(map[string]any{"transaction_sold_count": n + 1000})
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer srv.Close()
			t.Setenv("ETSY_BASE_URL", srv.URL)
			t.Setenv("ETSY_API_KEY", "fake:key")
			out, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic", "--pages", strconv.Itoa((tc.n+99)/100), "--per-page", "100")...)
			if err != nil {
				t.Fatal(err, out)
			}
			if batches != tc.batches || pages != (tc.n+99)/100 {
				t.Fatalf("pages=%d batches=%d sizes=%v", pages, batches, sizes)
			}
			wantShopGets := 0
			if tc.status == 400 {
				wantShopGets = tc.shops
			}
			if shopGets != wantShopGets {
				t.Fatal(shopGets)
			}
			if tc.n == 150 && (sizes[0] != 100 || sizes[1] != 50) {
				t.Fatal(sizes)
			}
			db := mktOpenFixtureStore(t)
			defer db.Close()
			var n int
			if err := db.DB().QueryRow("SELECT count(*) FROM market_shops WHERE transaction_sold_count=shop_id+1000").Scan(&n); err != nil || n != tc.shops {
				t.Fatal(n, err)
			}
			if mktDecode(t, out)["calls_used"] != float64(pages+batches+shopGets) {
				t.Fatal(out)
			}
		})
	}
}

func TestMarketDPrivacy(t *testing.T) {
	mktTestEnv(t)
	home := t.TempDir()
	for _, key := range []string{"HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME"} {
		t.Setenv(key, home)
	}
	f := newMktFakeEtsy(t)
	mktPointAtFake(t, f)
	if out, err := mktRunCLI(t, "market", "snapshot", "--rate-limit", "0", "--term", "linen apron", "--with-reviews"); err != nil {
		t.Fatal(err, out)
	}
	db := mktOpenFixtureStore(t)
	defer db.Close()
	rows, err := db.DB().Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatal(err)
	}
	tables := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, table := range tables {
		rows, err := db.DB().Query(`SELECT * FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`)
		if err != nil {
			t.Fatal(err)
		}
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		for rows.Next() {
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			for _, v := range values {
				var text string
				switch v := v.(type) {
				case []byte:
					text = string(v)
				default:
					text = fmt.Sprint(v)
				}
				if strings.Contains(text, "PRIVATE") {
					t.Error("private text in table", table)
				}
			}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
	}
	if err := filepath.WalkDir(home, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if bytes.Contains(b, []byte("PRIVATE")) {
				t.Errorf("private text in %s", path)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMarketDReviews(t *testing.T) {
	for _, total := range []int{230, 1200} {
		t.Run(strconv.Itoa(total), func(t *testing.T) {
			mktTestEnv(t)
			offsets := []int{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/listings/active" {
					fmt.Fprint(w, `{"results":[{"listing_id":1}]}`)
					return
				}
				if r.URL.Query().Get("limit") != "100" {
					t.Error("review limit", r.URL.Query())
				}
				off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
				offsets = append(offsets, off)
				res := []map[string]any{}
				for i := off; i < min(off+100, total); i++ {
					res = append(res, map[string]any{"created_timestamp": 1700000000 + i, "rating": 5})
				}
				json.NewEncoder(w).Encode(map[string]any{"count": total, "results": res})
			}))
			defer srv.Close()
			t.Setenv("ETSY_BASE_URL", srv.URL)
			t.Setenv("ETSY_API_KEY", "fake:key")
			out, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic", "--with-reviews", "--per-page", "25")...)
			if err != nil {
				t.Fatal(err, out)
			}
			want := min(total, 1000)
			if len(offsets) != (want+99)/100 {
				t.Fatal(offsets)
			}
			for i, off := range offsets {
				if off != i*100 {
					t.Fatal(offsets)
				}
			}
			db := mktOpenFixtureStore(t)
			defer db.Close()
			var n int
			if err := db.DB().QueryRow("SELECT count(*) FROM market_reviews").Scan(&n); err != nil || n != want {
				t.Fatal(n, err)
			}
			snap, err := mktGetSnapshot(db.DB(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if total == 1200 && (len(snap.Progress.ReviewsCapped) != 1 || snap.Progress.ReviewsCapped[0] != 1 || mktDecode(t, out)["reviews_capped"] != float64(1)) {
				t.Fatal(out, snap.Progress)
			}
			// Simulate interruption after commit and before progress save.
			if _, err := db.DB().Exec(`UPDATE market_snapshots SET status='partial',progress_json=json_set(progress_json,'$.reviews_done',json('[]'))`); err != nil {
				t.Fatal(err)
			}
			out, err = mktRunCLI(t, mktSnapArgs("--resume", "1")...)
			if err != nil {
				t.Fatal(err, out)
			}
			if err := db.DB().QueryRow("SELECT count(*) FROM market_reviews").Scan(&n); err != nil || n != want {
				t.Fatal(n, err)
			}
		})
	}
}

func TestMarketDDecodeAndResume(t *testing.T) {
	for _, mode := range []string{"missing", "empty", "batch500"} {
		t.Run(mode, func(t *testing.T) {
			mktTestEnv(t)
			corrected := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/listings/batch" {
					w.WriteHeader(500)
					return
				}
				if mode == "missing" && !corrected {
					fmt.Fprint(w, `{"count":0}`)
				} else if mode == "batch500" {
					fmt.Fprint(w, `{"results":[{"listing_id":1,"shop_id":1}]}`)
				} else {
					fmt.Fprint(w, `{"count":0,"results":[]}`)
				}
			}))
			defer srv.Close()
			t.Setenv("ETSY_BASE_URL", srv.URL)
			t.Setenv("ETSY_API_KEY", "fake:key")
			out, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic")...)
			sum := mktDecode(t, out)
			if mode == "empty" {
				if err != nil || sum["status"] != "complete" || sum["listings"] != float64(0) {
					t.Fatal(err, out)
				}
				return
			}
			if err == nil || sum["status"] != "partial" || sum["snapshot_id"] == nil || sum["resume_hint"] == nil || sum["error"] == nil {
				t.Fatal(err, out)
			}
			db := mktOpenFixtureStore(t)
			defer db.Close()
			snap, e := mktGetSnapshot(db.DB(), 1)
			if e != nil || snap.Status != "partial" {
				t.Fatal(e, snap)
			}
			if mode == "missing" {
				var n int
				if e := db.DB().QueryRow("SELECT count(*) FROM market_results").Scan(&n); e != nil || n != 0 {
					t.Fatal(n, e)
				}
				corrected = true
				out, err = mktRunCLI(t, mktSnapArgs("--resume", "1")...)
				if err != nil || mktDecode(t, out)["status"] != "complete" {
					t.Fatal(err, out)
				}
			}
		})
	}
}

func TestMarketDReviewTopAndAtomicBudget(t *testing.T) {
	mktTestEnv(t)
	seen := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/listings/active" {
			fmt.Fprint(w, `{"results":[{"listing_id":1},{"listing_id":2},{"listing_id":3},{"listing_id":4}]}`)
			return
		}
		seen = append(seen, r.URL.Path)
		fmt.Fprint(w, `{"count":0,"results":[]}`)
	}))
	defer srv.Close()
	t.Setenv("ETSY_BASE_URL", srv.URL)
	t.Setenv("ETSY_API_KEY", "fake:key")
	out, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic", "--with-reviews", "--review-top", "3", "--max-calls", "2")...)
	if err != nil || mktDecode(t, out)["status"] != "partial" {
		t.Fatal(err, out)
	}
	out, err = mktRunCLI(t, mktSnapArgs("--resume", "1")...)
	if err != nil || len(seen) != 3 {
		t.Fatal(err, out, seen)
	}
	for _, path := range seen {
		if path == "/listings/4/reviews" {
			t.Fatal(seen)
		}
	}
}

func TestMarketDDailyQuota(t *testing.T) {
	started := time.Now()
	mktTestEnv(t)
	seen := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/listings/active" {
			fmt.Fprint(w, `{"results":[{"listing_id":1},{"listing_id":2}]}`)
			return
		}
		seen = append(seen, r.URL.Path)
		w.Header().Set("x-remaining-today", "0")
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"error":"quota"}`)
	}))
	defer srv.Close()
	t.Setenv("ETSY_BASE_URL", srv.URL)
	t.Setenv("ETSY_API_KEY", "fake:key")
	out, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic", "--with-reviews")...)
	if err == nil || mktDecode(t, out)["status"] != "partial" || len(seen) != 1 {
		t.Fatal(err, out, seen)
	}
	summary := mktDecode(t, out)
	if summary["calls_this_run"] != float64(2) || summary["listings"] != float64(2) || summary["reviews"] != float64(0) || summary["resume_hint"] == nil {
		t.Fatalf("missing partial summary: %s", out)
	}
	if time.Since(started) >= 2*time.Second {
		t.Fatal("quota stop took over 2 seconds")
	}
	for _, path := range seen {
		if path != "/listings/1/reviews" {
			t.Fatal("continued after quota", seen)
		}
	}
}

func TestMarketDReviewBudgetRestarts(t *testing.T) {
	mktTestEnv(t)
	offsets := []int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/listings/active" {
			fmt.Fprint(w, `{"results":[{"listing_id":1}]}`)
			return
		}
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		offsets = append(offsets, off)
		res := []map[string]any{}
		for i := off; i < min(off+100, 230); i++ {
			res = append(res, map[string]any{"created_timestamp": 1700000000 + i})
		}
		json.NewEncoder(w).Encode(map[string]any{"count": 230, "results": res})
	}))
	defer srv.Close()
	t.Setenv("ETSY_BASE_URL", srv.URL)
	t.Setenv("ETSY_API_KEY", "fake:key")
	out, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic", "--with-reviews", "--max-calls", "2")...)
	if err != nil || mktDecode(t, out)["status"] != "partial" {
		t.Fatal(err, out)
	}
	db := mktOpenFixtureStore(t)
	defer db.Close()
	var n int
	if err := db.DB().QueryRow("SELECT count(*) FROM market_reviews").Scan(&n); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	snap, err := mktGetSnapshot(db.DB(), 1)
	if err != nil || len(snap.Progress.ReviewsDone) != 0 {
		t.Fatal(err, snap)
	}
	out, err = mktRunCLI(t, mktSnapArgs("--resume", "1")...)
	if err != nil {
		t.Fatal(err, out)
	}
	if fmt.Sprint(offsets) != "[0 0 100 200]" {
		t.Fatal(offsets)
	}
}
