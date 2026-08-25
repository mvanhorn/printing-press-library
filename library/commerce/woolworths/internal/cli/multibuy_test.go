// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command tests. The wiring smoke test is kept; behaviour cases follow.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestMultibuyUnitPriceCellNeverPrintsAFakeZero pins the human table's unit
// price cell. OfferUnitPrice is only filled when the OFFER normalised; in the
// shelf-only fallback it stays 0, and "$0.00/1kg" in that row reads as the
// cheapest offer on the shelf rather than as a figure that does not exist.
func TestMultibuyUnitPriceCellNeverPrintsAFakeZero(t *testing.T) {
	tests := []struct {
		name  string
		offer multibuyOffer
		want  string
	}{
		{
			name:  "offer normalised",
			offer: multibuyOffer{OfferUnitPrice: 30.303, UnitBasis: "1kg"},
			want:  "$30.303/1kg",
		},
		{
			name:  "shelf-only fallback leaves the offer unit price at zero",
			offer: multibuyOffer{OfferUnitPrice: 0, ShelfUnitPrice: 12, UnitBasis: "1kg"},
			want:  "n/a",
		},
		{
			name:  "no unit basis at all",
			offer: multibuyOffer{OfferUnitPrice: 12, UnitBasis: ""},
			want:  "n/a",
		},
		{
			name:  "sub-cent unit price is not rounded away to zero",
			offer: multibuyOffer{OfferUnitPrice: 0.004, UnitBasis: "1EA"},
			want:  "$0.004/1EA",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := multibuyUnitPriceCell(tt.offer); got != tt.want {
				t.Fatalf("multibuyUnitPriceCell(%+v) = %q, want %q", tt.offer, got, tt.want)
			}
		})
	}
}

// TestNovelMultibuyHelpWires smoke-tests that the multibuy command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMultibuyHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"multibuy", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("multibuy --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "multibuy", "--max-scan-pages", "--limit", "--db"} {
		if !strings.Contains(help, want) {
			t.Fatalf("multibuy --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestMultibuyBareInvocationPrintsHelp(t *testing.T) {
	out, err := runCLI(t, "multibuy")
	if err != nil {
		t.Fatalf("bare multibuy should print help and exit 0, got %v", err)
	}
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("bare multibuy did not print usage:\n%s", out)
	}
}

func TestMultibuyDryRunSkipsAllWork(t *testing.T) {
	out, err := runCLI(t, "multibuy", "--dry-run")
	if err != nil {
		t.Fatalf("multibuy --dry-run error = %v", err)
	}
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("multibuy --dry-run did not report the skipped action:\n%s", out)
	}
}

func TestMultibuyMissingPositionalIsUsageError(t *testing.T) {
	out, err := runCLI(t, "multibuy", "--limit", "3")
	if err == nil {
		t.Fatalf("multibuy with flags but no positional should be a usage error; output:\n%s", out)
	}
	if got := exitCodeFor(err); got != 2 {
		t.Fatalf("exit code = %d, want 2 (usage)", got)
	}
}

