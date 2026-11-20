// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Synthetic fixtures for the market command tests. Every shop, listing and
// keyword here is invented.

package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// mktTestEnv isolates HOME and clears Etsy credentials/env.
func mktTestEnv(t *testing.T) {
	t.Helper()
	testenv.Isolate(t)
	// The credential-scoped DB name is process-global state that earlier
	// tests may have left set; fixtures must resolve the same store path
	// the command under test will.
	mktResetDBScope()
	t.Cleanup(mktResetDBScope)
	for _, k := range []string{"ETSY_API_KEY", "ETSY_SHARED_SECRET", "ETSY_SHOP_ID", "ETSY_BASE_URL"} {
		t.Setenv(k, "")
	}
}

func mktResetDBScope() {
	defaultDBScopeState.Lock()
	defaultDBScopeState.hash, defaultDBScopeState.legacyHash, defaultDBScopeState.label, defaultDBScopeState.claimSuppressed = "", "", "", false
	defaultDBScopeState.Unlock()
}

// mktOpenFixtureStore opens the (isolated) default store with market tables.
func mktOpenFixtureStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(defaultDBPath(mktCLIName))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.EnsureMarketSchema(db.DB()); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

type mktFxProg struct {
	reviewsDone []int64
}

func mktFxSnapshot(t *testing.T, db *sql.DB, takenAt time.Time, terms []string, pages int, status string, p mktFxProg) int64 {
	t.Helper()
	tj, _ := json.Marshal(terms)
	completedPages := []string{}
	if status == "complete" {
		for _, term := range terms {
			for page := 1; page <= pages; page++ {
				completedPages = append(completedPages, fmt.Sprintf("%s|%d", term, page))
			}
		}
	}
	prog, _ := json.Marshal(mktProg{Pages: completedPages, ReviewsDone: append([]int64{}, p.reviewsDone...)})
	res, err := db.Exec(`INSERT INTO market_snapshots (taken_at, terms_json, pages, sort_on, max_calls, calls_used, status, with_reviews, review_window_days, progress_json)
		VALUES (?,?,?,?,?,?,?,?,?,?)`, takenAt.UTC().Format(time.RFC3339), string(tj), pages, "score", 200, 0, status, 0, 365, string(prog))
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

type mktFxListing struct {
	term     string
	pos      int
	id       int64
	shop     int64
	title    string
	price    int64 // minor units, divisor 100
	favs     any
	views    any
	tags     []string
	taxonomy int64
}

func mktFxResult(t *testing.T, db *sql.DB, snap int64, l mktFxListing) {
	t.Helper()
	tags, _ := json.Marshal(l.tags)
	var shop any
	if l.shop != 0 {
		shop = l.shop
	}
	_, err := db.Exec(`INSERT INTO market_results (snapshot_id, term, page, position, listing_id, shop_id, title, price_amount, price_divisor, currency,
		num_favorers, views, tags_json, taxonomy_id, original_creation_ts, is_digital, result_count) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		snap, l.term, 1, l.pos, l.id, shop, l.title, l.price, 100, "USD", l.favs, l.views, string(tags), l.taxonomy, nil, 0, 500)
	if err != nil {
		t.Fatalf("insert result: %v", err)
	}
}

func mktFxShop(t *testing.T, db *sql.DB, snap, shop int64, sold, active any, created any) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO market_shops (snapshot_id, shop_id, transaction_sold_count, listing_active_count, create_date, review_average, review_count, is_vacation)
		VALUES (?,?,?,?,?,?,?,?)`, snap, shop, sold, active, created, 4.8, 10, 0); err != nil {
		t.Fatalf("insert shop: %v", err)
	}
}

func mktFxReviews(t *testing.T, db *sql.DB, snap, listing int64, n int, at time.Time) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := db.Exec(`INSERT INTO market_reviews (snapshot_id, listing_id, created_ts, rating) VALUES (?,?,?,?)`, snap, listing, at.Unix()-int64(i), 5); err != nil {
			t.Fatalf("insert review: %v", err)
		}
	}
}

