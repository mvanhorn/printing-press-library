// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelMarketPriceHelpWires smoke-tests that the market price command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMarketPriceHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "price", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market price --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "price"} {
		if !strings.Contains(help, want) {
			t.Fatalf("market price --help missing %q in output:\n%s", want, help)
		}
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-4 }

func TestMktSpearmanKnownValues(t *testing.T) {
	if r, ok := mktSpearman([]float64{1, 2, 3, 4, 5}, []float64{2, 4, 6, 8, 10}); !ok || r != 1.0 {
		t.Errorf("perfect monotone rho = %v (%v), want 1.0", r, ok)
	}
	if r, ok := mktSpearman([]float64{1, 2, 3, 4, 5}, []float64{9, 7, 5, 3, 1}); !ok || r != -1.0 {
		t.Errorf("perfect inverse rho = %v, want -1.0", r)
	}
	// Hand-computed with a tie: y ranks = 1, 2, 3.5, 5, 3.5; rho = 8/sqrt(95).
	r, ok := mktSpearman([]float64{1, 2, 3, 4, 5}, []float64{5, 6, 7, 8, 7})
	if !ok || !approx(r, 8/math.Sqrt(95)) || !approx(r, 0.8208) {
		t.Errorf("tied rho = %v, want 0.8208", r)
	}
	if _, ok := mktSpearman([]float64{1, 2, 3}, []float64{4, 4, 4}); ok {
		t.Error("constant series must be undefined, not 0")
	}
	if _, ok := mktSpearman([]float64{1}, []float64{1}); ok {
		t.Error("single pair must be undefined")
	}
}

func mktPriceFixture(t *testing.T, nBig, nSmall int) {
	t.Helper()
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	defer db.Close()
	d := db.DB()
	taken := time.Now()
	var done []int64
	for i := 1; i <= nBig+nSmall; i++ {
		done = append(done, int64(1000000000+i))
	}
	snap := mktFxSnapshot(t, d, taken, []string{"linen apron"}, 1, "complete", mktFxProg{reviewsDone: done})
	for i := 1; i <= nBig; i++ { // taxonomy 100: price and reviews rise together (rho = 1)
		id := int64(1000000000 + i)
		mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: i, id: id, shop: int64(5000 + i), title: "Linen Apron " + fmt.Sprint(i), price: int64(1000 + 100*i), taxonomy: 100})
		mktFxReviews(t, d, snap, id, i, taken.Add(-time.Hour))
	}
	for j := 1; j <= nSmall; j++ { // taxonomy 200: small group
		id := int64(1000000000 + nBig + j)
		mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: nBig + j, id: id, shop: int64(6000 + j), title: "Ceramic Mug " + fmt.Sprint(j), price: int64(500 + 50*j), taxonomy: 200})
		mktFxReviews(t, d, snap, id, j, taken.Add(-time.Hour))
	}
}

func TestMarketPriceSpearmanByTaxonomyAndMinGroup(t *testing.T) {
	mktPriceFixture(t, 15, 5)
	out, err := mktRunCLI(t, "market", "price", "--group-by", "taxonomy")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", out)
	groups := map[string]map[string]any{}
	for _, g := range mktDecode(t, out)["results"].([]any) {
		groups[g.(map[string]any)["group"].(string)] = g.(map[string]any)
	}
	big := groups["100"]
	if big["n"].(float64) != 15 || big["spearman_rho"].(float64) != 1.0 {
		t.Errorf("big group = %v, want n=15 rho=1.0", big)
	}
	if big["min"].(float64) != 11 || big["max"].(float64) != 25 || big["median"].(float64) != 18 {
		t.Errorf("price stats = min %v median %v max %v, want 11/18/25", big["min"], big["median"], big["max"])
	}
	small := groups["200"]
	if small["spearman_rho"] != nil || !strings.Contains(small["note"].(string), "min-group") {
		t.Errorf("small group must have null rho and a note: %v", small)
	}
	// Lowering --min-group makes the small group computable.
	out, _ = mktRunCLI(t, "market", "price", "--min-group", "5")
	for _, g := range mktDecode(t, out)["results"].([]any) {
		if g.(map[string]any)["group"] == "200" && g.(map[string]any)["spearman_rho"] != 1.0 {
			t.Errorf("group 200 at min-group 5: %v", g)
		}
	}
}

