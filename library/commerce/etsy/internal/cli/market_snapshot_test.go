// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

// TestNovelMarketSnapshotHelpWires smoke-tests that the market snapshot command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMarketSnapshotHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "snapshot", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market snapshot --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "snapshot"} {
		if !strings.Contains(help, want) {
			t.Fatalf("market snapshot --help missing %q in output:\n%s", want, help)
		}
	}
}

func mktSnapArgs(extra ...string) []string {
	return append([]string{"market", "snapshot", "--rate-limit", "0", "--no-cache"}, extra...)
}

func TestMarketSnapshotTermsFileDryRun(t *testing.T) {
	for _, tc := range []struct {
		name     string
		readable bool
		dryRun   bool
	}{
		{"missing dry run", false, true},
		{"missing live", false, false},
		{"readable dry run", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mktTestEnv(t)
			f := newMktFakeEtsy(t)
			mktPointAtFake(t, f)
			p := filepath.Join(t.TempDir(), "terms.txt")
			if tc.readable {
				if err := os.WriteFile(p, []byte("linen apron\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := mktSnapArgs("--terms", p, "--pages", "2", "--max-calls", "200")
			if tc.dryRun {
				args = append(args, "--dry-run")
			}
			out, err := mktRunCLI(t, args...)
			if !tc.dryRun {
				if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "read --terms") {
					t.Fatalf("want terms usage error (exit 2), got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("dry-run: %v", err)
				}
				result := mktDecode(t, out)
				if result["dry_run"] != true || result["success"] != true {
					t.Fatalf("want successful dry-run, got %v", result)
				}
				if tc.readable {
					plan, ok := result["plan"].(map[string]any)
					calls, _ := plan["planned_calls"].(float64)
					if !ok || calls <= 0 {
						t.Fatalf("want full plan with positive calls, got %v", result)
					}
				} else if note, _ := result["note"].(string); !strings.Contains(note, "not readable") || result["terms_file"] != p || result["action"] != "market snapshot" {
					t.Fatalf("want unreadable terms note, got %v", result)
				}
			}
			if f.pageGets != 0 || f.shopTotal() != 0 || f.revGets != 0 {
				t.Fatal("terms-file validation/dry-run made HTTP calls")
			}
		})
	}
}

func TestMarketSnapshotStoresRowsAndDedupesShops(t *testing.T) {
	mktTestEnv(t)
	f := newMktFakeEtsy(t)
	mktPointAtFake(t, f)

	out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--term", "ceramic mug", "--pages", "1")...)
	if err != nil {
		t.Fatalf("snapshot: %v\n%s", err, out)
	}
	t.Logf("snapshot output:\n%s", out)
	sum := mktDecode(t, out)
	if sum["status"] != "complete" {
		t.Fatalf("status = %v, want complete", sum["status"])
	}
	if f.pageGets != 2 {
		t.Errorf("page GETs = %d, want 2 (2 terms x 1 page)", f.pageGets)
	}
	// Shops 5001 (two listings), 5002, 5003: exactly one GET each.
	if f.shopTotal() != 3 || f.shopGets[5001] != 1 {
		t.Errorf("shop GETs = %v, want 3 total with shop 5001 fetched once", f.shopGets)
	}
	if calls := sum["calls_used"].(float64); calls != 3 {
		t.Errorf("calls_used = %v, want 3 (2 pages + 1 batch)", calls)
	}

	db := mktOpenFixtureStore(t)
	defer db.Close()
	var n int
	_ = db.DB().QueryRow(`SELECT count(*) FROM market_results`).Scan(&n)
	if n != 4 {
		t.Fatalf("market_results rows = %d, want 4", n)
	}
	var amount, divisor, count int64
	var cur string
	if err := db.DB().QueryRow(`SELECT price_amount, price_divisor, currency, result_count FROM market_results WHERE listing_id = 1000000001`).Scan(&amount, &divisor, &cur, &count); err != nil {
		t.Fatal(err)
	}
	if amount != 2400 || divisor != 100 || cur != "USD" || count != 321 {
		t.Errorf("price/result_count = %d/%d %s count=%d", amount, divisor, cur, count)
	}
	var views sql.NullInt64
	_ = db.DB().QueryRow(`SELECT views FROM market_results WHERE listing_id = 1000000002`).Scan(&views)
	if views.Valid {
		t.Errorf("absent views stored as %d, want NULL", views.Int64)
	}
	_ = db.DB().QueryRow(`SELECT views FROM market_results WHERE listing_id = 1000000001`).Scan(&views)
	if !views.Valid || views.Int64 != 40 {
		t.Errorf("present views = %v, want 40", views)
	}
	var pos int
	_ = db.DB().QueryRow(`SELECT position FROM market_results WHERE listing_id = 1000000002`).Scan(&pos)
	if pos != 2 {
		t.Errorf("position = %d, want 2", pos)
	}
}

func TestMarketSnapshotMaxCallsPartialThenResume(t *testing.T) {
	mktTestEnv(t)
	f := newMktFakeEtsy(t)
	mktPointAtFake(t, f)

	out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--term", "ceramic mug", "--max-calls", "2")...)
	if err != nil {
		t.Fatalf("first run: %v\n%s", err, out)
	}
	t.Logf("partial output:\n%s", out)
	sum := mktDecode(t, out)
	if sum["status"] != "partial" || sum["calls_used"].(float64) != 2 {
		t.Fatalf("want partial with 3 calls, got %v", sum)
	}
	if !strings.Contains(sum["resume_hint"].(string), "--resume 1") {
		t.Errorf("resume hint = %v", sum["resume_hint"])
	}
	if f.pageGets != 2 || f.shopTotal() != 0 {
		t.Fatalf("after partial: pages=%d shops=%d, want 2 and 1", f.pageGets, f.shopTotal())
	}

	out, err = mktRunCLI(t, mktSnapArgs("--resume", "1")...)
	if err != nil {
		t.Fatalf("resume: %v\n%s", err, out)
	}
	t.Logf("resume output:\n%s", out)
	sum = mktDecode(t, out)
	if sum["status"] != "complete" {
		t.Fatalf("resume status = %v", sum["status"])
	}
	if f.pageGets != 2 {
		t.Errorf("resume refetched pages: page GETs = %d, want still 2", f.pageGets)
	}
	if f.shopTotal() != 3 {
		t.Errorf("shop GETs = %d, want 3 (no shop refetched)", f.shopTotal())
	}
	if sum["calls_used"].(float64) != 3 {
		t.Errorf("cumulative calls_used = %v, want 5", sum["calls_used"])
	}
}

func TestMarketSnapshotBudgetRefusal(t *testing.T) {
	mktTestEnv(t)
	f := newMktFakeEtsy(t)
	mktPointAtFake(t, f)
	h := http.Header{}
	h.Set("x-remaining-today", "3")
	h.Set("x-limit-per-day", "5000")
	client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))

	_, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--term", "ceramic mug")...)
	if err == nil || !strings.Contains(err.Error(), "remaining today") {
		t.Fatalf("want budget refusal, got %v", err)
	}
	if f.pageGets != 0 {
		t.Errorf("budget refusal still made %d calls", f.pageGets)
	}
	if _, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--term", "ceramic mug", "--ignore-budget")...); err != nil {
		t.Fatalf("--ignore-budget should bypass the check: %v", err)
	}
}

