// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelMarketShopsHelpWires smoke-tests that the market shops command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMarketShopsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "shops", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market shops --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "shops"} {
		if !strings.Contains(help, want) {
			t.Fatalf("market shops --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestMarketShopsNewVersusEstablishedCrowdOut(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	d := db.DB()
	snap := mktFxSnapshot(t, d, time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
	old := time.Now().AddDate(-5, 0, 0).Unix()
	recent := time.Now().AddDate(0, -3, 0).Unix()
	// pos1: established (old, 900 sales); pos2: new by age; pos3: new by sales (old but 20 sales);
	// pos4: unknown shop (no lookup row); pos5: established but outside --top 4.
	for i, sh := range []int64{5001, 5002, 5003, 5004, 5005} {
		mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: i + 1, id: int64(1000000001 + i), shop: sh, title: "Apron", price: 2400})
	}
	mktFxShop(t, d, snap, 5001, 900, 40, old)
	mktFxShop(t, d, snap, 5002, 900, 5, recent)
	mktFxShop(t, d, snap, 5003, 20, 5, old)
	mktFxShop(t, d, snap, 5005, 900, 5, old)
	db.Close()

	out, err := mktRunCLI(t, "market", "shops", "--top", "4")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", out)
	m := mktDecode(t, out)
	co := m["crowd_out"].(map[string]any)
	if co["top_n_slots"].(float64) != 4 || co["new_shop_slots"].(float64) != 2 || co["established_slots"].(float64) != 1 || co["unknown_slots"].(float64) != 1 {
		t.Errorf("crowd_out = %v, want 4 slots: 2 new, 1 established, 1 unknown", co)
	}
	if co["new_shop_share"].(float64) != 0.5 {
		t.Errorf("new_shop_share = %v, want 0.5", co["new_shop_share"])
	}
	byShop := map[float64]map[string]any{}
	for _, r := range m["results"].([]any) {
		byShop[r.(map[string]any)["shop_id"].(float64)] = r.(map[string]any)
	}
	if byShop[5001]["new_shop"] != false || byShop[5002]["new_shop"] != true || byShop[5003]["new_shop"] != true {
		t.Errorf("new_shop flags wrong: %v", byShop)
	}
	if byShop[5004]["new_shop"] != nil || byShop[5004]["transaction_sold_count"] != nil {
		t.Errorf("unknown shop must report null, not zero: %v", byShop[5004])
	}
	if byShop[5005]["top_n_appearances"].(float64) != 0 || byShop[5005]["appearances"].(float64) != 1 {
		t.Errorf("shop 5005 appearances = %v", byShop[5005])
	}
	// Loosening the thresholds flips the verdicts.
	out, err = mktRunCLI(t, "market", "shops", "--top", "4", "--new-shop-age", "10y", "--new-shop-sales", "1000")
	if err != nil {
		t.Fatal(err)
	}
	if co := mktDecode(t, out)["crowd_out"].(map[string]any); co["new_shop_slots"].(float64) != 3 {
		t.Errorf("with loose thresholds crowd_out = %v, want 3 new slots", co)
	}
}

func TestMarketShopsEmptyStoreExitsZero(t *testing.T) {
	mktTestEnv(t)
	out, err := mktRunCLI(t, "market", "shops")
	if err != nil {
		t.Fatal(err)
	}
	if len(mktDecode(t, out)["results"].([]any)) != 0 {
		t.Errorf("want empty results: %s", out)
	}
}

