// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/store"
	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

func fp(v float64) *float64 { return &v }
func ip(v int) *int         { return &v }

func stored(l zimmo.Listing) store.StoredListing { return store.StoredListing{Listing: l} }

func TestComputeDrops(t *testing.T) {
	withHistory := zimmo.Listing{Code: "AAAAA", Address: "Rue A 1, 1030 Schaerbeek", PriceHistory: []zimmo.PriceChange{
		{Date: "2026-08-01T00:00:00Z", Before: 300000, After: 280000},
		{Date: "2026-09-01T00:00:00Z", Before: 280000, After: 270000},
	}}
	localOnly := zimmo.Listing{Code: "BBBBB"}
	rising := zimmo.Listing{Code: "CCCCC"}
	obs := map[string][]store.PriceObs{
		"BBBBB": {{ObservedAt: "2026-09-01T00:00:00Z", Price: 200000}, {ObservedAt: "2026-09-10T00:00:00Z", Price: 190000}},
		"CCCCC": {{ObservedAt: "2026-09-01T00:00:00Z", Price: 200000}, {ObservedAt: "2026-09-10T00:00:00Z", Price: 210000}},
	}
	rows := computeDrops([]store.StoredListing{stored(withHistory), stored(localOnly), stored(rising)}, obs, "", 0)
	if len(rows) != 2 {
		t.Fatalf("want 2 drops (rising price excluded), got %d: %+v", len(rows), rows)
	}
	if rows[0].Code != "AAAAA" || rows[0].From != 300000 || rows[0].To != 270000 || rows[0].CutPct != 10 {
		t.Errorf("biggest cut first with first→last prices: %+v", rows[0])
	}
	if rows[0].Since != "2026-09-01T00:00:00Z" || rows[0].Source != "zimmo_history" {
		t.Errorf("since = first point at final price, source zimmo_history: %+v", rows[0])
	}
	if rows[1].Source != "local_observations" {
		t.Errorf("local-only drop source: %+v", rows[1])
	}
	if got := computeDrops([]store.StoredListing{stored(withHistory)}, nil, "2026-09-05T00:00:00Z", 0); len(got) != 0 {
		t.Errorf("--since cutoff after the cut must drop it, got %+v", got)
	}
	if got := computeDrops([]store.StoredListing{stored(withHistory)}, nil, "", 15); len(got) != 0 {
		t.Errorf("--min-pct 15 must drop a 10%% cut, got %+v", got)
	}
}

func TestScoreMotivatedRelistingNeedsSameUnit(t *testing.T) {
	now := time.Now().UTC()
	old := now.AddDate(0, 0, -200).Format(time.RFC3339)
	recent := now.AddDate(0, 0, -20).Format(time.RFC3339)
	same := now.AddDate(0, 0, -20).Format(time.RFC3339)
	cur := zimmo.Listing{Code: "NEW01", Type: "APARTMENT", Street: "Rue X", Number: "5", PostalCode: "1060", Surface: fp(80), Bedrooms: ip(2), PublishedAt: recent, DaysOnMarket: ip(20), EPC: "G"}
	prev := zimmo.Listing{Code: "OLD01", Status: "FOR_SALE", Type: "APARTMENT", Street: "Rue X", Number: "5", PostalCode: "1060", Surface: fp(81), Bedrooms: ip(2), PublishedAt: old}
	sibling := zimmo.Listing{Code: "SIB01", Status: "FOR_SALE", Type: "APARTMENT", Street: "Rue X", Number: "5", PostalCode: "1060", Surface: fp(120), Bedrooms: ip(3), PublishedAt: old}
	sameDay := zimmo.Listing{Code: "DAY01", Status: "FOR_SALE", Type: "APARTMENT", Street: "Rue X", Number: "5", PostalCode: "1060", Surface: fp(80), Bedrooms: ip(2), PublishedAt: same}
	rental := zimmo.Listing{Code: "RENT1", Status: "TO_RENT", Type: "APARTMENT", Street: "Rue X", Number: "5", PostalCode: "1060", Surface: fp(80), Bedrooms: ip(2), PublishedAt: old}
	all := []store.StoredListing{stored(cur), stored(prev), stored(sibling), stored(sameDay), stored(rental)}
	rows := scoreMotivated([]store.StoredListing{stored(cur)}, all, nil, 0)
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %+v", rows)
	}
	r := rows[0]
	if len(r.RelistedFrom) != 1 || r.RelistedFrom[0] != "OLD01" {
		t.Errorf("only the same unit, for sale, published >=14 days earlier is a re-listing, got %v", r.RelistedFrom)
	}
	if r.MarketDays == nil || *r.MarketDays < 199 {
		t.Errorf("true market days should come from the older publication, got %v", r.MarketDays)
	}
	quiet := zimmo.Listing{Code: "QUIET", Type: "HOUSE", DaysOnMarket: ip(10), EPC: "B"}
	if got := scoreMotivated([]store.StoredListing{stored(quiet)}, nil, nil, 0); len(got) != 0 {
		t.Errorf("a fresh listing without signals must not be ranked: %+v", got)
	}
	cutFresh := zimmo.Listing{Code: "CUT01", Type: "HOUSE", DaysOnMarket: ip(30), PriceHistory: []zimmo.PriceChange{{Date: "2026-09-01T00:00:00Z", Before: 300000, After: 270000}}}
	if got := scoreMotivated([]store.StoredListing{stored(cutFresh)}, nil, nil, 120); len(got) != 0 {
		t.Errorf("--min-days 120 is a hard filter, even with a price cut: %+v", got)
	}
}