func TestMarketSnapshotStoresNoReviewText(t *testing.T) {
	mktTestEnv(t)
	f := newMktFakeEtsy(t)
	mktPointAtFake(t, f)
	out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--with-reviews", "--review-window", "12mo")...)
	if err != nil {
		t.Fatalf("snapshot: %v\n%s", err, out)
	}
	if f.revGets != 2 {
		t.Errorf("review GETs = %d, want 2 (two listings)", f.revGets)
	}
	if f.revSince == "" || f.revSince == "0" {
		t.Errorf("min_created not sent: %q", f.revSince)
	}
	db := mktOpenFixtureStore(t)
	defer db.Close()
	cols := mktCols(t, db.DB(), "market_reviews")
	if strings.Join(cols, ",") != "snapshot_id,listing_id,created_ts,rating" {
		t.Errorf("market_reviews columns = %v; must be timestamp and rating only", cols)
	}
	var n int
	_ = db.DB().QueryRow(`SELECT count(*) FROM market_reviews`).Scan(&n)
	if n != 4 {
		t.Errorf("review rows = %d, want 4", n)
	}
	// No table in the store may contain the review text.
	var hits int
	for _, tbl := range []string{"market_snapshots", "market_results", "market_shops", "market_reviews"} {
		rows, _ := db.DB().Query(`SELECT * FROM ` + tbl)
		cs, _ := rows.Columns()
		vals := make([]any, len(cs))
		ptrs := make([]any, len(cs))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		for rows.Next() {
			_ = rows.Scan(ptrs...)
			if strings.Contains(fmt.Sprint(vals...), "PRIVATE") {
				hits++
			}
		}
		rows.Close()
	}
	if hits != 0 {
		t.Errorf("review text leaked into %d stored rows", hits)
	}
}

