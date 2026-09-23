package immovlan

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseSearchFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/search.html")
	if err != nil {
		t.Fatal(err)
	}
	sp, err := ParseSearch(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(sp.Items) != 20 {
		t.Fatalf("cards = %d, want 20", len(sp.Items))
	}
	if sp.LastPage != 3 || sp.Page != 1 || !sp.HasNext {
		t.Errorf("pagination = page %d, last %d, next %v; want 1, 3, true", sp.Page, sp.LastPage, sp.HasNext)
	}
	last := strings.Replace(string(body), `class="v3-pagination-btn active" href="/fr/immobilier?transactiontypes=a-vendre`, `class="v3-pagination-btn active" href="/fr/immobilier?page=3&transactiontypes=a-vendre`, 1)
	if lp, err := ParseSearch([]byte(last)); err != nil || lp.Page != 3 || lp.HasNext {
		t.Errorf("active page 3 must have no next: page %d next %v err %v", lp.Page, lp.HasNext, err)
	}
	if e, _ := ParseSearch([]byte("<html><body><p>rien</p></body></html>")); len(e.Items) != 0 || e.HasNext || e.LastPage != 1 {
		t.Errorf("empty page = %+v", e)
	}
	first := sp.Items[0]
	if first.ID != "VBE69761" || first.Deal != DealSale || first.Type != "maison" || first.PostalCode != "1030" || first.Locality != "Schaerbeek" {
		t.Errorf("first card = %+v", first)
	}
	if first.Price == nil || *first.Price != 1290000 || first.EPC != "F" || first.EPCBand != "bad" || first.Flag != "new" {
		t.Errorf("price/epc/flag = %v %q %q %q", first.Price, first.EPC, first.EPCBand, first.Flag)
	}
	if first.Bedrooms == nil || *first.Bedrooms != 6 || first.Surface == nil || *first.Surface != 631 || first.Bathrooms == nil || *first.Bathrooms != 2 {
		t.Errorf("pills = %v %v %v", first.Bedrooms, first.Surface, first.Bathrooms)
	}
	if first.Private || first.SellerType != "estateAgents" || first.AgencyID != "48202" || first.PricePerSqm == nil {
		t.Errorf("seller = %+v", first)
	}
	if first.URL != "https://immovlan.be/fr/detail/maison/a-vendre/1030/schaerbeek/vbe69761" {
		t.Errorf("url = %s", first.URL)
	}
	letters := 0
	for _, l := range sp.Items {
		if l.EPC != "" {
			letters++
		}
	}
	if letters < 15 {
		t.Errorf("only %d/20 cards carry a PEB letter", letters)
	}
}

func TestParseDetailFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/detail.html")
	if err != nil {
		t.Fatal(err)
	}
	d, err := ParseDetail(body)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "VBE69490" || d.Deal != DealSale || d.Type != "maison" || d.PostalCode != "1030" {
		t.Errorf("identity = %+v", d.Listing)
	}
	if d.Price == nil || *d.Price != 725000 || d.Surface == nil || *d.Surface != 285 || d.EPC != "C" {
		t.Errorf("price/surface/epc = %v %v %q", d.Price, d.Surface, d.EPC)
	}
	if d.Street != "Rue Thiéfry 54" || d.Lat == nil || d.Lng == nil || d.CreatedAt != "2026-09-19T18:39:35Z" {
		t.Errorf("address/geo/date = %q %v %v %q", d.Street, d.Lat, d.Lng, d.CreatedAt)
	}
	if d.Year == nil || *d.Year != 1886 || d.Bedrooms == nil || *d.Bedrooms != 5 || d.Bathrooms == nil || *d.Bathrooms != 3 {
		t.Errorf("year/rooms = %v %v %v", d.Year, d.Bedrooms, d.Bathrooms)
	}
	if d.Condition != "Remis à neuf" || d.Rented == nil || *d.Rented || d.CadastralIncome == nil || *d.CadastralIncome != 1455 {
		t.Errorf("features = %q %v %v", d.Condition, d.Rented, d.CadastralIncome)
	}
	if d.Garden == nil || *d.Garden != 46 || d.Land == nil || *d.Land != 130 || d.Heating != "Gaz" {
		t.Errorf("surfaces/heating = %v %v %q", d.Garden, d.Land, d.Heating)
	}
	if d.Agency != "Eventimmo" || d.AgencyID != "27443" || d.SellerType != "estateAgents" || d.Private || d.Software != "Omnicasa" {
		t.Errorf("seller = %q %q %q %v %q", d.Agency, d.AgencyID, d.SellerType, d.Private, d.Software)
	}
	if len(d.Photos) < 2 || d.PricePerSqm == nil {
		t.Errorf("photos/pps = %d %v", len(d.Photos), d.PricePerSqm)
	}
	if days, ok := DaysListed(d.CreatedAt, time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)); !ok || days != 2 {
		t.Errorf("days listed = %d %v", days, ok)
	}
}