// Exercise the actual snapshot fetch paths, persistence, and report together.
func TestMarketShopNamesSnapshot(t *testing.T) {
	for _, tc := range []struct {
		label    string
		fallback bool
		status   int
		name     *string
		want     any
	}{
		{"batch", false, 200, mktNamePointer("  Juniper Ceramics  "), "Juniper Ceramics"},
		{"fallback", true, 200, mktNamePointer("Juniper Ceramics"), "Juniper Ceramics"},
		{"empty", false, 200, mktNamePointer(""), nil},
		{"whitespace", true, 200, mktNamePointer(" \t "), nil},
		{"missing", false, 200, nil, nil},
		{"not_found", true, 404, nil, nil},
		{"forbidden", true, 403, nil, nil},
	} {
		t.Run(tc.label, func(t *testing.T) {
			mktTestEnv(t)
			fake := newMktFakeEtsy(t)
			batchCalls, shopCalls := 0, 0
			fake.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				shop := map[string]any{"shop_id": 9000001, "transaction_sold_count": 10}
				if tc.name != nil {
					shop["shop_name"] = *tc.name
				}
				switch r.URL.Path {
				case "/listings/active":
					_, _ = w.Write([]byte(`{"count":1,"results":[{"listing_id":1000000001,"shop_id":9000001,"title":"linen apron"}]}`))
				case "/listings/batch":
					batchCalls++
					if tc.fallback {
						w.WriteHeader(404)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"listing_id": 1000000001, "shop_id": 9000001, "shop": shop}}})
				case "/shops/9000001":
					shopCalls++
					w.WriteHeader(tc.status)
					if tc.status == 200 {
						_ = json.NewEncoder(w).Encode(shop)
					}
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
					w.WriteHeader(404)
				}
			})
			mktPointAtFake(t, fake)
			out, err := mktRunCLI(t, mktSnapArgs("--term", "linen apron", "--pages", "1", "--max-calls", "10")...)
			if err != nil {
				t.Fatalf("snapshot: %v\n%s", err, out)
			}
			if mktDecode(t, out)["status"] != "complete" {
				t.Fatalf("snapshot: %s", out)
			}
			if batchCalls == 0 || (tc.fallback && shopCalls != 1) || (!tc.fallback && shopCalls != 0) {
				t.Fatalf("batch=%d shop=%d", batchCalls, shopCalls)
			}
			db := mktOpenFixtureStore(t)
			var name sql.NullString
			err = db.DB().QueryRow(`SELECT shop_name FROM market_shops WHERE shop_id=9000001`).Scan(&name)
			db.Close()
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == nil {
				if name.Valid {
					t.Fatalf("stored name=%v, want NULL", name)
				}
			} else if !name.Valid || name.String != tc.want {
				t.Fatalf("stored name=%v, want %v", name, tc.want)
			}
			mktAssertShopName(t, tc.want)
		})
	}
}

func mktNamePointer(s string) *string { return &s }

func mktAssertShopName(t *testing.T, want any) {
	t.Helper()
	out, err := mktRunCLI(t, "market", "shops")
	if err != nil {
		t.Fatal(err)
	}
	rows := mktDecode(t, out)["results"].([]any)
	if len(rows) != 1 {
		t.Fatalf("rows: %s", out)
	}
	name, exists := rows[0].(map[string]any)["shop_name"]
	if !exists || name != want {
		t.Fatalf("shop_name=%v exists=%v want=%v", name, exists, want)
	}
}

func TestMarketShopNamesLegacyRowsAndSchema(t *testing.T) {
	for _, legacySchema := range []bool{false, true} {
		t.Run(fmt.Sprint(legacySchema), func(t *testing.T) {
			mktTestEnv(t)
			db := mktOpenFixtureStore(t)
			snap := mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
			mktFxResult(t, db.DB(), snap, mktFxListing{term: "linen apron", pos: 1, id: 1000000001, shop: 555001})
			mktFxShop(t, db.DB(), snap, 555001, 10, 3, nil)
			if legacySchema {
				if _, err := db.DB().Exec(`ALTER TABLE market_shops DROP COLUMN shop_name`); err != nil {
					t.Fatal(err)
				}
				shops, err := mktLoadShops(db.DB(), snap)
				if err != nil || shops[555001].Name.Valid {
					t.Fatalf("legacy shops=%v err=%v", shops, err)
				}
			}
			db.Close()
			mktAssertShopName(t, nil)
		})
	}
}
