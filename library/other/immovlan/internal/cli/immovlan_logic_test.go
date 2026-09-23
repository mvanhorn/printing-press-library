// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// Pure-logic tests for the hand-written Immovlan commands (no network, no cobra).

package cli

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

func fptr(v float64) *float64 { return &v }
func iptr(v int) *int         { return &v }

func TestPct1AndPercentile(t *testing.T) {
	for in, want := range map[float64]float64{-0.0524: -5.2, -0.0526: -5.3, 0.0524: 5.2, 0: 0, 1: 100} {
		if got := pct1(in); got != want {
			t.Errorf("pct1(%v) = %v, want %v", in, got, want)
		}
	}
	if got := percentileRank([]float64{1, 2, 3, 4}, 2); got != 50 {
		t.Errorf("percentileRank = %v", got)
	}
	if got := percentileRank(nil, 2); got != 0 {
		t.Errorf("empty pool = %v", got)
	}
	if m, ok := median([]float64{3, 1, 2, 10}); !ok || m != 2.5 {
		t.Errorf("median = %v %v", m, ok)
	}
}

func TestSanitizers(t *testing.T) {
	for in, want := range map[string]string{"=HYPERLINK(x)": "'=HYPERLINK(x)", "\t=1+1": "'\t=1+1", "\r@x": "'\r@x", "+cmd": "'+cmd", "-5": "'-5", "Rue X": "Rue X", "": ""} {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
	if got := termSafe("abc\x1b[31m‮evil⁦x​\tok"); got != "abc[31mevilx\tok" {
		t.Errorf("termSafe = %q", got)
	}
	v := sanitizeStrings(map[string]any{"a": "=x", "b": []any{"+y", 1.0}, "c": map[string]any{"d": "@z"}}, csvSafe)
	b, _ := json.Marshal(v)
	if s := string(b); !strings.Contains(s, `"'=x"`) || !strings.Contains(s, `"'+y"`) || !strings.Contains(s, `"'@z"`) {
		t.Errorf("sanitizeStrings = %s", s)
	}
}

func TestParseHelpers(t *testing.T) {
	if pcs, err := parsePostcodes("1030, BE-1210,1030"); err != nil || len(pcs) != 2 || pcs[1] != "1210" {
		t.Errorf("parsePostcodes = %v %v", pcs, err)
	}
	if _, err := parsePostcodes("103"); err == nil {
		t.Error("bad postcode must error")
	}
	if d, err := parseDealFlag(""); err != nil || d != "" {
		t.Errorf("empty deal = %q %v", d, err)
	}
	if d, err := parseDealFlag("a-vendre"); err != nil || d != immovlan.DealSale {
		t.Errorf("deal = %q %v", d, err)
	}
	if _, err := parseDealFlag("swap"); err == nil {
		t.Error("bad deal must error")
	}
	letters, err := parseEPCLetters("F,bad,b")
	if err != nil || !letters["F"] || !letters["G"] || !letters["B"] || letters["C"] || len(letters) != 3 {
		t.Errorf("parseEPCLetters = %v %v", letters, err)
	}
	if l, err := parseEPCLetters("D"); err != nil || len(l) != 1 || !l["D"] {
		t.Errorf("single letter must not expand to its band: %v %v", l, err)
	}
	if _, err := parseEPCLetters("zz"); err == nil {
		t.Error("unknown band must error")
	}
	if _, err := parseEPCLetters("z"); err == nil {
		t.Error("unknown letter must error")
	}
	if l, err := parseEPCLetters("a+,A++"); err != nil || !l["A+"] || !l["A++"] || len(l) != 2 {
		t.Errorf("A+ letters must stay exact: %v %v", l, err)
	}
	if got := fmtEUR(999.5); got != "1 000 €" {
		t.Errorf("fmtEUR = %q", got)
	}
	if got := fmtEUR(1234567); got != "1 234 567 €" {
		t.Errorf("fmtEUR = %q", got)
	}
	for in, want := range map[string]bool{"https://api-image.immovlan.be/x.jpg": true, "https://foo.immovlan.be/x": true, "http://api-image.immovlan.be/x.jpg": false, "https://evil-immovlan.be/x": false, "https://foo.immovlan.be.evil/x": false, "https://IMMOVLAN.BE/x": false, "https://API-IMAGE.immovlan.be/x": true} {
		u, _ := url.Parse(in)
		if got := immovlanImageURL(u); got != want {
			t.Errorf("immovlanImageURL(%s) = %v", in, got)
		}
	}
}

func TestTripleKey(t *testing.T) {
	if k := tripleKey("1030", nil, fptr(100), iptr(2), "x"); k != "" {
		t.Errorf("nil price = %q", k)
	}
	k1 := tripleKey("1030", fptr(500000), fptr(200), nil, "Agence Immobilière Trop Longue")
	k2 := tripleKey("1030", fptr(500000), fptr(200), nil, "AGENCE IMMOBILIERE trop longue")
	if k1 == "" || k1 != k2 || !strings.HasSuffix(k1, "|-1|agence immob") {
		t.Errorf("tripleKey = %q vs %q", k1, k2)
	}
}

func TestScopePostcodes(t *testing.T) {
	c := immovlan.Criteria{Towns: []string{"1030-schaerbeek", "liege"}, Deal: immovlan.DealSale}
	if pcs := scopePostcodes(c); len(pcs) != 1 || pcs[0] != "1030" {
		t.Errorf("scopePostcodes = %v", pcs)
	}
	if !postcodeScoped(c) {
		t.Error("town-scoped search must count as postcode scoped")
	}
	c.EPC = []string{"bad"}
	if postcodeScoped(c) {
		t.Error("an EPC filter breaks pure scope")
	}
}

func stored(id, addr, photo, firstSeen, gone string, price float64, typ string, beds int, surface float64) store.StoredListing {
	r := store.StoredListing{AddrKey: addr, PhotoHash: photo, FirstSeen: firstSeen, LastSeen: firstSeen, GoneAt: gone}
	r.ID, r.Price, r.Type, r.PostalCode, r.Locality = id, fptr(price), typ, "1030", "Schaerbeek"
	if beds > 0 {
		r.Bedrooms = iptr(beds)
	}
	if surface > 0 {
		r.Surface = fptr(surface)
	}
	return r
}

func TestGroupRelisted(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	rows := []store.StoredListing{
		// true re-listing: older ref gone before the new one appeared, price cut
		stored("VBE1", "1030|thiefry|54", "", "2026-06-01T00:00:00Z", "2026-08-01T00:00:00Z", 500000, "maison", 4, 200),
		stored("VBE2", "1030|thiefry|54", "", "2026-08-15T00:00:00Z", "", 475000, "maison", 4, 200),
		// sibling units in one building, all live the same day: not re-listings
		stored("VBE3", "1030|roodebeek|125", "", "2026-09-20T00:00:00Z", "", 300000, "appartement", 1, 50),
		stored("VBE4", "1030|roodebeek|125", "", "2026-09-20T00:00:00Z", "", 450000, "appartement", 2, 90),
		stored("VBE5", "1030|roodebeek|125", "", "2026-09-20T00:00:00Z", "", 600000, "appartement", 3, 130),
		// photo match without any address
		stored("VBE6", "", "abcd", "2026-05-01T00:00:00Z", "", 800000, "maison", 5, 300),
		stored("VBE7", "", "abcd", "2026-09-01T00:00:00Z", "", 780000, "maison", 5, 300),
	}
	rows[1].Flag = "new"
	groups := groupRelisted(rows, now)
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2: %+v", len(groups), groups)
	}
	byCurrent := map[string]relistGroup{}
	for _, g := range groups {
		byCurrent[g.Current] = g
	}
	g := byCurrent["VBE2"]
	if g.MatchedBy != "address" || g.Key != "1030|thiefry|54" || len(g.Refs) != 2 || g.FirstListed != "2026-06-01T00:00:00Z" || g.TrueDaysOnMkt != 113 || !g.FlaggedAsNew {
		t.Errorf("address group = %+v", g)
	}
	if g.PriceChangePct == nil || *g.PriceChangePct != -5 {
		t.Errorf("price change = %v", g.PriceChangePct)
	}
	if p := byCurrent["VBE7"]; p.MatchedBy != "photos" || p.Key != "abcd" || p.PriceChangePct == nil || *p.PriceChangePct != -2.5 {
		t.Errorf("photo group = %+v", p)
	}
	if _, ok := byCurrent["VBE5"]; ok {
		t.Error("sibling units must not be grouped")
	}
	// same home, both live, compatible: grouped even without a gone date
	a := stored("VBE8", "1030|max|57", "", "2026-07-01T00:00:00Z", "", 400000, "appartement", 2, 95)
	b := stored("VBE9", "1030|max|57", "", "2026-09-01T00:00:00Z", "", 390000, "appartement", 2, 100)
	if gs := groupRelisted([]store.StoredListing{a, b}, now); len(gs) != 1 {
		t.Errorf("compatible live pair = %d groups", len(gs))
	}
	if gs := groupRelisted([]store.StoredListing{a}, now); len(gs) != 0 {
		t.Error("single ref is not a group")
	}
	// an older, unrelated unit at the same address must not hide a real re-listing pair
	older := stored("VBE0", "1030|max|57", "", "2026-01-01T00:00:00Z", "", 250000, "appartement", 1, 50)
	if gs := groupRelisted([]store.StoredListing{older, a, b}, now); len(gs) != 1 || gs[0].Current != "VBE9" || len(gs[0].Refs) != 2 {
		t.Errorf("chain must skip the incompatible anchor: %+v", gs)
	}
	// address chain A,B,C with photos only on A,B: one group, current = C
	c := stored("VBE10", "1030|max|57", "", "2026-09-15T00:00:00Z", "", 385000, "appartement", 2, 100)
	a2, b2 := a, b
	a2.PhotoHash, b2.PhotoHash = "ph9", "ph9"
	if gs := groupRelisted([]store.StoredListing{c, a2, b2}, now); len(gs) != 1 || gs[0].Current != "VBE10" || len(gs[0].Refs) != 3 {
		t.Errorf("photo sub-chain must not duplicate the address group: %+v", gs)
	}
	// a sale that comes back as a rental at the same address is not a re-listing
	sale := stored("VBE11", "1030|max|58", "", "2026-03-01T00:00:00Z", "2026-06-01T00:00:00Z", 500000, "maison", 4, 200)
	sale.Deal = immovlan.DealSale
	rent := stored("VBE12", "1030|max|58", "", "2026-07-01T00:00:00Z", "", 1500, "maison", 4, 200)
	rent.Deal = immovlan.DealRent
	if gs := groupRelisted([]store.StoredListing{sale, rent}, now); len(gs) != 0 {
		t.Errorf("sale→rent must not group: %+v", gs)
	}
	// duplicate ads posted the same day, both live and compatible: not a re-listing
	d1 := stored("VBE13", "1030|max|59", "", "2026-09-20T00:00:00Z", "", 400000, "appartement", 2, 95)
	d2 := stored("VBE14", "1030|max|59", "", "2026-09-20T00:00:00Z", "", 400000, "appartement", 2, 95)
	if gs := groupRelisted([]store.StoredListing{d1, d2}, now); len(gs) != 0 {
		t.Errorf("same-day live pair must not group: %+v", gs)
	}
	// the site's created_at orders references, not the scrape order
	e1 := stored("VBE15", "1030|max|60", "", "2026-09-10T00:00:00Z", "", 400000, "appartement", 2, 95)
	e1.CreatedAt = "2026-09-08T00:00:00Z"
	e2 := stored("VBE16", "1030|max|60", "", "2026-09-10T00:00:00Z", "", 390000, "appartement", 2, 95)
	e2.CreatedAt = "2026-06-01T00:00:00Z"
	if gs := groupRelisted([]store.StoredListing{e1, e2}, now); len(gs) != 1 || gs[0].Current != "VBE15" || gs[0].FirstListed != "2026-06-01T00:00:00Z" {
		t.Errorf("created_at must order the chain: %+v", gs)
	}
	// pair matched both by address and photos: reported once, as an address match, every run
	a.PhotoHash, b.PhotoHash = "ph1", "ph1"
	for i := 0; i < 20; i++ {
		gs := groupRelisted([]store.StoredListing{b, a}, now)
		if len(gs) != 1 || gs[0].MatchedBy != "address" || gs[0].Refs[0].ID != "VBE8" {
			t.Fatalf("run %d: %+v", i, gs)
		}
	}
}