func TestMarketSnapshotDryRunAndDogfoodCurtail(t *testing.T) {
	mktTestEnv(t)
	f := newMktFakeEtsy(t)
	mktPointAtFake(t, f)
	out, err := mktRunCLI(t, "market", "snapshot", "--term", "linen apron", "--term", "ceramic mug", "--pages", "2", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	t.Logf("dry-run output:\n%s", out)
	plan := mktDecode(t, out)["plan"].(map[string]any)
	if plan["page_calls"].(float64) != 4 || plan["planned_calls"].(float64) != 8 {
		t.Errorf("plan = %v", plan)
	}
	if f.pageGets != 0 {
		t.Error("dry-run made API calls")
	}

	t.Setenv(cliutil.DogfoodEnvVar, "1")
	if _, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--term", "ceramic mug", "--pages", "3", "--with-reviews")...); err != nil {
		t.Fatal(err)
	}
	if f.pageGets != 1 || f.revGets != 0 {
		t.Errorf("dogfood curtail: pages=%d reviews=%d, want 1 page and no reviews", f.pageGets, f.revGets)
	}
}

func TestMarketSnapshotRequiresTerm(t *testing.T) {
	mktTestEnv(t)
	if _, err := mktRunCLI(t, "market", "snapshot"); err == nil {
		t.Fatal("expected a usage error without any term")
	}
	if _, err := mktRunCLI(t, "market", "snapshot", "--term", "x", "--pages", "9"); err == nil {
		t.Fatal("expected a usage error for --pages 9")
	}
}

func TestMarketSnapshotTermsFile(t *testing.T) {
	mktTestEnv(t)
	f := newMktFakeEtsy(t)
	mktPointAtFake(t, f)
	p := filepath.Join(t.TempDir(), "terms.txt")
	if err := os.WriteFile(p, []byte("# demo terms\nlinen apron\n\nceramic mug # inline\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := mktRunCLI(t, mktSnapArgs("--terms", p)...); err != nil {
		t.Fatal(err)
	}
	if f.pageGets != 2 {
		t.Errorf("page GETs = %d, want 2 from the terms file", f.pageGets)
	}
}

func TestMarketSnapshotPerPagePlan(t *testing.T) {
	mktTestEnv(t)
	for _, tc := range []struct {
		size string
		want float64
	}{{"100", 2}, {"25", 2}} {
		out, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic", "--per-page", tc.size, "--dry-run")...)
		if err != nil {
			t.Fatal(err)
		}
		plan := mktDecode(t, out)["plan"].(map[string]any)
		if plan["page_calls"] != float64(1) || plan["planned_calls"] != tc.want || plan["shop_calls_estimate"] != tc.want-1 || fmt.Sprint(plan["per_page"]) != tc.size {
			t.Fatalf("plan = %v", plan)
		}
	}
	for _, size := range []string{"0", "101"} {
		_, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic", "--per-page", size, "--dry-run")...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("size %s: want usage error, got %v", size, err)
		}
	}
	plan := mktPlanSnapshot([]string{"synthetic"}, 1, 25, true, 40, 200, 0)
	if plan.ReviewCallsEst != 25 {
		t.Fatalf("reviews = %d", plan.ReviewCallsEst)
	}
}

