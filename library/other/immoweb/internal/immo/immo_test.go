package immo

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeType(t *testing.T) {
	cases := map[string]string{"house": "HOUSE", "Maison": "HOUSE", "appartement": "APARTMENT", "APARTMENT": "APARTMENT", "terrain": "LAND", "kantoor": "OFFICE"}
	for in, want := range cases {
		got, err := NormalizeType(in)
		if err != nil || got != want {
			t.Errorf("NormalizeType(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := NormalizeType("castle"); err == nil {
		t.Error("expected error for unknown type")
	}
	got, err := NormalizeTypes("house, maison,apartment")
	if err != nil || !reflect.DeepEqual(got, []string{"HOUSE", "APARTMENT"}) {
		t.Errorf("NormalizeTypes dedupe = %v, %v", got, err)
	}
}

func TestNormalizeDealAndSort(t *testing.T) {
	for in, want := range map[string]string{"sale": "FOR_SALE", "a-vendre": "FOR_SALE", "te-koop": "FOR_SALE", "rent": "FOR_RENT", "location": "FOR_RENT", "te-huur": "FOR_RENT"} {
		if got, err := NormalizeDeal(in); err != nil || got != want {
			t.Errorf("NormalizeDeal(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := NormalizeDeal("swap"); err == nil {
		t.Error("expected deal error")
	}
	for in, want := range map[string]string{"": "relevance", "newest": "newest", "most-expensive": "most_expensive", "cheapest": "cheapest"} {
		if got, err := NormalizeSort(in); err != nil || got != want {
			t.Errorf("NormalizeSort(%q) = %q, %v", in, got, err)
		}
	}
}

func TestCriteriaParamsAndValidate(t *testing.T) {
	c := Criteria{Types: []string{"APARTMENT"}, Deal: "FOR_RENT", PostalCodes: []string{"BE-1050"}, MaxPrice: 1500, MinBedrooms: 2, EPC: []string{"a", "b"}, Garden: true, Sort: "newest"}
	p := c.Params()
	want := map[string]string{"countries": "BE", "propertyTypes": "APARTMENT", "transactionTypes": "FOR_RENT", "postalCodes": "BE-1050", "maxPrice": "1500", "minBedroomCount": "2", "epcScores": "A,B", "hasGarden": "true", "orderBy": "newest"}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("Params = %v\nwant %v", p, want)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
	if err := (Criteria{Deal: "FOR_SALE"}).Validate(); err == nil || !strings.Contains(err.Error(), "property type") {
		t.Errorf("missing type should fail, got %v", err)
	}
	if err := (Criteria{Types: []string{"HOUSE"}, Deal: "FOR_SALE", MinPrice: 5, MaxPrice: 1}).Validate(); err == nil {
		t.Error("min > max should fail")
	}
	if err := (Criteria{Types: []string{"HOUSE"}, Deal: "FOR_SALE", EPC: []string{"Z"}}).Validate(); err == nil {
		t.Error("bad EPC should fail")
	}
	a := Criteria{Types: []string{"HOUSE"}, Deal: "FOR_SALE", Communes: []string{"Liège", "ixelles"}, Sort: "newest"}
	b := Criteria{Types: []string{"HOUSE"}, Deal: "FOR_SALE", Communes: []string{"Ixelles", "liege"}, Sort: "cheapest"}
	a.Types, b.Types = []string{"HOUSE", "APARTMENT"}, []string{"APARTMENT", "HOUSE"}
	if a.Key() != b.Key() {
		t.Errorf("Key should ignore sort/commune order/accents: %q vs %q", a.Key(), b.Key())
	}
}

func TestParseSearchURL(t *testing.T) {
	c, err := ParseSearchURL("https://www.immoweb.be/fr/recherche/appartement/a-louer/ixelles/1050?countries=BE&maxPrice=1500&minBedroomCount=2&orderBy=newest")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Types, []string{"APARTMENT"}) || c.Deal != "FOR_RENT" || !reflect.DeepEqual(c.PostalCodes, []string{"BE-1050"}) || c.MaxPrice != 1500 || c.MinBedrooms != 2 || c.Sort != "newest" {
		t.Errorf("unexpected criteria %+v", c)
	}
	c, err = ParseSearchURL("https://www.immoweb.be/nl/zoeken/huis-en-appartement/te-koop?postalCodes=BE-9000,BE-9050&hasGarden=true")
	if err != nil || !reflect.DeepEqual(c.Types, []string{"HOUSE", "APARTMENT"}) || c.Deal != "FOR_SALE" || len(c.PostalCodes) != 2 || !c.Garden {
		t.Errorf("nl url = %+v, %v", c, err)
	}
	if _, err := ParseSearchURL("https://www.example.com/fr/recherche/maison/a-vendre"); err == nil {
		t.Error("foreign host should fail")
	}
	if _, err := ParseSearchURL("https://www.immoweb.be/fr/page/vie-privee"); err == nil {
		t.Error("non-search URL should fail")
	}
}

func TestParseListingID(t *testing.T) {
	for in, want := range map[string]int64{
		"21828249": 21828249,
		"https://www.immoweb.be/fr/annonce/maison/a-vendre/dilbeek/1700/21828249":           21828249,
		"https://www.immoweb.be/en/classified/21828249?searchId=abc":                        21828249,
		"https://www.immoweb.be/nl/zoekertje/appartement/te-huur/elsene/1050/21836536#pics": 21836536,
	} {
		got, err := ParseListingID(in)
		if err != nil || got != want {
			t.Errorf("ParseListingID(%q) = %d, %v", in, got, err)
		}
	}
	if _, err := ParseListingID("ixelles"); err == nil {
		t.Error("expected error for non-ID")
	}
}

func TestFold(t *testing.T) {
	if Fold("  Liège ") != "liege" || Fold("Saint-Gilles") != "saint gilles" || Fold("saint  gilles") != "saint gilles" {
		t.Errorf("Fold mismatch: %q %q %q", Fold("  Liège "), Fold("Saint-Gilles"), Fold("saint  gilles"))
	}
}

func TestFromResultFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/rent-results.json")
	if err != nil {
		t.Fatal(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	l, err := FromResult(items[0])
	if err != nil {
		t.Fatal(err)
	}
	if l.ID != 21836536 || l.Deal != "FOR_RENT" || l.Price == nil || *l.Price != 1350 || l.RentCosts == nil || *l.RentCosts != 75 {
		t.Errorf("unexpected rent listing %+v", l)
	}
	if l.PostalCode != "1050" || l.Private || !strings.HasSuffix(l.URL, "/21836536") {
		t.Errorf("location/agency fields wrong: %+v", l)
	}
	if l.Surface != nil && l.PricePerSqm == nil {
		t.Error("price per m² should be derived when surface is known")
	}
	if _, err := FromResult(json.RawMessage(`{"property":{}}`)); err == nil {
		t.Error("listing without id must error")
	}
}

func TestFromClassifiedFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/classified.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := FromClassified(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 21828249 || d.Deal != "FOR_SALE" || d.Price == nil || *d.Price <= 0 {
		t.Fatalf("unexpected detail %+v", d.Listing)
	}
	if d.EPC == "" || d.EPCKwhPerSqm == nil || d.CadastralIncome == nil || d.ConstructionYear == nil {
		t.Errorf("detail enrichment missing: epc=%q kwh=%v ci=%v year=%v", d.EPC, d.EPCKwhPerSqm, d.CadastralIncome, d.ConstructionYear)
	}
	if d.Views == nil || d.Bookmarks == nil || len(d.Pictures) == 0 || d.CreatedAt == "" {
		t.Errorf("stats/pictures/created missing: %+v", d)
	}
	pics, err := PictureURLs(raw, "xl")
	if err != nil || len(pics) != len(d.Pictures) || !strings.Contains(pics[0], "2560x1440") {
		t.Errorf("PictureURLs xl = %v, %v", pics, err)
	}
	if _, err := PictureURLs(raw, "huge"); err == nil {
		t.Error("unknown size should fail")
	}
}

func TestAutocompleteAndPickCommune(t *testing.T) {
	raw, err := os.ReadFile("testdata/autocomplete-ixelles.json")
	if err != nil {
		t.Fatal(err)
	}
	ms, err := ParseAutocomplete(raw)
	if err != nil || len(ms) == 0 {
		t.Fatalf("ParseAutocomplete = %v, %v", ms, err)
	}
	m, ok := PickCommune("ixelles", ms)
	if !ok || !strings.Contains(m.QueryValue, "BE-1050") {
		t.Errorf("PickCommune(ixelles) = %+v, %v", m, ok)
	}
	if _, ok := PickCommune("zzzz", ms); ok {
		t.Error("unrelated name must not match")
	}
	synthetic := []LocationMatch{
		{Type: "DISTRICT", Label: "Liège (District)", QueryParam: "districts", QueryValue: "LIEGE"},
		{Type: "LOCALITY_NAME", Label: "Liège (all localities)", MainLocality: "Liège (all localities)", QueryParam: "postalCodes", QueryValue: "BE-4000,BE-4020"},
		{Type: "LOCALITY_NAME", Label: "Liège (4000)", MainLocality: "Liège", QueryParam: "postalCodes", QueryValue: "BE-4000"},
	}
	m, ok = PickCommune("liege", synthetic)
	if !ok || m.QueryValue != "BE-4000,BE-4020" || len(m.PostalCodes()) != 2 {
		t.Errorf("all-localities should win: %+v", m)
	}
}

func TestStats(t *testing.T) {
	if m, _ := Median([]float64{3, 1, 2}); m != 2 {
		t.Errorf("median odd = %v", m)
	}
	if m, _ := Median([]float64{4, 1, 2, 3}); m != 2.5 {
		t.Errorf("median even = %v", m)
	}
	if _, ok := Median(nil); ok {
		t.Error("empty median must be !ok")
	}
	if q, _ := Quantile([]float64{1, 2, 3, 4, 5}, 0.25); q != 2 {
		t.Errorf("q25 = %v", q)
	}
	if p, _ := PercentileRank([]float64{1, 2, 3, 4}, 1); p != 12.5 {
		t.Errorf("rank of min = %v", p)
	}
	if p, _ := PercentileRank([]float64{1, 2, 3, 4}, 5); p != 100 {
		t.Errorf("rank above all = %v", p)
	}
	if y, ok := GrossYield(1000, 240000); !ok || y != 5 {
		t.Errorf("yield = %v", y)
	}
	if _, ok := GrossYield(0, 1); ok {
		t.Error("zero rent yield must be !ok")
	}
	d := Haversine(50.8466, 4.3528, 50.8503, 4.3517)
	if d < 0.3 || d > 0.5 {
		t.Errorf("haversine = %v km", d)
	}
	two, zero, five := 2, 0, 5
	if BedroomBand(&two) != "2" || BedroomBand(&zero) != "studio" || BedroomBand(&five) != "4+" || BedroomBand(nil) != "unknown" {
		t.Error("bedroom bands wrong")
	}
}

func TestDaysListed(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	if d, ok := DaysListed("2026-09-03T12:00:00Z", now); !ok || d != 10 {
		t.Errorf("DaysListed = %d %v", d, ok)
	}
	if _, ok := DaysListed("", now); ok {
		t.Error("empty date must not report an age")
	}
}

func TestTriageScore(t *testing.T) {
	p10 := 10.0
	d2 := 2
	hi, factors := TriageScore(TriageInput{PricePercentile: &p10, DaysListed: &d2, PriceCut: true, Private: true, EPC: "A"})
	lo, _ := TriageScore(TriageInput{PricePercentile: func() *float64 { v := 95.0; return &v }(), DaysListed: func() *int { v := 90; return &v }(), EPC: "G"})
	if hi <= lo || hi > 100 || lo < 0 {
		t.Errorf("score ordering wrong: hi=%v lo=%v", hi, lo)
	}
	if len(factors) != 5 {
		t.Errorf("expected 5 factors, got %d", len(factors))
	}
	neutral, _ := TriageScore(TriageInput{})
	if neutral <= 0 || neutral >= hi {
		t.Errorf("neutral score %v should sit between", neutral)
	}
	if EPCRank("C")-EPCRank("A") != 2 || EPCRank("?") != 0 {
		t.Error("EPC rank wrong")
	}
}

func TestClassifiedAgencyNotPrivate(t *testing.T) {
	raw, _ := os.ReadFile("testdata/classified.json")
	d, err := FromClassified(raw)
	if err != nil {
		t.Fatal(err)
	}
	if d.Private || d.Agency == "" {
		t.Errorf("agency listing flagged private=%v agency=%q", d.Private, d.Agency)
	}
}

func TestPricePerSqmPlausibility(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	if p := PricePerSqm("FOR_RENT", f(1500), f(100)); p == nil || *p != 15 {
		t.Errorf("rent pps = %v", p)
	}
	if p := PricePerSqm("FOR_RENT", f(450), f(400)); p != nil {
		t.Errorf("implausible rent pps should be nil, got %v", *p)
	}
	if p := PricePerSqm("FOR_SALE", f(300000), f(100)); p == nil || *p != 3000 {
		t.Errorf("sale pps = %v", p)
	}
	if p := PricePerSqm("FOR_SALE", f(300000), f(5)); p != nil {
		t.Error("tiny surface should be nil")
	}
	if p := PricePerSqm("FOR_SALE", nil, f(100)); p != nil {
		t.Error("nil price should be nil")
	}
}

func TestPrivateSellerDetection(t *testing.T) {
	priv, err := FromResult(json.RawMessage(`{"id":1234567,"customerName":"PRIVATE","customerLogoUrl":null,"property":{"type":"APARTMENT"},"transaction":{"type":"FOR_RENT"}}`))
	if err != nil || !priv.Private || priv.Agency != "" {
		t.Errorf("PRIVATE customerName should be private with empty agency: %+v %v", priv, err)
	}
	company, _ := FromResult(json.RawMessage(`{"id":1234568,"customerName":"Colonies Belgium","customerLogoUrl":null,"property":{"type":"APARTMENT"},"transaction":{"type":"FOR_RENT"}}`))
	if company.Private || company.Agency != "Colonies Belgium" {
		t.Errorf("company without logo must not be private: %+v", company)
	}
}

func TestBrusselsPostcodes(t *testing.T) {
	raw, err := os.ReadFile("testdata/autocomplete-ixelles.json")
	if err != nil {
		t.Fatal(err)
	}
	matches, err := ParseAutocomplete(raw)
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := PickCommune("Ixelles", matches); !ok || m.QueryValue != "BE-1000,BE-1050" {
		t.Errorf("Immoweb's own grouping still spans 1000: %+v", m)
	}
	if codes := BrusselsPostcodes("Ixelles"); len(codes) != 1 || codes[0] != "BE-1050" || BrusselsLabel("ixelles", codes) != "Ixelles (1050)" {
		t.Errorf("Ixelles must drop the City of Brussels 1000: %v", codes)
	}
	if got := BrusselsPostcodes("Sint-Gillis"); len(got) != 1 || got[0] != "BE-1060" {
		t.Errorf("Dutch name not resolved: %v", got)
	}
	for in, want := range map[string]string{
		"saint-gilles":          "Saint-Gilles (1060)",
		"woluwe-saint-lambert":  "Woluwe-Saint-Lambert (1060)",
		"saint-josse-ten-noode": "Saint-Josse-ten-Noode (1060)",
		"ville de bruxelles":    "Ville de Bruxelles (1060)",
	} {
		if got := BrusselsLabel(in, []string{"BE-1060"}); got != want {
			t.Errorf("BrusselsLabel(%q) = %q, want %q", in, got, want)
		}
	}
	if BrusselsPostcodes("Liège") != nil {
		t.Error("non-Brussels commune must return nil")
	}
	if got := BrusselsPostcodes("bruxelles"); len(got) != 4 {
		t.Errorf("City of Brussels owns 1000,1020,1120,1130: %v", got)
	}
}

func TestIsRoomLetAndCleanLocality(t *testing.T) {
	p := func(v float64) *float64 { return &v }
	b := func(v int) *int { return &v }
	cases := []struct {
		l    Listing
		want bool
	}{
		{Listing{Deal: "FOR_RENT", Subtype: "KOT", Price: p(450), Bedrooms: b(1)}, true},
		{Listing{Deal: "FOR_SALE", Subtype: "KOT", Price: p(150000)}, true},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(400), Bedrooms: b(6)}, true},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(1100), Bedrooms: b(3)}, false},
		{Listing{Deal: "FOR_RENT", Subtype: "HOUSE", Price: p(700), Bedrooms: b(2), Title: "Colocation proche ULB"}, true},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(900), Bedrooms: b(2), Title: "Appartement 2 chambres"}, false},
		{Listing{Deal: "FOR_RENT", Subtype: "TRIPLEX", Price: p(1920), Bedrooms: b(3), Title: "Triplex 3 chambres – Idéal pour une colocation"}, false},
		{Listing{Deal: "FOR_RENT", Subtype: "HOUSE", Price: p(700), Bedrooms: b(1), Title: "Rooms in a beautiful shared house"}, true},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(540), Bedrooms: b(1), Title: "Studentroom near campus"}, true},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(1050), Bedrooms: b(1), Title: "BRUXELLES - Appartement 1 chambre à louer."}, false},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(1300), Bedrooms: b(2), Title: "Spacious 2 bedrooms in Ixelles"}, false},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(950), Bedrooms: b(1), Title: "1 bedroom for rent"}, false},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(1000), Bedrooms: b(1), Title: "Appartement met 1 slaapkamer te huur"}, false},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(1100), Bedrooms: b(1), Title: "Appartement 1 chambre meublée"}, false},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(450), Bedrooms: b(1), Title: "Chambre meublée à louer"}, true},
		{Listing{Deal: "FOR_RENT", Subtype: "APARTMENT", Price: p(400), Bedrooms: b(1), Title: "Kot étudiant"}, true},
		{Listing{Deal: "FOR_SALE", Subtype: "HOUSE", Price: p(300000), Bedrooms: b(6)}, false},
	}
	for i, c := range cases {
		if got := IsRoomLet(c.l); got != c.want {
			t.Errorf("case %d: IsRoomLet = %v, want %v", i, got, c.want)
		}
	}
	for in, want := range map[string]string{"IXELLES": "Ixelles", "Bruxelles  1": "Bruxelles 1", "SINT-GILLIS": "Sint-Gillis", "Liège": "Liège", "": ""} {
		if got := CleanLocality(in); got != want {
			t.Errorf("CleanLocality(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTriageFreshnessFallsBackToUpdate(t *testing.T) {
	d := 3
	_, f := TriageScore(TriageInput{DaysSinceUpdate: &d})
	if f[1].Factor != "freshness" || f[1].Value != 90 {
		t.Errorf("freshness fallback = %+v", f[1])
	}
	_, f = TriageScore(TriageInput{})
	if f[1].Value != 50 {
		t.Errorf("no dates must stay neutral: %+v", f[1])
	}
}

func TestNormalizeProvince(t *testing.T) {
	for in, want := range map[string]string{"brabant_wallon": "WALLOON_BRABANT", "WALLOON_BRABANT": "WALLOON_BRABANT", "Oost-Vlaanderen": "EAST_FLANDERS", "Liège": "LIEGE", "anvers": "ANTWERP"} {
		if got, err := NormalizeProvince(in); err != nil || got != want {
			t.Errorf("NormalizeProvince(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := NormalizeProvince("brabantwallon"); err == nil {
		t.Error("unknown province must be refused")
	}
}