func TestMarketPriceGroupByClass(t *testing.T) {
	mktPriceFixture(t, 15, 5)
	rules := filepath.Join(t.TempDir(), "classes.yaml")
	_ = os.WriteFile(rules, []byte("targets:\n  - name: aprons\n    include: [apron]\n  - name: mugs\n    include: [mug]\n"), 0o600)
	if _, err := mktRunCLI(t, "market", "price", "--group-by", "class"); err == nil {
		t.Error("--group-by class without --rules must be a usage error")
	}
	out, err := mktRunCLI(t, "market", "price", "--group-by", "class", "--rules", rules)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, g := range mktDecode(t, out)["results"].([]any) {
		got[g.(map[string]any)["group"].(string)] = g.(map[string]any)["n"].(float64)
	}
	if got["aprons"] != 15 || got["mugs"] != 5 {
		t.Errorf("class groups = %v", got)
	}
}

func TestMarketPriceNoDemandDataGivesNullRho(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	d := db.DB()
	snap := mktFxSnapshot(t, d, time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
	for i := 1; i <= 16; i++ {
		mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: i, id: int64(1000000000 + i), shop: int64(5000 + i), title: "Apron", price: int64(1000 + i), taxonomy: 100})
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "price")
	if err != nil {
		t.Fatal(err)
	}
	g := mktDecode(t, out)["results"].([]any)[0].(map[string]any)
	if g["n"].(float64) != 16 || g["spearman_rho"] != nil || !strings.Contains(g["note"].(string), "absent demand is not zero") {
		t.Errorf("no review data: %v", g)
	}
}

func TestMarketPriceEmptyStoreExitsZero(t *testing.T) {
	mktTestEnv(t)
	out, err := mktRunCLI(t, "market", "price")
	if err != nil {
		t.Fatal(err)
	}
	if len(mktDecode(t, out)["results"].([]any)) != 0 {
		t.Errorf("want empty: %s", out)
	}
}

func TestMarketPriceSeparatesCurrenciesAndFilters(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	d := db.DB()
	snap := mktFxSnapshot(t, d, time.Now(), []string{"mixed"}, 1, "complete", mktFxProg{})
	for i, price := range []int64{1000, 2000, 3000, 10000000, 60000000} {
		id := int64(1000000001 + i)
		mktFxResult(t, d, snap, mktFxListing{term: "mixed", pos: i + 1, id: id, price: price, taxonomy: 1062})
		if i >= 3 {
			if _, err := d.Exec("UPDATE market_results SET currency = 'VND' WHERE listing_id = ?", id); err != nil {
				t.Fatal(err)
			}
		}
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "price", "--min-group", "3")
	if err != nil {
		t.Fatal(err)
	}
	groups := mktDecode(t, out)["results"].([]any)
	if len(groups) != 2 {
		t.Fatalf("groups = %v", groups)
	}
	for _, value := range groups {
		g := value.(map[string]any)
		wantMax, wantN := float64(30), float64(3)
		if g["currency"] == "VND" {
			wantMax, wantN = 600000, 2
		}
		if g["group"] != "1062" || g["max"] != wantMax || g["n"] != wantN || g["spearman_rho"] != nil {
			t.Errorf("currency bucket = %v", g)
		}
		if g["currency"] == "VND" && !strings.Contains(g["note"].(string), "group has 2 listings") {
			t.Errorf("min-group was not applied per currency: %v", g)
		}
	}
	out, err = mktRunCLI(t, "market", "price", "--currency", "usd")
	if err != nil {
		t.Fatal(err)
	}
	result := mktDecode(t, out)
	groups = result["results"].([]any)
	if result["excluded_other_currency"] != float64(2) || len(groups) != 1 {
		t.Fatalf("filtered result = %v", result)
	}
	if g := groups[0].(map[string]any); g["currency"] != "USD" || g["max"] != float64(30) {
		t.Errorf("filtered bucket = %v", g)
	}
	out, err = mktRunCLI(t, "market", "price", "--currency", "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if got := mktDecode(t, out); got["excluded_other_currency"] != float64(5) || len(got["results"].([]any)) != 0 {
		t.Errorf("empty currency filter = %v", got)
	}
}