func TestMarketSnapshotPerPageRequestsAndResume(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(fmt.Sprint(resume), func(t *testing.T) {
			mktTestEnv(t)
			var mu sync.Mutex
			var queries []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				queries = append(queries, r.URL.Query().Encode())
				mu.Unlock()
				results := make([]map[string]any, 0)
				if r.URL.Query().Get("offset") == "0" {
					for i := 0; i < 25; i++ {
						results = append(results, map[string]any{"listing_id": i + 1})
					}
				} else {
					results = append(results, map[string]any{"listing_id": 26})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"count": 26, "results": results})
			}))
			defer server.Close()
			t.Setenv("ETSY_BASE_URL", server.URL)
			t.Setenv("ETSY_API_KEY", "fakekey:fakesecret")
			cap := "10"
			if resume {
				cap = "1"
			}
			out, err := mktRunCLI(t, mktSnapArgs("--term", "synthetic", "--pages", "3", "--per-page", "25", "--max-calls", cap)...)
			if err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			if resume {
				if mktDecode(t, out)["status"] != "partial" {
					t.Fatalf("expected partial: %s", out)
				}
				out, err = mktRunCLI(t, mktSnapArgs("--resume", "1")...)
				if err != nil {
					t.Fatalf("%v: %s", err, out)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if len(queries) != 2 || !strings.Contains(queries[0], "limit=25") || !strings.Contains(queries[0], "offset=0") || !strings.Contains(queries[1], "limit=25") || !strings.Contains(queries[1], "offset=25") {
				t.Fatalf("queries: %v", queries)
			}
			db := mktOpenFixtureStore(t)
			defer db.Close()
			var position int
			if err := db.DB().QueryRow("SELECT position FROM market_results WHERE listing_id = 26").Scan(&position); err != nil {
				t.Fatal(err)
			}
			if position != 26 {
				t.Fatalf("position=%d", position)
			}
		})
	}
}

func TestMarketSnapshotRecordsZeroResultPage(t *testing.T) {
	mktTestEnv(t)
	fake := newMktFakeEtsy(t)
	mktPointAtFake(t, fake)
	out, err := mktRunCLI(t, mktSnapArgs("--term", "Juniper Ceramics", "--pages", "1", "--max-calls", "1")...)
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	db := mktOpenFixtureStore(t)
	var page, listings, resultCount int
	if err := db.DB().QueryRow(`SELECT page, listings, result_count FROM market_term_pages WHERE term = 'Juniper Ceramics'`).Scan(&page, &listings, &resultCount); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if page != 1 || listings != 0 || resultCount != 0 {
		t.Fatalf("ZERO_RESULT_COUNT_BROKEN: page=%d listings=%d result_count=%d", page, listings, resultCount)
	}
	out, err = mktRunCLI(t, "market", "demand")
	if err != nil {
		t.Fatal(err)
	}
	if got := mktDemandTermByName(t, out, "Juniper Ceramics"); got["state"] != "no_results" {
		t.Fatalf("term: %v", got)
	}
}