// mktRunCLI executes the root command with --json and returns stdout.
func mktRunCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(append(args, "--json"))
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	return out.String(), err
}

func mktDecode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("output is not a JSON object: %v\n%s", err, s)
	}
	return m
}

// mktFakeEtsy is a synthetic Etsy server that counts requests per route.
type mktFakeEtsy struct {
	*httptest.Server
	mu        sync.Mutex
	pageGets  int
	batchGets int
	shopGets  map[int64]int
	revGets   int
	// 'revSince' records the min_created param of the last review call.
	revSince string
}

func (f *mktFakeEtsy) shopTotal() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.shopGets {
		n += c
	}
	return n
}

func newMktFakeEtsy(t *testing.T) *mktFakeEtsy {
	t.Helper()
	f := &mktFakeEtsy{shopGets: map[int64]int{}}
	listing := func(id, shop int64, title string, views any) map[string]any {
		m := map[string]any{
			"listing_id": id, "shop_id": shop, "title": title,
			"price":        map[string]any{"amount": 2400, "divisor": 100, "currency_code": "USD"},
			"num_favorers": 7, "tags": []string{"linen", "handmade"}, "taxonomy_id": 1234,
			"original_creation_timestamp": 1700000000, "listing_type": "physical",
		}
		if views != nil {
			m["views"] = views
		}
		return m
	}
	byTerm := map[string][]map[string]any{
		"linen apron": {listing(1000000001, 5001, "Linen Apron Natural", 40), listing(1000000002, 5002, "Linen Apron Striped", nil)},
		"ceramic mug": {listing(1000000003, 5001, "Ceramic Mug Speckled", 12), listing(1000000004, 5003, "Ceramic Mug Blue", nil)},
	}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.URL.Path == "/listings/active":
			f.pageGets++
			res, knownTerm := byTerm[r.URL.Query().Get("keywords")]
			count := 0
			if knownTerm {
				count = 321
			}
			if r.URL.Query().Get("offset") != "0" {
				res = []map[string]any{}
			}
			if res == nil {
				res = []map[string]any{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"count": count, "results": res})
		case r.URL.Path == "/listings/batch":
			f.batchGets++
			results := []map[string]any{}
			for _, text := range strings.Split(r.URL.Query().Get("listing_ids"), ",") {
				id, _ := strconv.ParseInt(text, 10, 64)
				shop := int64(5001)
				if id == 1000000002 {
					shop = 5002
				}
				if id == 1000000004 {
					shop = 5003
				}
				f.shopGets[shop]++
				results = append(results, map[string]any{"listing_id": id, "shop_id": shop, "shop": map[string]any{"shop_id": shop, "create_date": 1600000000, "transaction_sold_count": 1000 + shop, "listing_active_count": 30, "review_average": 4.9, "review_count": 200, "is_vacation": false}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
		case strings.HasPrefix(r.URL.Path, "/shops/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/shops/"), 10, 64)
			f.shopGets[id]++
			_ = json.NewEncoder(w).Encode(map[string]any{"shop_id": id, "create_date": 1600000000, "transaction_sold_count": 1000 + id, "listing_active_count": 30, "review_average": 4.9, "review_count": 200, "is_vacation": false})
		case strings.HasSuffix(r.URL.Path, "/reviews"):
			f.revGets++
			f.revSince = r.URL.Query().Get("min_created")
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 2, "results": []map[string]any{
				{"created_timestamp": time.Now().Unix() - 100, "rating": 5, "review": "PRIVATE REVIEW TEXT", "buyer_user_id": 99},
				{"created_timestamp": time.Now().Unix() - 200, "rating": 4, "review": "MORE PRIVATE TEXT", "buyer_user_id": 98},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func mktPointAtFake(t *testing.T, f *mktFakeEtsy) {
	t.Helper()
	t.Setenv("ETSY_BASE_URL", f.URL)
	t.Setenv("ETSY_API_KEY", "fakekey:fakesecret")
}

func mktCols(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
	}
	return cols
}
