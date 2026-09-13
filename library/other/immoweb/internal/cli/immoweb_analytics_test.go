package cli

import (
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

func f64(v float64) *float64 { return &v }
func iptr(v int) *int        { return &v }

func stored(id int64, deal, pc string, price, surface float64, beds int) store.StoredListing {
	l := immo.Listing{ID: id, Deal: deal, Type: "APARTMENT", PostalCode: pc, Price: f64(price), Surface: f64(surface), Bedrooms: iptr(beds)}
	l.PricePerSqm = immo.PricePerSqm(deal, l.Price, l.Surface)
	return store.StoredListing{Listing: l}
}

func TestPriceCutSources(t *testing.T) {
	l := stored(1, "FOR_SALE", "1050", 380000, 100, 2)
	hist := []store.PriceObs{{ObservedAt: "2026-09-01T00:00:00Z", Price: 400000}, {ObservedAt: "2026-09-05T00:00:00Z", Price: 390000}, {ObservedAt: "2026-09-10T00:00:00Z", Price: 380000}}
	ci, ok := priceCut(l, hist)
	if !ok || ci.Cuts != 2 || ci.CutPct != 5 || ci.Source != "local-history" || ci.LastCutAt != "2026-09-10T00:00:00Z" {
		t.Errorf("history cut = %+v %v", ci, ok)
	}
	l2 := stored(2, "FOR_SALE", "1050", 300000, 100, 2)
	l2.OldPrice = f64(320000)
	ci, ok = priceCut(l2, nil)
	if !ok || ci.Source != "immoweb-old-price" || ci.CutEUR != 20000 {
		t.Errorf("old-price cut = %+v %v", ci, ok)
	}
	if _, ok := priceCut(stored(3, "FOR_SALE", "1050", 300000, 100, 2), nil); ok {
		t.Error("no history and no old price must not report a cut")
	}
	up := []store.PriceObs{{Price: 300000}, {Price: 310000}}
	if _, ok := priceCut(stored(4, "FOR_SALE", "1050", 310000, 100, 2), up); ok {
		t.Error("a price increase is not a cut")
	}
}

func TestComparablesAndVerdict(t *testing.T) {
	target := stored(10, "FOR_SALE", "1050", 300000, 100, 2).Listing // 3000/m2
	pool := []store.StoredListing{}
	for i, pps := range []float64{3500, 3600, 3700, 3800, 4000, 4200} {
		pool = append(pool, stored(int64(100+i), "FOR_SALE", "1050", pps*100, 100, 2))
	}
	pool = append(pool, stored(200, "FOR_SALE", "1060", 1000*100, 100, 2)) // other postcode
	pool = append(pool, stored(201, "FOR_SALE", "1050", 2000*300, 300, 2)) // surface out of ±25%
	pool = append(pool, stored(202, "FOR_RENT", "1050", 1500, 100, 2))     // other deal
	vals, comps := comparablePrices(target, pool)
	if len(vals) != 6 || len(comps) != 6 {
		t.Fatalf("expected 6 comparables, got %d", len(vals))
	}
	v := dealView{Listing: target}
	verdict := dealVerdict(target.PricePerSqm, vals, &v)
	if verdict != "cheap" || v.PercentileVsComps == nil || *v.PercentileVsComps != 0 || v.FairPriceAtMedian == nil {
		t.Errorf("verdict = %s %+v", verdict, v)
	}
	few := dealView{Listing: target}
	if got := dealVerdict(target.PricePerSqm, vals[:3], &few); got != "not enough comparables" {
		t.Errorf("few comparables verdict = %s", got)
	}
	none := dealView{Listing: target}
	if got := dealVerdict(nil, vals, &none); got != "unknown" {
		t.Errorf("no surface verdict = %s", got)
	}
}

func TestFillMarketAndYield(t *testing.T) {
	rows := []store.StoredListing{}
	for i := 0; i < 6; i++ {
		r := stored(int64(300+i), "FOR_RENT", "1050", 1000+float64(i)*100, 60, 1)
		r.CreatedAt = time.Now().Add(-time.Duration(10+i) * 24 * time.Hour).UTC().Format(time.RFC3339)
		rows = append(rows, r)
	}
	rows = append(rows, stored(400, "FOR_RENT", "1050", 2000, 100, 2))
	mc := marketCommune{}
	fillMarket(&mc, rows, 2, map[int64][]store.PriceObs{}, "bedrooms", time.Now())
	if mc.Sample != 7 || mc.GoneLast30d != 2 || mc.MedianPrice == nil || *mc.MedianPrice != 1300 {
		t.Errorf("market = %+v", mc)
	}
	if mc.DaysSample != 6 || mc.MedianDaysListed == nil {
		t.Errorf("days sample = %d %v", mc.DaysSample, mc.MedianDaysListed)
	}
	if len(mc.Bands) != 2 || mc.Bands[0].Band != "1" || mc.Bands[0].N != 6 {
		t.Errorf("bands = %+v", mc.Bands)
	}
	est := yieldEstimate{}
	fillYield(&est, f64(288000), rows[:6])
	if est.Insufficient || est.GrossYieldPct == nil || *est.GrossYieldPct != 5.21 {
		t.Errorf("yield = %+v", est)
	}
	short := yieldEstimate{}
	fillYield(&short, f64(288000), rows[:3])
	if !short.Insufficient || short.GrossYieldPct != nil {
		t.Errorf("fewer than 5 rents must refuse: %+v", short)
	}
}

func TestCritFlagsBuild(t *testing.T) {
	cf := critFlags{types: "maison,appartement", deal: "rent", communes: "ixelles,1060", postcodes: "1050", maxPrice: 1500, epc: "a,b", sort: "newest"}
	c, err := cf.build()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Types) != 2 || c.Deal != "FOR_RENT" || len(c.Communes) != 1 || len(c.PostalCodes) != 2 || c.Sort != "newest" || c.EPC[0] != "A" {
		t.Errorf("build = %+v", c)
	}
	if _, err := (&critFlags{postcodes: "abc"}).build(); err == nil {
		t.Error("bad postcode should fail")
	}
	if _, err := (&critFlags{deal: "swap"}).build(); err == nil {
		t.Error("bad deal should fail")
	}
}