func TestMatchSameAs(t *testing.T) {
	r1 := stored("VBE1", "1030|thiefry|54", "", "2026-09-01T00:00:00Z", "", 500000, "maison", 4, 200)
	r1.Street, r1.Agency = "Rue Thiéfry 54", "Alpha"
	r2 := stored("VBE2", "", "", "2026-09-01T00:00:00Z", "", 300000, "appartement", 2, 90)
	r2.Agency = "Beta"
	r3 := stored("VBE3", "", "", "2026-09-01T00:00:00Z", "", 999999, "maison", 5, 300)
	immo := []immowebRow{
		{ID: 10, Street: "Rue Thiefry 54", PostalCode: "1030", Price: fptr(520000), GoneAt: "2026-08-01"},
		{ID: 11, Street: "Rue Thiéfry, 54", PostalCode: "1030", Price: fptr(510000)},
		{ID: 12, PostalCode: "1030", Price: fptr(300000), Surface: fptr(90), Bedrooms: iptr(2), Agency: "BETA"},
	}
	res, matched, unmatched, noStreet := matchSameAs([]store.StoredListing{r1, r2, r3}, immo, false, false)
	if matched != 2 || unmatched != 1 || noStreet != 2 || len(res) != 3 {
		t.Fatalf("counts = %d %d %d %d", matched, unmatched, noStreet, len(res))
	}
	if m := res[0].Match; m == nil || m.ImmowebID != 11 || m.MatchedBy != "address" || m.Confidence != "high" || m.ImmowebGone || m.DeltaEUR == nil || *m.DeltaEUR != -10000 || *m.DeltaPct != -2 {
		t.Errorf("address match must prefer the live row: %+v", m)
	}
	if m := res[1].Match; m == nil || m.ImmowebID != 12 || m.MatchedBy != "price_surface_agency" || m.DeltaEUR == nil || *m.DeltaEUR != 0 {
		t.Errorf("triple match = %+v", m)
	}
	if res[2].Match != nil || res[2].MatchStatus != "none" {
		t.Error("VBE3 must be unmatched")
	}
	if res[0].ImmowebID != 11 || res[0].MatchStatus != "address" || res[0].GoneStatus != "live" || res[0].ImmowebURL == "" {
		t.Errorf("flat match fields = %+v", res[0])
	}
	if only, _, _, _ := matchSameAs([]store.StoredListing{r1, r2, r3}, immo, true, false); len(only) != 1 || only[0].ID != "VBE3" {
		t.Errorf("--unmatched = %+v", only)
	}
	if gap, _, _, _ := matchSameAs([]store.StoredListing{r1, r2, r3}, immo, false, true); len(gap) != 1 || gap[0].ID != "VBE1" {
		t.Errorf("--price-gap = %+v", gap)
	}
}