func TestMatchSiblings(t *testing.T) {
	l := zimmo.Listing{Code: "LRZA4", Street: "Rue Anatole France", Number: "37", PostalCode: "1030", Price: fp(575000), Surface: fp(215), Bedrooms: ip(5)}
	sibs := []siblingRow{
		{Portal: "immoweb", ID: "1", Street: "Rue Anatole France 37", PostalCode: "1030", Price: fp(575000), Surface: fp(215)},
		{Portal: "immovlan", ID: "VBE1", Street: "", PostalCode: "1030", Price: fp(560000), Surface: fp(214), Bedrooms: ip(5)},
		{Portal: "immoweb", ID: "2", Street: "Rue Anatole France 39", PostalCode: "1030", Price: fp(575000), Surface: fp(100)},
		{Portal: "immoweb", ID: "3", Street: "", PostalCode: "1050", Price: fp(575000), Surface: fp(215), Bedrooms: ip(5)},
	}
	for i := range sibs {
		sibs[i].AddrKey = store.AddrKey(sibs[i].PostalCode, sibs[i].Street)
	}
	row := matchSiblings(l, indexSiblings(sibs))
	if row.MatchStatus != "matched" || len(row.Matches) != 2 {
		t.Fatalf("want address match + surface/bedrooms/price match, got %+v", row.Matches)
	}
	if row.Matches[0].MatchedBy != "address" || row.Matches[0].Confidence != "high" {
		t.Errorf("address match must rank first: %+v", row.Matches[0])
	}
	if !row.Matches[1].StreetMissing || row.Matches[1].DeltaPct == nil || *row.Matches[1].DeltaPct != 2.7 {
		t.Errorf("immovlan copy has no street and a +2.7%% gap: %+v", row.Matches[1])
	}
	lone := zimmo.Listing{Code: "LONE1", Street: "Rue Nulle", Number: "1", PostalCode: "1180", Price: fp(1)}
	if r := matchSiblings(lone, indexSiblings(sibs)); r.MatchStatus != "zimmo_only" || len(r.Matches) != 0 {
		t.Errorf("unrelated listing must be zimmo_only: %+v", r)
	}
}

func TestPebTrapRows(t *testing.T) {
	ls := []store.StoredListing{
		stored(zimmo.Listing{Code: "F1", EPC: "F", EPCKWh: fp(450)}),
		stored(zimmo.Listing{Code: "G1", EPC: "G-", EPCKWh: fp(27000), EPCSuspect: true}),
		stored(zimmo.Listing{Code: "G2", EPC: "G", EPCKWh: fp(700)}),
		stored(zimmo.Listing{Code: "R1", EPC: "F", EPCKWh: fp(400), Rented: true, RentPerYear: fp(12000), Price: fp(240000)}),
		stored(zimmo.Listing{Code: "B1", EPC: "B", EPCKWh: fp(120)}),
	}
	rows := pebTrapRows(ls, []string{"F", "G"}, 0, false, false)
	order := []string{}
	for _, r := range rows {
		order = append(order, r.Code)
	}
	want := []string{"R1", "G2", "G1", "F1"}
	if len(order) != len(want) {
		t.Fatalf("want %v, got %v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("rented first, then G before F, suspect kWh last within a letter: want %v, got %v", want, order)
		}
	}
	if rows[0].GrossYieldPct == nil || *rows[0].GrossYieldPct != 5 {
		t.Errorf("rented yield 12000/240000 = 5%%: %+v", rows[0].GrossYieldPct)
	}
	if got := pebTrapRows(ls, []string{"F", "G"}, 0, true, false); len(got) != 1 {
		t.Errorf("--rented-only keeps one: %+v", got)
	}
	if got := pebTrapRows(ls, []string{"G"}, 400, false, false); len(got) != 4 {
		t.Errorf("--min-kwh 400 adds F rows at >=400 but never suspect ones: got %d", len(got))
	}
}