func TestLocalTriageFilters(t *testing.T) {
	c := immo.Criteria{Types: []string{"APARTMENT"}, Deal: "FOR_RENT", MaxPrice: 1500, MinBedrooms: 2, EPC: []string{"A", "B"}}
	ok := stored(1, "FOR_RENT", "1050", 1400, 80, 2)
	ok.EPC = "B"
	if !matchesStored(ok, c) {
		t.Error("matching listing rejected")
	}
	if matchesStored(stored(2, "FOR_RENT", "1050", 1600, 80, 2), c) {
		t.Error("price above max accepted")
	}
	if matchesStored(stored(3, "FOR_RENT", "1050", 1400, 80, 1), c) {
		t.Error("too few bedrooms accepted")
	}
	badEPC := stored(4, "FOR_RENT", "1050", 1400, 80, 2)
	badEPC.EPC = "F"
	if matchesStored(badEPC, c) {
		t.Error("EPC outside the list accepted")
	}
	if !matchesStored(stored(5, "FOR_RENT", "1050", 1400, 80, 3), c) {
		t.Error("unknown EPC must not exclude")
	}
	if localUnsupported(immo.Criteria{Garden: true}) == "" || localUnsupported(immo.Criteria{Provinces: []string{"NAMUR"}}) == "" {
		t.Error("unsupported local criteria not reported")
	}
	if localUnsupported(c) != "" {
		t.Error("numeric criteria are supported locally")
	}
}