func TestParseDetailWithoutListing(t *testing.T) {
	if _, err := ParseDetail([]byte("<html><body><h1>Oups</h1></body></html>")); err != ErrNoListing {
		t.Errorf("expected ErrNoListing, got %v", err)
	}
	// dataLayer-only page (no JSON-LD): identity still comes through; a bogus reference is rejected.
	page := `<html><head><script>dataLayer.push({"property_type":"apartment","transaction_type":"rent","zip_code":"1210","vlan_code":"rbu21531","seller_type":"private","price":"950.00"} || {});</script></head><body></body></html>`
	d, err := ParseDetail([]byte(page))
	if err != nil || d.ID != "RBU21531" || d.Type != "appartement" || d.Deal != DealRent || d.PostalCode != "1210" || !d.Private || d.Price == nil || *d.Price != 950 {
		t.Errorf("dataLayer identity = %+v %v", d.Listing, err)
	}
	bad := strings.Replace(page, `"rbu21531"`, `"/../../evil\n"`, 1)
	if _, err := ParseDetail([]byte(bad)); err != ErrNoListing {
		t.Errorf("invalid vlan_code must be rejected, got %v", err)
	}
	ld := `<html><head><script type="application/ld+json">{"@type":"RealEstateListing","name":"Immeuble mixte à vendre  à Schaerbeek","url":"https://immovlan.be/fr/detail/immeuble-mixte/a-vendre/1030/schaerbeek/vbe1234","mainEntity":{"@type":"House"}}</script></head><body></body></html>`
	if d, err := ParseDetail([]byte(ld)); err != nil || d.ID != "VBE1234" || d.Type != "maison" || d.Title != "Immeuble mixte à vendre à Schaerbeek" {
		t.Errorf("subtype slug must not blank the type: %+v %v", d.Listing, err)
	}
}

