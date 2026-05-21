package loopnet

import "testing"

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"$1,999,000", 1999000},
		{"Offered at $1,200,000 in Worcester", 1200000},
		{"$1,891.20/SF", 1891.20},
		{"Price Upon Request", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := ParseMoney(c.in); got != c.want {
			t.Errorf("ParseMoney(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseSqftAndPercent(t *testing.T) {
	if got := ParseSqft("1,057 SF Office Building"); got != 1057 {
		t.Errorf("ParseSqft = %v, want 1057", got)
	}
	if got := ParseSqft("0.30 AC Lot"); got != 0 {
		t.Errorf("ParseSqft(no SF) = %v, want 0", got)
	}
	if got := ParsePercent("Cap Rate 6.5%"); got != 6.5 {
		t.Errorf("ParsePercent = %v, want 6.5", got)
	}
}

func TestNormalizeType(t *testing.T) {
	cases := map[string]string{
		"office":           "office",
		"Office Buildings": "office",
		"warehouse":        "industrial",
		"apartments":       "multifamily",
		"all":              "commercial-real-estate",
		"land":             "land",
		"some-exact-slug":  "some-exact-slug",
	}
	for in, want := range cases {
		if got := NormalizeType(in); got != want {
			t.Errorf("NormalizeType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeListingType(t *testing.T) {
	cases := map[string]string{
		"for-sale": "for-sale", "sale": "for-sale", "": "for-sale",
		"for-lease": "for-lease", "lease": "for-lease", "rent": "for-lease",
		"businesses-for-sale": "businesses-for-sale",
	}
	for in, want := range cases {
		if got := NormalizeListingType(in); got != want {
			t.Errorf("NormalizeListingType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugLocationAndMarketKey(t *testing.T) {
	if got := SlugLocation("Worcester, MA"); got != "worcester-ma" {
		t.Errorf("SlugLocation = %q, want worcester-ma", got)
	}
	if got := SlugLocation("  Los Angeles   CA "); got != "los-angeles-ca" {
		t.Errorf("SlugLocation(spaces) = %q, want los-angeles-ca", got)
	}
	if got := MarketKey("Worcester, MA", "warehouse", "lease"); got != "worcester-ma|industrial|for-lease" {
		t.Errorf("MarketKey = %q", got)
	}
}

func TestBuildSearchURL(t *testing.T) {
	got := BuildSearchURL("office", "Worcester, MA", "for-sale", 1, SearchFilters{})
	want := "https://www.loopnet.com/search/office/worcester-ma/for-sale/"
	if got != want {
		t.Errorf("BuildSearchURL = %q, want %q", got, want)
	}
	page2 := BuildSearchURL("industrial", "ma", "lease", 2, SearchFilters{MinPrice: 500000})
	if page2 != "https://www.loopnet.com/search/industrial/ma/for-lease/2/?min-price=500000" {
		t.Errorf("BuildSearchURL page2 = %q", page2)
	}
}

func TestListingIDFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.loopnet.com/Listing/2035-W-15th-St-Long-Beach-CA/38523625/": "38523625",
		"https://www.loopnet.com/Listing/12345/":                                 "12345",
		"https://www.loopnet.com/search/office/":                                 "",
	}
	for in, want := range cases {
		if got := ListingIDFromURL(in); got != want {
			t.Errorf("ListingIDFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHasDistressSignal(t *testing.T) {
	if ok, kw := HasDistressSignal("Great building, PRICE REDUCED this week"); !ok || kw != "price reduced" {
		t.Errorf("HasDistressSignal(price reduced) = %v, %q", ok, kw)
	}
	if ok, _ := HasDistressSignal("A well-maintained office building"); ok {
		t.Errorf("HasDistressSignal(clean text) = true, want false")
	}
}

const searchFixture = `<html><head>
<script type="application/ld+json">
{"@type":"CollectionPage","mainEntity":{"numberOfItems":2,"itemListElement":[
{"@type":"RealEstateListing","name":"PII_ADDRESS_EXAMPLE, Worcester, MA 01608","description":"5,000 SF Office Building Offered at $1,200,000 in Worcester, MA","url":"https://www.loopnet.com/Listing/100-Main-St-Worcester/12345/","image":"https://img/a.jpg","offers":[{"@type":"Offer","price":"1200000","offeredBy":{"@type":"Person","name":"Jane Broker","worksFor":{"@type":"Organization","name":"Acme Realty"}}}]},
{"@type":"RealEstateListing","name":"PII_ADDRESS_EXAMPLE","description":"Industrial property","url":"https://www.loopnet.com/Listing/200-Oak/67890/","offers":[{"@type":"Offer","price":2500000}]}
]}}
</script></head><body>
<a data-automation-id="NextPage">Next</a></body></html>`

func TestParseSearchHTML(t *testing.T) {
	res, err := ParseSearchHTML([]byte(searchFixture))
	if err != nil {
		t.Fatalf("ParseSearchHTML error: %v", err)
	}
	if len(res.Listings) != 2 {
		t.Fatalf("got %d listings, want 2", len(res.Listings))
	}
	if !res.HasNextPage {
		t.Error("HasNextPage = false, want true")
	}
	l := res.Listings[0]
	if l.ID != "12345" {
		t.Errorf("listing ID = %q, want 12345", l.ID)
	}
	if l.Price != 1200000 {
		t.Errorf("listing price = %v, want 1200000", l.Price)
	}
	if l.SizeSqft != 5000 {
		t.Errorf("listing size = %v, want 5000", l.SizeSqft)
	}
	if l.BrokerName != "Jane Broker" || l.BrokerCompany != "Acme Realty" {
		t.Errorf("broker = %q / %q", l.BrokerName, l.BrokerCompany)
	}
	if res.Listings[1].Price != 2500000 {
		t.Errorf("second listing price = %v, want 2500000", res.Listings[1].Price)
	}
}

const detailFixture = `<html><head>
<script type="application/ld+json">
{"@type":["RealEstateListing","Product"],"name":"PII_ADDRESS_EXAMPLE","description":"Price reduced. Motivated seller.","url":"https://www.loopnet.com/Listing/100-Main-St/12345/","dateModified":"2026-05-01","offers":[{"@type":"Offer","price":1200000}],"provider":[{"@type":"RealEstateAgent","name":"Jane Broker","memberOf":{"@type":"Organization","name":"Acme Realty"}}],"contentLocation":{"address":{"streetAddress":"PII_ADDRESS_EXAMPLE","addressLocality":"Worcester","addressRegion":"MA","postalCode":"01608"}},"additionalProperty":[{"@type":"PropertyValue","name":"Cap Rate","value":["6.5%"]},{"@type":"PropertyValue","name":"Building Class","value":["B"]},{"@type":"PropertyValue","name":"Year Built","value":["1988"]}]}
</script></head><body>
<table>
<tr><td data-fact-type="TotalAssessment">Total Assessment</td><td data-fact-type="TotalAssessment">$900,000</td></tr>
<tr><td data-fact-type="LandAssessment">Land Assessment</td><td data-fact-type="LandAssessment">$300,000</td></tr>
<tr><td data-fact-type="ParcelNumber">Parcel Number</td><td data-fact-type="ParcelNumber">12-345-678</td></tr>
</table></body></html>`

func TestParseDetailHTML(t *testing.T) {
	p, err := ParseDetailHTML([]byte(detailFixture))
	if err != nil {
		t.Fatalf("ParseDetailHTML error: %v", err)
	}
	if p.ID != "12345" {
		t.Errorf("ID = %q, want 12345", p.ID)
	}
	if p.Price != 1200000 {
		t.Errorf("Price = %v, want 1200000", p.Price)
	}
	if p.City != "Worcester" || p.State != "MA" || p.Zip != "01608" {
		t.Errorf("address = %q/%q/%q", p.City, p.State, p.Zip)
	}
	if p.CapRate != 6.5 {
		t.Errorf("CapRate = %v, want 6.5", p.CapRate)
	}
	if p.BuildingClass != "B" {
		t.Errorf("BuildingClass = %q, want B", p.BuildingClass)
	}
	if p.YearBuilt != "1988" {
		t.Errorf("YearBuilt = %q, want 1988", p.YearBuilt)
	}
	if p.TotalAssessment != 900000 {
		t.Errorf("TotalAssessment = %v, want 900000", p.TotalAssessment)
	}
	if p.LandAssessment != 300000 {
		t.Errorf("LandAssessment = %v, want 300000", p.LandAssessment)
	}
	if len(p.ParcelNumbers) == 0 || p.ParcelNumbers[0] != "12-345-678" {
		t.Errorf("ParcelNumbers = %v", p.ParcelNumbers)
	}
	if p.BrokerName != "Jane Broker" {
		t.Errorf("BrokerName = %q", p.BrokerName)
	}
	if ok, _ := HasDistressSignal(p.Description); !ok {
		t.Error("expected distress signal in detail description")
	}
}