func TestCSVSafeAndPostcodeScoped(t *testing.T) {
	if csvSafe("=HYPERLINK(1)") != "'=HYPERLINK(1)" || csvSafe("+32") != "'+32" || csvSafe("Ixelles") != "Ixelles" || csvSafe("") != "" {
		t.Error("csvSafe did not neutralise formula prefixes")
	}
	if !postcodeScoped(immo.Criteria{Types: []string{"HOUSE"}, Deal: "FOR_SALE", PostalCodes: []string{"BE-1050"}}) {
		t.Error("deal/type/location search is a pure scope")
	}
	if postcodeScoped(immo.Criteria{Types: []string{"HOUSE"}, Deal: "FOR_SALE", MaxPrice: 1}) {
		t.Error("price-filtered search is not a pure scope")
	}
}

func TestPriceCutOldPriceWinsSource(t *testing.T) {
	l := stored(9, "FOR_SALE", "1050", 300000, 100, 2)
	l.OldPrice = f64(350000)
	l.ModifiedAt = "2026-09-10T00:00:00Z"
	ci, ok := priceCut(l, []store.PriceObs{{ObservedAt: "2026-09-01T00:00:00Z", Price: 320000}, {ObservedAt: "2026-09-05T00:00:00Z", Price: 320000}})
	if !ok || ci.Source != "immoweb-old-price" || ci.FirstPrice != 350000 || ci.LastCutAt == "" {
		t.Errorf("old price should win with its own source: %+v", ci)
	}
}

func TestBandOrderAndSharedPostcodes(t *testing.T) {
	keys := []string{"4+", "studio", "unknown", "2", "1", "3"}
	sort.Slice(keys, func(i, j int) bool { return bandOrder(keys[i]) < bandOrder(keys[j]) })
	if strings.Join(keys, ",") != "studio,1,2,3,4+,unknown" {
		t.Errorf("band order = %v", keys)
	}
	w := sharedPostcodeWarnings([]marketCommune{{Commune: "a", PostalCodes: []string{"BE-1050"}}, {Commune: "b", PostalCodes: []string{"BE-1050", "BE-1060"}}, {Commune: "c", PostalCodes: []string{"BE-4000"}}})
	if len(w) != 1 || !strings.Contains(w[0], "1050") {
		t.Errorf("shared warnings = %v", w)
	}
}

func TestComparablesKeepRoomsApart(t *testing.T) {
	target := stored(1, "FOR_RENT", "4000", 1000, 80, 2).Listing
	pool := []store.StoredListing{}
	for i := 0; i < 6; i++ {
		pool = append(pool, stored(int64(10+i), "FOR_RENT", "4000", 900+float64(i)*20, 80, 2))
	}
	kot := stored(99, "FOR_RENT", "4000", 400, 80, 2)
	kot.Subtype = "KOT"
	pool = append(pool, kot)
	vals, _ := comparablePrices(target, pool)
	if len(vals) != 6 {
		t.Errorf("a kot must not be a comparable for a whole flat: %d", len(vals))
	}
	kept, n := dropRoomLets(pool)
	if n != 1 || len(kept) != 6 {
		t.Errorf("dropRoomLets = %d kept, %d dropped", len(kept), n)
	}
}

func TestTermSafeAndPhotoHost(t *testing.T) {
	if got := termSafe("Nice flat\x1b]52;c;aGk=\x07 now\ttab"); got != "Nice flat]52;c;aGk= now\ttab" {
		t.Errorf("termSafe = %q", got)
	}
	for raw, want := range map[string]bool{
		"https://media.immowebstatic.be/a.jpg": true,
		"https://immowebstatic.be/a.jpg":       true,
		"https://evilimmowebstatic.be/a.jpg":   false,
		"http://media.immowebstatic.be/a.jpg":  false,
		"https://MEDIA.IMMOWEBSTATIC.BE/a.jpg": true,
	} {
		u, _ := url.Parse(raw)
		if immowebStaticURL(u) != want {
			t.Errorf("immowebStaticURL(%s) != %v", raw, want)
		}
	}
}