func TestDiffWatch(t *testing.T) {
	seen := map[string]store.SeenEntry{"KEEP1": {Code: "KEEP1", LastPrice: fp(300000)}, "GONE1": {Code: "GONE1", LastPrice: fp(250000)}}
	cur := []zimmo.Listing{{Code: "KEEP1", Price: fp(290000)}, {Code: "NEW01", Price: fp(100000)}}
	v := diffWatch("s", seen, cur, false)
	if v.New != 1 || v.Cheaper != 1 || v.Gone != 1 {
		t.Fatalf("want 1 new, 1 cheaper, 1 gone, got %+v", v)
	}
	dup := diffWatch("s", nil, []zimmo.Listing{{Code: "DUP01"}, {Code: "DUP01"}}, true)
	if dup.New != 1 || len(dup.Changes) != 1 {
		t.Errorf("a code repeated across pages is one new listing: %+v", dup)
	}
	if v2 := diffWatch("s", map[string]store.SeenEntry{"KEEP1": {Code: "KEEP1", LastPrice: fp(290000)}}, cur[:1], false); len(v2.Changes) != 0 {
		t.Errorf("unchanged listing must produce no change: %+v", v2.Changes)
	}
}

func TestParsers(t *testing.T) {
	for in, want := range map[string]int{"800": 800, "800m": 800, "1.5km": 1500} {
		if got, err := parseMeters(in); err != nil || got != want {
			t.Errorf("parseMeters(%q)=%d,%v", in, got, err)
		}
	}
	if _, err := parseMeters("50km"); err == nil {
		t.Error("50km radius must be refused")
	}
	for in, want := range map[string]string{"laisz": "LAISZ", "https://www.zimmo.be/fr/lanaken-3620/a-vendre/maison/LAISZ": "LAISZ", "00003C25-4B94-4E38-BEB3-29CBA71EA00C": "00003c25-4b94-4e38-beb3-29cba71ea00c"} {
		if got, err := parseCodeArg(in); err != nil || got != want {
			t.Errorf("parseCodeArg(%q)=%q,%v", in, got, err)
		}
	}
	if _, err := parseCodeArg("rue de la loi"); err == nil {
		t.Error("free text is not a code")
	}
	h := make([]zimmo.PricePoint, 13)
	h[0] = zimmo.PricePoint{Types: []zimmo.TypePrice{{Type: "HOUSE", Price: 2000}}}
	h[12] = zimmo.PricePoint{Types: []zimmo.TypePrice{{Type: "HOUSE", Price: 2200}}}
	if c := change12m(h); c["HOUSE"] != 10 {
		t.Errorf("12-month change = +10%%, got %v", c)
	}
}

func TestRentPoolSample(t *testing.T) {
	p := &rentPool{}
	for i := 0; i < 6; i++ {
		p.comps = append(p.comps, rentComp{perM2: 15, surface: 60, bedrooms: ip(1)})
	}
	for i := 0; i < 2; i++ {
		p.comps = append(p.comps, rentComp{perM2: 10, surface: 200, bedrooms: ip(4)})
	}
	if s, basis := p.sample(62, ip(1)); len(s) != 6 || basis != "similar_surface" {
		t.Errorf("similar surface sample: %v %s", s, basis)
	}
	if s, _ := p.sample(500, ip(6)); s != nil {
		t.Errorf("a 500 m² house has no comparable rentals: %v", s)
	}
}

func TestSafetyHelpers(t *testing.T) {
	if got := safeFileStem("../../etc/x"); got != "etcx" {
		t.Errorf("safeFileStem strips path characters: %q", got)
	}
	if got := csvSafe("  =HYPERLINK(1)"); got[0] != '\'' {
		t.Errorf("csvSafe neutralises formulas after leading spaces: %q", got)
	}
	if got := termSafe("a\u009b31mb\u202ec"); got != "a 31mbc" {
		t.Errorf("termSafe strips C1 and bidi controls: %q", got)
	}
	if !isUUID("00003c25-4b94-4e38-beb3-29cba71ea00c") || isUUID("LAISZ") {
		t.Error("isUUID")
	}
}