func TestMarketSnapshotResumeReplacesTermPage(t *testing.T) {
	mktTestEnv(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		results := []map[string]any{}
		if requests > 1 {
			results = append(results, map[string]any{"listing_id": 1000000001, "shop_id": 9000001})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"count": len(results), "results": results})
	}))
	defer server.Close()
	db := mktOpenFixtureStore(t)
	defer db.Close()
	snapID := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
	c := client.New(&config.Config{BaseURL: server.URL}, time.Second*5, 0)
	c.NoCache = true
	run := &mktRun{ctx: context.Background(), c: c, db: db.DB(), snap: &mktSnapshot{ID: snapID, SortOn: "score"}, perPage: 25}
	for want := 0; want <= 1; want++ {
		n, err := run.mktFetchPage("linen apron", 1)
		if err != nil || n != want {
			t.Fatalf("fetch n=%d err=%v", n, err)
		}
		var count, listings, resultCount int
		if err := db.DB().QueryRow(`SELECT COUNT(*), MAX(listings), MAX(result_count) FROM market_term_pages WHERE snapshot_id=? AND term='linen apron' AND page=1`, snapID).Scan(&count, &listings, &resultCount); err != nil {
			t.Fatal(err)
		}
		if count != 1 || listings != want || resultCount != want {
			t.Fatalf("count=%d listings=%d result_count=%d", count, listings, resultCount)
		}
	}
	if requests != 2 {
		t.Fatalf("requests=%d", requests)
	}
}

func TestMarketSnapshotErrorSummaryEnvelope(t *testing.T) {
	mktTestEnv(t)
	f := newMktFakeEtsy(t)
	defer f.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "25" {
			http.Error(w, "synthetic page failure", http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/listings/active" {
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 26, "results": func() []any {
				rows := []any{}
				for i := 0; i < 25; i++ {
					rows = append(rows, map[string]any{"listing_id": 1000000001 + i, "title": "linen apron"})
				}
				return rows
			}()})
			return
		}
		f.Config.Handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	mktPointAtFake(t, f)
	t.Setenv("ETSY_BASE_URL", server.URL)
	out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "2", "--per-page", "25", "--max-calls", "20")...)
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("ERROR_SUMMARY_ENVELOPE_BROKEN exit=%d err=%v out=%s", ExitCode(err), err, out)
	}
	m := mktDecode(t, out)
	if _, ok := m["results"]; !ok {
		t.Fatalf("ERROR_SUMMARY_ENVELOPE_BROKEN missing results: %s", out)
	}
	text, ok := m["error"].(string)
	if !ok || text == "" || !mktNotesContain(m, text) {
		t.Fatalf("ERROR_SUMMARY_ENVELOPE_BROKEN missing error note: %s", out)
	}
}

func TestMarketSnapshotRefetchDropsDepartedListings(t *testing.T) {
	mktTestEnv(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		results := []map[string]any{{"listing_id": 1000000002, "shop_id": 9000001}}
		if requests == 1 {
			results = append(results, map[string]any{"listing_id": 1000000001, "shop_id": 9000001})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"count": len(results), "results": results})
	}))
	defer server.Close()
	db := mktOpenFixtureStore(t)
	defer db.Close()
	id := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
	c := client.New(&config.Config{BaseURL: server.URL}, time.Second*5, 0)
	c.NoCache = true
	run := &mktRun{ctx: context.Background(), c: c, db: db.DB(), snap: &mktSnapshot{ID: id, SortOn: "score"}, perPage: 25}
	for i := 0; i < 2; i++ {
		if _, err := run.mktFetchPage("linen apron", 1); err != nil {
			t.Fatal(err)
		}
	}
	var count, departed, listings int
	if err := db.DB().QueryRow(`SELECT COUNT(*), COALESCE(SUM(listing_id=1000000001),0) FROM market_results WHERE snapshot_id=? AND term='linen apron' AND page=1`, id).Scan(&count, &departed); err != nil {
		t.Fatal(err)
	}
	if err := db.DB().QueryRow(`SELECT listings FROM market_term_pages WHERE snapshot_id=? AND term='linen apron' AND page=1`, id).Scan(&listings); err != nil {
		t.Fatal(err)
	}
	if departed != 0 || count != 1 || count != listings {
		t.Fatalf("STALE_RESULTS_REMAIN: count=%d departed=%d listings=%d", count, departed, listings)
	}
}