func TestCriteria(t *testing.T) {
	c := Criteria{Types: []string{"maison", "appartement"}, Deal: DealSale, PostalCodes: []string{"1030", "1210"}, EPC: []string{"bad"}, MinPrice: 500000, MaxPrice: 2500000, SortBy: "price", SortDir: "descending"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	p := c.Params()
	if p["transactiontypes"] != "a-vendre" || p["towns"] != "1030,1210" || p["epcratings"] != "bad" || p["minprice"] != "500000" || p["sortby"] != "price" || p["sortdirection"] != "descending" {
		t.Errorf("params = %v", p)
	}
	k1 := c.Key()
	c2 := c
	c2.PostalCodes = []string{"1210", "1030"}
	c2.SortBy, c2.SortDir = "", ""
	if k1 != c2.Key() {
		t.Errorf("key must ignore list order and sort: %s vs %s", k1, c2.Key())
	}
	if err := (Criteria{Deal: DealSale}).Validate(); err == nil {
		t.Error("location must be required")
	}
	for in, want := range map[string]string{"F": "bad", "g": "bad", "D": "poor", "b": "good", "A+": "excellent", "poor": "poor"} {
		if got, _ := EPCBand(in); got != want {
			t.Errorf("EPCBand(%s) = %s", in, got)
		}
	}
	if d, _ := NormalizeDeal("en-vente-publique"); d != DealPublicSale {
		t.Error("public sale slug")
	}
	if ty, _ := NormalizeType("HOUSE"); ty != "maison" {
		t.Error("HOUSE → maison")
	}
	if f, dir, _ := NormalizeSort("price_desc"); f != "price" || dir != "descending" {
		t.Errorf("sort = %s %s", f, dir)
	}
	if pcs := TownsPostcodes([]string{"1030-schaerbeek", "liege", "4000-liege"}); len(pcs) != 2 || pcs[1] != "4000" {
		t.Errorf("TownsPostcodes = %v", pcs)
	}
	if l := BrusselsPostcodeList(); len(l) != 22 || l[0] != "1000" || l[21] != "1210" {
		t.Errorf("BrusselsPostcodeList = %v", l)
	}
	if TownSlug("1210", "Saint-Josse-ten-Noode") != "1210-saint-josse-ten-noode" {
		t.Error(TownSlug("1210", "Saint-Josse-ten-Noode"))
	}
}

func TestParseSearchURL(t *testing.T) {
	c, err := ParseSearchURL("https://immovlan.be/fr/immobilier?transactiontypes=a-vendre&propertytypes=maison,appartement&towns=1030-schaerbeek&epcratings=bad,poor&minprice=500000&maxprice=2500000&page=2")
	if err != nil {
		t.Fatal(err)
	}
	if c.Deal != DealSale || len(c.Types) != 2 || c.Towns[0] != "1030-schaerbeek" || len(c.EPC) != 2 || c.MaxPrice != 2500000 {
		t.Errorf("parsed = %+v", c)
	}
	if _, err := ParseSearchURL("https://www.immoweb.be/en/search"); err == nil {
		t.Error("foreign host must fail")
	}
}

func TestReferenceAndBrussels(t *testing.T) {
	for in, want := range map[string]string{"vbe69761": "VBE69761", "https://immovlan.be/fr/detail/maison/a-vendre/1030/schaerbeek/vbe69761?x=1": "VBE69761", "RBU21531": "RBU21531"} {
		if got, err := ParseReference(in); err != nil || got != want {
			t.Errorf("ParseReference(%s) = %s %v", in, got, err)
		}
	}
	if _, err := ParseReference("12345"); err == nil {
		t.Error("numeric id must fail")
	}
	if c := BrusselsPostcodes("Saint-Josse"); len(c) != 1 || c[0] != "1210" {
		t.Errorf("saint-josse = %v", c)
	}
	if c := BrusselsPostcodes("Bruxelles"); len(c) != 4 {
		t.Errorf("bruxelles = %v", c)
	}
	if BrusselsCommune("1030") != "Schaerbeek" || BrusselsPostcodes("Liège") != nil {
		t.Error("commune lookup")
	}
}

func TestPureParsers(t *testing.T) {
	for in, want := range map[string]string{"BrusselsF": "F", "FlandersAPLUS": "A+", "WallonieAPLUSPLUS": "A++", "BruxellesG": "G", "BrusselsZ": "", "Brussels": "", "BrusselsF\x1b]": "", "Nope": ""} {
		if got := epcFromClass(in); got != want {
			t.Errorf("epcFromClass(%q) = %q, want %q", in, got, want)
		}
	}
	f := func(v float64) *float64 { return &v }
	if p := PricePerSqm(DealSale, f(500000), f(200)); p == nil || *p != 2500 {
		t.Errorf("sale pps = %v", p)
	}
	if p := PricePerSqm(DealSale, f(500000), f(5)); p != nil {
		t.Error("tiny surface must give nil")
	}
	if p := PricePerSqm(DealRent, f(1000), f(80)); p == nil || *p != 12.5 {
		t.Errorf("rent pps = %v", p)
	}
	if p := PricePerSqm(DealRent, f(500000), f(80)); p != nil {
		t.Error("implausible rent pps must give nil")
	}
	if p := PricePerSqm(DealSale, f(100000), f(30)); p == nil || *p != 3333.3 {
		t.Errorf("rounding = %v", p)
	}
	for in, want := range map[string]string{"2026-09-19T18:39:35.1234567": "2026-09-19T18:39:35Z", "2026-09-19": "2026-09-19T00:00:00Z", "yesterday": "", "\x1b[31m2026": ""} {
		if got := normalizeDate(in); got != want {
			t.Errorf("normalizeDate(%q) = %q", in, got)
		}
	}
	for in, want := range map[string]float64{"1.290.000 €": 1290000, "725 000€": 725000, "1 095 000 €": 1095000, "950,50 €": 950} {
		if p := parseEUR(in); p == nil || *p != want {
			t.Errorf("parseEUR(%q) = %v", in, p)
		}
	}
	if parseEUR("Prix sur demande") != nil {
		t.Error("no digits must give nil")
	}
	if !immovlanHost("https://immovlan.be/fr/detail/x") || immovlanHost("https://www.immoweb.be/x") || immovlanHost("http://immovlan.be/x") {
		t.Error("immovlanHost")
	}
}
