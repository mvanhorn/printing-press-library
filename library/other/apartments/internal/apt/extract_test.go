package apt

import (
	"strings"
	"testing"
)

func TestParsePlacards(t *testing.T) {
	htmlSrc := `
<html><body>
<ul class="placardContainer">
  <li>
    <a class="property-link placard js-placard"
       href="/the-domain-austin-tx/abc123/"
       data-beds="2" data-baths="2" data-maxrent="2400"
       title="The Domain Austin">The Domain Austin</a>
  </li>
  <li>
    <a class="property-link placard"
       href="https://www.apartments.com/another-place-tx/xyz789/"
       data-beds="Studio" data-baths="1" data-maxrent="1500">Another Place</a>
  </li>
  <li>
    <a class="not-a-card" href="/something/">Skip Me</a>
  </li>
</ul>
</body></html>`

	got, err := ParsePlacards([]byte(htmlSrc), "https://www.apartments.com")
	if err != nil {
		t.Fatalf("ParsePlacards: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 placards, got %d: %+v", len(got), got)
	}

	if !strings.HasPrefix(got[0].URL, "https://www.apartments.com/") {
		t.Errorf("first URL not absolute: %q", got[0].URL)
	}
	if got[0].PropertyID != "abc123" {
		t.Errorf("first PropertyID: got %q want abc123", got[0].PropertyID)
	}
	if got[0].Beds != 2 || got[0].Baths != 2 || got[0].MaxRent != 2400 {
		t.Errorf("first numbers: %+v", got[0])
	}
	if got[0].Title == "" {
		t.Errorf("first Title empty")
	}

	// Studio => 0
	if got[1].Beds != 0 {
		t.Errorf("second beds: got %d want 0 (studio)", got[1].Beds)
	}
	if got[1].PropertyID != "xyz789" {
		t.Errorf("second PropertyID: got %q want xyz789", got[1].PropertyID)
	}
}

func TestParseListing(t *testing.T) {
	htmlSrc := `
<html><head><title>The Domain Apartments — Austin TX</title></head>
<body itemscope itemtype="http://schema.org/ApartmentComplex">
  <meta itemprop="streetAddress" content="123 Main St" />
  <meta itemprop="addressLocality" content="Austin" />
  <meta itemprop="addressRegion" content="TX" />
  <meta itemprop="postalCode" content="78704" />
  <meta itemprop="telephone" content="555-555-1212" />
  <div data-beds="2" data-baths="2" data-maxrent="2400" data-sqft-min="950">
    Floor area
  </div>
  <ul class="amenitiesList">
    <li>Washer/Dryer In Unit</li>
    <li>Cats Allowed</li>
    <li>Dogs Allowed</li>
  </ul>
  <table>
    <tr data-name="A1" data-beds-min="1" data-baths-min="1" data-sqft-min="700"
        data-rent-min="1400" data-rent-max="1600">
      <td>Plan A1</td>
    </tr>
    <tr data-name="B2" data-beds-min="2" data-baths-min="2" data-sqft-min="950"
        data-rent-min="2200" data-rent-max="2400">
      <td>Plan B2</td>
    </tr>
  </table>
</body></html>`

	got, err := ParseListing([]byte(htmlSrc), "https://www.apartments.com/the-domain-austin-tx/abc123/")
	if err != nil {
		t.Fatalf("ParseListing: %v", err)
	}
	if got.PropertyID != "abc123" {
		t.Errorf("PropertyID: got %q want abc123", got.PropertyID)
	}
	if got.Address.StreetAddress != "123 Main St" || got.Address.City != "Austin" || got.Address.State != "TX" || got.Address.PostalCode != "78704" {
		t.Errorf("Address: %+v", got.Address)
	}
	if got.Beds != 2 || got.Baths != 2 || got.MaxRent != 2400 || got.Sqft != 950 {
		t.Errorf("numbers: beds=%d baths=%v max=%d sqft=%d", got.Beds, got.Baths, got.MaxRent, got.Sqft)
	}
	if got.Phone != "555-555-1212" {
		t.Errorf("Phone: %q", got.Phone)
	}
	if len(got.Amenities) != 3 {
		t.Errorf("Amenities count: %d (%v)", len(got.Amenities), got.Amenities)
	}
	if !got.PetPolicy.AllowsCats || !got.PetPolicy.AllowsDogs {
		t.Errorf("pet policy not inferred from amenities: %+v", got.PetPolicy)
	}
	if len(got.FloorPlans) != 2 {
		t.Fatalf("FloorPlans: got %d want 2 (%+v)", len(got.FloorPlans), got.FloorPlans)
	}
	if got.FloorPlans[0].Name != "A1" || got.FloorPlans[0].RentMin != 1400 || got.FloorPlans[0].RentMax != 1600 {
		t.Errorf("FloorPlan[0]: %+v", got.FloorPlans[0])
	}
	if got.FloorPlans[1].Sqft != 950 {
		t.Errorf("FloorPlan[1].Sqft: %d", got.FloorPlans[1].Sqft)
	}
}

func TestListingURLToPropertyID(t *testing.T) {
	cases := map[string]string{
		"https://www.apartments.com/the-domain-austin-tx/abc123/": "abc123",
		"https://www.apartments.com/the-domain-austin-tx/abc123":  "abc123",
		"/the-domain-austin-tx/abc123/":                           "abc123",
		"":                                                        "",
		"https://www.apartments.com/":                             "",
	}
	for in, want := range cases {
		if got := ListingURLToPropertyID(in); got != want {
			t.Errorf("ListingURLToPropertyID(%q) = %q want %q", in, got, want)
		}
	}
}
