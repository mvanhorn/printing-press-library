package craigslist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeSearchBody_ipadFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "sapi-search-ipad.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	results, err := decodeSearchBody(body, "sfbay")
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got, want := len(results.Items), 360; got != want {
		t.Errorf("items: got %d, want %d", got, want)
	}
	if results.Hostname != "sfbay" {
		t.Errorf("hostname: got %q, want sfbay", results.Hostname)
	}
	if results.CategoryAbbr != "sss" {
		t.Errorf("categoryAbbr: got %q, want sss", results.CategoryAbbr)
	}
	if results.TotalResultCount <= 0 {
		t.Errorf("totalResultCount should be positive, got %d", results.TotalResultCount)
	}
	// First item known shape: uuid present, title present, slug present.
	if len(results.Items) == 0 {
		t.Fatal("no items decoded")
	}
	first := results.Items[0]
	if first.UUID == "" {
		t.Error("first item UUID empty")
	}
	if first.Title == "" {
		t.Error("first item Title empty")
	}
	if first.Slug == "" {
		t.Error("first item Slug empty")
	}
	if first.PostingID == 0 {
		t.Error("first item PostingID zero")
	}
	if first.CanonicalURL == "" || !strings.HasPrefix(first.CanonicalURL, "https://sfbay.craigslist.org/") {
		t.Errorf("first item CanonicalURL unexpected: %q", first.CanonicalURL)
	}
	// Most listings should have at least one image.
	withImages := 0
	for _, l := range results.Items {
		if len(l.Images) > 0 {
			withImages++
		}
	}
	if withImages < len(results.Items)/2 {
		t.Errorf("expected >50%% of items to have images, got %d/%d", withImages, len(results.Items))
	}
}

func TestSearchQueryValues_defaults(t *testing.T) {
	q := SearchQuery{Query: "ipad"}
	v := q.values()
	if got := v.Get("searchPath"); got != "sss" {
		t.Errorf("default searchPath: got %q, want sss", got)
	}
	if got := v.Get("cc"); got != "US" {
		t.Errorf("default cc: got %q, want US", got)
	}
	if got := v.Get("batch"); got != "1-0-360-0-0" {
		t.Errorf("default batch: got %q, want 1-0-360-0-0", got)
	}
	if got := v.Get("query"); got != "ipad" {
		t.Errorf("query: got %q, want ipad", got)
	}
	if v.Get("min_price") != "" {
		t.Error("zero min_price should not be set")
	}
}

func TestSearchQueryValues_filters(t *testing.T) {
	q := SearchQuery{
		Query:          "1BR",
		SearchPath:     "apa",
		MinPrice:       1500,
		MaxPrice:       3000,
		HasPic:         true,
		Postal:         "94110",
		SearchDistance: 25,
		TitleOnly:      true,
		Sort:           "date",
		Page:           2,
	}
	v := q.values()
	if v.Get("min_price") != "1500" {
		t.Errorf("min_price: %q", v.Get("min_price"))
	}
	if v.Get("max_price") != "3000" {
		t.Errorf("max_price: %q", v.Get("max_price"))
	}
	if v.Get("hasPic") != "1" {
		t.Errorf("hasPic: %q", v.Get("hasPic"))
	}
	if v.Get("srchType") != "T" {
		t.Errorf("srchType: %q", v.Get("srchType"))
	}
	if v.Get("postal") != "94110" {
		t.Errorf("postal: %q", v.Get("postal"))
	}
	if v.Get("search_distance") != "25" {
		t.Errorf("search_distance: %q", v.Get("search_distance"))
	}
	if v.Get("sort") != "date" {
		t.Errorf("sort: %q", v.Get("sort"))
	}
	if v.Get("batch") != "2-0-360-0-0" {
		t.Errorf("batch on page 2: %q", v.Get("batch"))
	}
}

func TestParseLocation_full(t *testing.T) {
	dec := &raw0Decode{
		// minimal stub — locations[1] = ["1","sfbay","sfc"]
		locations: []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`[1,"sfbay","sfc"]`),
		},
		neighborhoods: []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"SOMA / south beach"`),
		},
	}
	var l Listing
	parseLocation("1:1:1~37.7813~-122.402", &l, dec)
	if l.Subarea != "sfc" {
		t.Errorf("subarea: %q", l.Subarea)
	}
	if l.Neighborhood != "SOMA / south beach" {
		t.Errorf("neighborhood: %q", l.Neighborhood)
	}
	if l.Latitude < 37.78 || l.Latitude > 37.79 {
		t.Errorf("lat: %v", l.Latitude)
	}
	if l.Longitude < -122.41 || l.Longitude > -122.40 {
		t.Errorf("lng: %v", l.Longitude)
	}
}