func TestApplyFindFilters(t *testing.T) {
	mk := func(id, epc string, private bool) immovlan.Listing {
		return immovlan.Listing{ID: id, EPC: epc, Private: private}
	}
	fetched := []immovlan.Listing{mk("A", "F", false), mk("B", "C", true), mk("C", "G", true), mk("D", "F", true)}
	v := findView{Fetched: 4, PagesFetched: 1, LastPage: 3}
	applyFindFilters(&v, fetched, map[string]bool{"A": true}, true, map[string]bool{"F": true, "G": true}, 1)
	if v.Returned != 1 || v.Results[0].ID != "C" || v.Filtered["hidden"] != 1 || v.Filtered["agency"] != 0 || v.Filtered["epc_letter"] != 1 || !strings.Contains(v.Note, "1 of 3 pages (20 per page)") {
		t.Errorf("filters = %+v", v)
	}
	v2 := findView{Fetched: 2, TotalIsExact: true}
	applyFindFilters(&v2, fetched[:2], nil, false, map[string]bool{"A": true}, 0)
	if v2.Returned != 0 || v2.Note == "" || v2.Filtered["epc_letter"] != 2 {
		t.Errorf("all filtered = %+v", v2)
	}
	v3 := findView{Fetched: 2, TotalIsExact: true}
	applyFindFilters(&v3, fetched[:2], nil, false, nil, 0)
	if v3.Returned != 2 || v3.Note != "" || v3.Filtered != nil {
		t.Errorf("no filter = %+v", v3)
	}
}