// TestMultibuyCostsOfferAtRequiredQuantity is the core maths case. The fixture
// carries a "2 for $10" tag on a 165g block whose shelf single is $7.50, plus a
// 500g block that is cheaper per kilo than the offer ever gets. The offer must
// therefore read as a real saving against the single AND as worse than the
// larger single pack.
func TestMultibuyCostsOfferAtRequiredQuantity(t *testing.T) {
	srv := newWowFakeServer(t, &wowFakeServer{
		searchResultsCount: 3,
		pages: map[int][]wowTestTile{
			1: {
				{
					Stockcode: 11, Name: "Blocky Chocolate 165g", Brand: "Blocky",
					Price: 7.5, CupPrice: 4.55, CupMeasure: "100G", PackageSize: "165g",
					IsAvailable: true, IsInStock: true,
					Multibuy: &wowMultibuyData{Quantity: 2, Price: 10, CupTag: "$3.03/100G"},
				},
				{
					Stockcode: 12, Name: "Value Chocolate 500g", Brand: "Value",
					Price: 6, CupPrice: 1.2, CupMeasure: "100G", PackageSize: "500g",
					IsAvailable: true, IsInStock: true,
				},
				{
					Stockcode: 13, Name: "Chocolate Milk 1L", Brand: "Drinky",
					Price: 3, CupPrice: 3, CupMeasure: "1L", PackageSize: "1L",
					IsAvailable: true, IsInStock: true,
				},
			},
		},
	})

	out, err := runCLI(t, "multibuy", "chocolate", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("multibuy error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	offers, _ := env["offers"].([]any)
	if len(offers) != 1 {
		t.Fatalf("offers = %d, want 1 (only one tile carries a multibuy tag)", len(offers))
	}
	offer, _ := offers[0].(map[string]any)

	if got, _ := offer["effective_unit_price"].(float64); got != 5 {
		t.Fatalf("effective_unit_price = %v, want 5 (2 for $10)", got)
	}
	if got, _ := offer["shelf_single_price"].(float64); got != 7.5 {
		t.Fatalf("shelf_single_price = %v, want 7.5", got)
	}
	if got, _ := offer["saving_per_unit_vs_single"].(float64); got != 2.5 {
		t.Fatalf("saving_per_unit_vs_single = %v, want 2.5", got)
	}
	if got, _ := offer["outlay_at_offer_quantity"].(float64); got != 10 {
		t.Fatalf("outlay_at_offer_quantity = %v, want 10; the tag demands both units", got)
	}
	// $5.00 for a 165g block is $30.303 per kilo.
	unit, _ := offer["offer_normalised_unit_price"].(float64)
	if unit < 30.3 || unit > 30.31 {
		t.Fatalf("offer_normalised_unit_price = %v, want ~30.303 ($5.00 over 165g restated per kilo)", unit)
	}
	if basis, _ := offer["unit_basis"].(string); basis != "1kg" {
		t.Fatalf("unit_basis = %q, want 1kg", basis)
	}
	worse, _ := offer["worse_than_single_pack"].(bool)
	if !worse {
		t.Fatalf("worse_than_single_pack = false, but the 500g block is $12/kg against the offer's $30.30/kg")
	}
	pack, _ := offer["cheapest_larger_single_pack"].(map[string]any)
	if pack == nil || pack["stockcode"] != "12" {
		t.Fatalf("cheapest_larger_single_pack = %v, want the 500g Value block (stockcode 12)", pack)
	}
	if got, _ := pack["unit_price"].(float64); got != 12 {
		t.Fatalf("larger pack unit_price = %v, want 12 ($1.20/100G per kilo)", got)
	}
	// The 1L chocolate milk is a different measure kind and must never be the
	// benchmark, even though it is the cheapest tile by raw cup price.
	if pack["stockcode"] == "13" {
		t.Fatalf("a volume tile was used as the benchmark for a mass offer")
	}
	if reason, _ := offer["verdict_reason"].(string); reason == "" {
		t.Fatalf("offer carried no verdict_reason")
	}
	assertNoOversizedPageSize(t, srv.seen())
}

// TestMultibuyGoodOfferIsNotFlagged is the mirror image: when no larger pack
// undercuts the offer, worse_than_single_pack must stay false.
func TestMultibuyGoodOfferIsNotFlagged(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{
		searchResultsCount: 2,
		pages: map[int][]wowTestTile{
			1: {
				{
					Stockcode: 21, Name: "Bargain Block 200g", Brand: "Bargain",
					Price: 4, CupPrice: 2, CupMeasure: "100G", PackageSize: "200g",
					IsAvailable: true, IsInStock: true,
					Multibuy: &wowMultibuyData{Quantity: 2, Price: 5, CupTag: "$1.25/100G"},
				},
				{
					Stockcode: 22, Name: "Pricey Slab 500g", Brand: "Pricey",
					Price: 30, CupPrice: 6, CupMeasure: "100G", PackageSize: "500g",
					IsAvailable: true, IsInStock: true,
				},
			},
		},
	})

	out, err := runCLI(t, "multibuy", "chocolate", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("multibuy error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)
	offers, _ := env["offers"].([]any)
	if len(offers) != 1 {
		t.Fatalf("offers = %d, want 1", len(offers))
	}
	offer, _ := offers[0].(map[string]any)
	if worse, _ := offer["worse_than_single_pack"].(bool); worse {
		t.Fatalf("worse_than_single_pack = true, but the offer is $12.50/kg against the larger pack's $60/kg")
	}
	if got, _ := env["worse_than_single_pack_count"].(float64); got != 0 {
		t.Fatalf("worse_than_single_pack_count = %v, want 0", got)
	}
}

// TestMultibuyEmptyResultIsValidJSON is the absence-of-correctness case on a
// fixture with no multibuy tags at all.
func TestMultibuyEmptyResultIsValidJSON(t *testing.T) {
	newWowFakeServer(t, &wowFakeServer{
		searchResultsCount: 2,
		pages: map[int][]wowTestTile{
			1: {
				{Stockcode: 31, Name: "Free Range Eggs 12 pack", Price: 8, CupPrice: 0.67, CupMeasure: "1EA", PackageSize: "12 pack", IsAvailable: true, IsInStock: true},
				{Stockcode: 32, Name: "Cage Free Eggs 6 pack", Price: 5, CupPrice: 0.83, CupMeasure: "1EA", PackageSize: "6 pack", IsAvailable: true, IsInStock: true},
			},
		},
	})

	out, err := runCLI(t, "multibuy", "eggs", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("multibuy error = %v\n%s", err, out)
	}
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)
	requireEmptyJSONArray(t, env, "offers")
	requireEmptyJSONArray(t, env, "fetch_failures")
	if got, _ := env["offers_found"].(float64); got != 0 {
		t.Fatalf("offers_found = %v, want 0", got)
	}
	if got, _ := env["scanned_products"].(float64); got != 2 {
		t.Fatalf("scanned_products = %v, want 2; an empty offer list still has to report what was looked at", got)
	}
	note, _ := env["note"].(string)
	if !strings.Contains(note, "no multi-buy offers found") {
		t.Fatalf("note = %q, want an explicit statement that no offers exist", note)
	}
	if !strings.Contains(note, "--max-scan-pages") {
		t.Fatalf("note = %q, want it to name the flag that widens the search", note)
	}
}

// TestMultibuyCategoryPositionalUsesBrowseEndpoint proves a hyphenated
// category id is routed to /apis/ui/browse/category rather than search.
func TestMultibuyCategoryPositionalUsesBrowseEndpoint(t *testing.T) {
	srv := newWowFakeServer(t, &wowFakeServer{
		searchResultsCount: 1,
		pages: map[int][]wowTestTile{
			1: {{Stockcode: 41, Name: "Category Block 100g", Price: 2, CupPrice: 2, CupMeasure: "100G", PackageSize: "100g", IsAvailable: true, IsInStock: true}},
		},
	})

	out, err := runCLI(t, "multibuy", "1-E5BEE36E", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	if err != nil {
		t.Fatalf("multibuy error = %v\n%s", err, out)
	}
	env := decodeEnvelope(t, out)
	if kind, _ := env["query_kind"].(string); kind != "category" {
		t.Fatalf("query_kind = %q, want category", kind)
	}
	requests := srv.seen()
	if len(requests) == 0 || requests[0].Path != wowCategoryPath {
		t.Fatalf("first request went to %v, want %s", requests, wowCategoryPath)
	}
	assertNoOversizedPageSize(t, requests)
	t.Logf("query_kind=%v path=%s", env["query_kind"], requests[0].Path)
}

func TestMultibuyLooksLikeCategory(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"1-E5BEE36E", true},
		{"1-E5BEE36E-2", true},
		{"chocolate", false},
		{"laundry powder", false},
		{"698861", false},
		{"olive-oil", false},
		{"1-", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := multibuyLooksLikeCategory(tt.in); got != tt.want {
				t.Fatalf("multibuyLooksLikeCategory(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestMultibuyLiveNoOffersTerm is the live absence-of-correctness case. A term
// whose shelf carries no multi-buy tags must return [] with an honest note, not
// a fabricated offer.
func TestMultibuyLiveNoOffersTerm(t *testing.T) {
	if !liveTestsEnabled(t) {
		return
	}
	out, err := runCLI(t, "multibuy", "fresh bananas", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	skipIfUnreachable(t, err, out)
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	offers, _ := env["offers"].([]any)
	found, _ := env["offers_found"].(float64)
	if len(offers) == 0 {
		if int(found) != 0 {
			t.Fatalf("offers is empty but offers_found = %v", found)
		}
		requireEmptyJSONArray(t, env, "offers")
		note, _ := env["note"].(string)
		if !strings.Contains(note, "no multi-buy offers found") {
			t.Fatalf("note = %q, want an honest statement that nothing was found", note)
		}
		t.Logf("PASS: empty offers with note %q", note)
		return
	}
	// If the live shelf does carry offers today, every one of them must be a
	// real tag with a real quantity — nothing may be invented to fill the list.
	for i, raw := range offers {
		offer, _ := raw.(map[string]any)
		qty, _ := offer["offer_quantity"].(float64)
		total, _ := offer["offer_total"].(float64)
		if qty < 2 || total <= 0 {
			t.Fatalf("offer %d has quantity %v total %v; that is not a real multi-buy tag", i, qty, total)
		}
	}
	t.Logf("live shelf carried %d real offer(s) today; all had a genuine quantity and total", len(offers))
}

// TestMultibuyLiveChocolateHasRealOffers exercises the happy path against the
// live shelf that reliably carries multi-buy tags.
func TestMultibuyLiveChocolateHasRealOffers(t *testing.T) {
	if !liveTestsEnabled(t) {
		return
	}
	out, err := runCLI(t, "multibuy", "chocolate", "--limit", "3", "--json", "--max-scan-pages", "1", "--db", tempDBPath(t, "absent.db"))
	skipIfUnreachable(t, err, out)
	t.Logf("raw output:\n%s", out)
	env := decodeEnvelope(t, out)

	offers, _ := env["offers"].([]any)
	if len(offers) == 0 {
		t.Skipf("the live chocolate shelf carried no multi-buy tags at this moment; note=%v", env["note"])
	}
	if len(offers) > 3 {
		t.Fatalf("--limit 3 returned %d offers", len(offers))
	}
	for i, raw := range offers {
		offer, _ := raw.(map[string]any)
		qty, _ := offer["offer_quantity"].(float64)
		total, _ := offer["offer_total"].(float64)
		effective, _ := offer["effective_unit_price"].(float64)
		if qty <= 0 || total <= 0 {
			t.Fatalf("offer %d: quantity %v total %v", i, qty, total)
		}
		want := total / qty
		if diff := effective - want; diff > 0.011 || diff < -0.011 {
			t.Fatalf("offer %d effective_unit_price = %v, want %v (total/quantity)", i, effective, want)
		}
		if _, ok := offer["worse_than_single_pack"]; !ok {
			t.Fatalf("offer %d has no worse_than_single_pack verdict", i)
		}
		t.Logf("offer %d: %v | %v for $%v | $%v each | worse_than_single_pack=%v | %v",
			i, offer["name"], qty, total, effective, offer["worse_than_single_pack"], offer["unit_basis"])
	}
}

// TestMultibuySortBreaksIdenticalSavingsOnUnitPrice pins the ranking. Every
// "2 for $10" against a $7.50 single saves exactly 33.33%, so an entire shelf
// ties on SavingPct; without a unit-price tie-break the order collapses to
// stockcode and a dearer offer takes a slot inside --limit from a cheaper one.
func TestMultibuySortBreaksIdenticalSavingsOnUnitPrice(t *testing.T) {
	offers := []multibuyOffer{
		{Stockcode: "111", SavingPct: 33.33, OfferUnitPrice: 39.70, UnitBasis: "1kg", MeasureKind: "mass"},
		{Stockcode: "222", SavingPct: 33.33, OfferUnitPrice: 27.80, UnitBasis: "1kg", MeasureKind: "mass"},
		{Stockcode: "333", SavingPct: 40.00, OfferUnitPrice: 51.00, UnitBasis: "1kg", MeasureKind: "mass"},
		{Stockcode: "444", SavingPct: 33.33, OfferUnitPrice: 37.00, UnitBasis: "1kg", MeasureKind: "mass"},
	}
	multibuySortOffers(offers)

	want := []string{"333", "222", "444", "111"}
	for i, code := range want {
		if offers[i].Stockcode != code {
			t.Fatalf("offers[%d].Stockcode = %q, want %q (full order: %v)",
				i, offers[i].Stockcode, code, multibuyStockcodes(offers))
		}
	}
}

// TestMultibuySortKeepsUnnormalisedOffersOutOfTheLead guards the other half of
// the tie-break: an offer whose measure never normalised has no unit price at
// all, and must not sort ahead of priced offers at a phantom zero.
func TestMultibuySortKeepsUnnormalisedOffersOutOfTheLead(t *testing.T) {
	offers := []multibuyOffer{
		{Stockcode: "aaa", SavingPct: 33.33},
		{Stockcode: "bbb", SavingPct: 33.33, OfferUnitPrice: 27.80, UnitBasis: "1kg", MeasureKind: "mass"},
	}
	multibuySortOffers(offers)

	if offers[0].Stockcode != "bbb" {
		t.Fatalf("unnormalised offer led the ranking: %v", multibuyStockcodes(offers))
	}
}

// TestMultibuySortNeverRanksAcrossMeasureKinds keeps dollars-per-kilo and
// dollars-per-each in separate tie-break groups: the cheaper number in a
// different basis must not jump the queue, it is simply not the same question.
func TestMultibuySortNeverRanksAcrossMeasureKinds(t *testing.T) {
	offers := []multibuyOffer{
		{Stockcode: "kg-cheap", SavingPct: 33.33, OfferUnitPrice: 27.80, UnitBasis: "1kg", MeasureKind: "mass"},
		{Stockcode: "ea-cheap", SavingPct: 33.33, OfferUnitPrice: 1.20, UnitBasis: "1ea", MeasureKind: "count"},
		{Stockcode: "kg-dear", SavingPct: 33.33, OfferUnitPrice: 39.70, UnitBasis: "1kg", MeasureKind: "mass"},
		{Stockcode: "ea-dear", SavingPct: 33.33, OfferUnitPrice: 2.40, UnitBasis: "1ea", MeasureKind: "count"},
	}
	multibuySortOffers(offers)

	got := multibuyStockcodes(offers)
	// Bases stay contiguous, and inside each basis the cheaper unit price wins.
	want := []string{"ea-cheap", "ea-dear", "kg-cheap", "kg-dear"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ranking mixed measure bases: got %v, want %v", got, want)
		}
	}
}

func multibuyStockcodes(offers []multibuyOffer) []string {
	codes := make([]string, 0, len(offers))
	for _, o := range offers {
		codes = append(codes, o.Stockcode)
	}
	return codes
}
