package zameen

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

func TestExtractState(t *testing.T) {
	html := []byte(`<html><head><script>window.foo=1;</script></head>` +
		`<body><script>window.state = {"algolia":{"content":{"nbHits":42,"nbPages":2,` +
		`"hits":[{"externalID":"123","title":"Nice { house }","price":1000000,"area":209.03,` +
		`"rooms":3,"baths":2,"isVerified":true,"slug":"nice-house-123-9-1",` +
		`"geography":{"lat":33.6,"lng":73.0},` +
		`"category":[{"name":"Homes","level":0}],` +
		`"location":[{"name":"Pakistan","externalID":"1","level":0},{"name":"Islamabad","externalID":"3","level":2},{"name":"DHA Defence","externalID":"9","level":3}],` +
		`"agency":{"name":"Acme Estates"}}]}}};</script></body></html>`)
	state, err := extractState(html)
	if err != nil {
		t.Fatalf("extractState: %v", err)
	}
	var env stateEnvelope
	if err := json.Unmarshal(state, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Algolia.Content.NbHits != 42 || env.Algolia.Content.NbPages != 2 {
		t.Fatalf("meta: got nbHits=%d nbPages=%d", env.Algolia.Content.NbHits, env.Algolia.Content.NbPages)
	}
	if len(env.Algolia.Content.Hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(env.Algolia.Content.Hits))
	}
}

func TestExtractStateMissing(t *testing.T) {
	if _, err := extractState([]byte("<html>no state here</html>")); err == nil {
		t.Fatal("expected error when window.state is absent")
	}
}

func TestHitToListing(t *testing.T) {
	h := hit{
		ExternalID: flexString("123"),
		Title:      "House &amp; Home",
		Purpose:    "for-sale",
		Price:      1000000,
		Area:       418.06368, // exactly 20 Marla
		Rooms:      5,
		Baths:      4,
		IsVerified: true,
		Slug:       "some-house-123-9-1",
	}
	h.Category = []locCat{{Name: "Homes", Level: 0}}
	h.Location = []locCat{
		{Name: "Pakistan", ExternalID: "1", Level: 0},
		{Name: "Lahore", ExternalID: "1", Level: 2},
		{Name: "DHA Defence", ExternalID: "9", Level: 3},
		{Name: "Phase 6", ExternalID: "99", Level: 5},
	}
	h.Geography.Lat = 31.4
	h.Geography.Lng = 74.4
	h.Agency.Name = "Acme Estates"

	l := h.toListing()
	if l.ExternalId != "123" {
		t.Errorf("external id: %q", l.ExternalId)
	}
	if l.City != "Lahore" {
		t.Errorf("city: %q", l.City)
	}
	if l.Location != "DHA Defence" {
		t.Errorf("area (level 3): %q", l.Location)
	}
	if l.AreaMarla < 19.99 || l.AreaMarla > 20.01 {
		t.Errorf("area marla: got %v, want ~20", l.AreaMarla)
	}
	if l.Beds != 5 || l.Baths != 4 {
		t.Errorf("beds/baths: %d/%d", l.Beds, l.Baths)
	}
	if l.Agency != "Acme Estates" {
		t.Errorf("agency: %q", l.Agency)
	}
	if l.Url != BaseURL+"/Property/some-house-123-9-1.html" {
		t.Errorf("url: %q", l.Url)
	}
}

func TestFlexStringNumberOrString(t *testing.T) {
	var fs flexString
	if err := json.Unmarshal([]byte(`"abc"`), &fs); err != nil || string(fs) != "abc" {
		t.Fatalf("string: %v %q", err, fs)
	}
	if err := json.Unmarshal([]byte(`12345`), &fs); err != nil || string(fs) != "12345" {
		t.Fatalf("number: %v %q", err, fs)
	}
	if err := json.Unmarshal([]byte(`null`), &fs); err != nil || string(fs) != "" {
		t.Fatalf("null: %v %q", err, fs)
	}
}

func TestResolveLocation(t *testing.T) {
	cases := []struct {
		city, location, want string
		wantErr              bool
	}{
		{"Islamabad", "", "Islamabad-3", false},
		{"lahore", "", "Lahore-1", false},
		{"", "Lahore_DHA_Defence-9", "Lahore_DHA_Defence-9", false},
		{"", "", "", true},
		{"Neverland", "", "", true},
	}
	for _, c := range cases {
		got, err := ResolveLocation(c.city, c.location)
		if c.wantErr {
			if err == nil {
				t.Errorf("ResolveLocation(%q,%q): want error", c.city, c.location)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ResolveLocation(%q,%q) = %q, %v; want %q", c.city, c.location, got, err, c.want)
		}
	}
}

func TestResolveCategory(t *testing.T) {
	cases := []struct{ purpose, ptype, want string }{
		{"buy", "Homes", "Homes"},
		{"buy", "Plots", "Plots"},
		{"buy", "Commercial", "Commercial"},
		{"rent", "Homes", "Rentals"},
		{"rent", "Plots", "Rentals"},
	}
	for _, c := range cases {
		if got := ResolveCategory(c.purpose, c.ptype); got != c.want {
			t.Errorf("ResolveCategory(%q,%q) = %q; want %q", c.purpose, c.ptype, got, c.want)
		}
	}
}

func TestParamsMatches(t *testing.T) {
	h := hit{Price: 5_000_000, Rooms: 3, Baths: 2, Area: 209, IsVerified: false}
	h.Category = []locCat{{Name: "Homes"}}
	h.Location = []locCat{{Name: "DHA Defence", Level: 3}}

	if !(SearchParams{MinPrice: 1_000_000, MaxPrice: 10_000_000, MinBeds: 2}).matches(h) {
		t.Error("should match within price/bed range")
	}
	if (SearchParams{MaxPrice: 1_000_000}).matches(h) {
		t.Error("should reject above max price")
	}
	if (SearchParams{MinBeds: 4}).matches(h) {
		t.Error("should reject below min beds")
	}
	if (SearchParams{VerifiedOnly: true}).matches(h) {
		t.Error("should reject unverified when VerifiedOnly")
	}
	if !(SearchParams{Area: "dha"}).matches(h) {
		t.Error("should match area substring 'dha'")
	}
	if (SearchParams{Area: "bahria"}).matches(h) {
		t.Error("should reject non-matching area")
	}
}

func TestSortListings(t *testing.T) {
	items := []types.Listing{
		{ExternalId: "a", Price: 300, CreatedAt: 10},
		{ExternalId: "b", Price: 100, CreatedAt: 30},
		{ExternalId: "c", Price: 200, CreatedAt: 20},
	}
	sortListings(items, "price-asc")
	if items[0].Price != 100 || items[2].Price != 300 {
		t.Errorf("price-asc failed: %+v", items)
	}
	sortListings(items, "newest")
	if items[0].CreatedAt != 30 {
		t.Errorf("newest failed: %+v", items)
	}
}