func TestAggregateAgencies(t *testing.T) {
	mk := func(id, agency, agencyID, epc string, private bool, detail string, software string) store.StoredListing {
		r := store.StoredListing{DetailAt: detail, Software: software}
		r.ID, r.Agency, r.AgencyID, r.EPC, r.Private, r.PostalCode, r.PricePerSqm = id, agency, agencyID, epc, private, "1030", fptr(3000)
		return r
	}
	rows := []store.StoredListing{
		mk("A1", "Alpha", "1", "F", false, "2026-09-01T00:00:00Z", "Whise"),
		mk("A2", "Alpha", "1", "C", false, "", ""),
		mk("A3", "Alpha", "1", "G", false, "2026-09-01T00:00:00Z", "Whise"),
		mk("B1", "Beta", "2", "B", false, "", ""),
		mk("P1", "", "", "F", true, "", ""),
	}
	all, priv := aggregateAgencies(rows, nil)
	if priv != 1 || len(all) != 3 || all[0].Agency != "Alpha" || all[0].Listings != 3 || all[0].FG != 2 || all[0].FGSharePct != 66.7 {
		t.Errorf("all = %+v priv=%d", all, priv)
	}
	if all[0].Syndicated == nil || !*all[0].Syndicated || all[0].Software != "Whise" || all[1].Syndicated != nil {
		t.Errorf("syndication = %+v / %+v", all[0], all[1])
	}
	fg, _ := aggregateAgencies(rows, map[string]bool{"F": true, "G": true})
	if len(fg) != 2 || fg[0].Agency != "Alpha" || fg[0].Listings != 3 || fg[0].Matching != 2 || fg[1].Agency != "private sellers" {
		t.Errorf("filtered = %+v", fg)
	}
}

func TestRankRowAndCSV(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	rows := []store.StoredListing{}
	for i := 1; i <= 5; i++ {
		r := store.StoredListing{}
		r.ID, r.PostalCode, r.PricePerSqm = "V"+string(rune('0'+i)), "1030", fptr(float64(i)*1000)
		rows = append(rows, r)
	}
	pool := ppsPoolByPostcode(rows)
	rows[1].CreatedAt = "2026-09-20T00:00:00Z"
	rr := rankRow(rows[1], pool, now)
	if rr.PercentileInPostcode == nil || *rr.PercentileInPostcode != 40 || rr.DaysListed == nil || *rr.DaysListed != 2 {
		t.Errorf("rankRow = %+v", rr)
	}
	if thin := rankRow(rows[1], ppsPoolByPostcode(rows[:3]), now); thin.PercentileInPostcode != nil {
		t.Error("percentile must be blank under minPercentilePool")
	}
	var sb strings.Builder
	r := store.StoredListing{}
	r.ID, r.Title, r.Agency, r.Price = "VBE1", "=HYPERLINK(\"http://evil\")", "+Agency", fptr(500000)
	if err := writeVlanCSV(&sb, []store.StoredListing{r}); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	if !strings.HasPrefix(out, strings.Join(vlanCSVHeader, ",")) || !strings.Contains(out, `"'=HYPERLINK(""http://evil"")"`) || !strings.Contains(out, "'+Agency") {
		t.Errorf("csv = %s", out)
	}
	if len(vlanCSVRow(r)) != len(vlanCSVHeader) {
		t.Error("row/header length mismatch")
	}
}
