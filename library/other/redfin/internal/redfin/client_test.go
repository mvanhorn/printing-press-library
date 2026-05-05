// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package redfin

import (
	"bytes"
	"strings"
	"testing"
)

func TestStripStingrayPrefix(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want []byte
	}{
		{name: "with prefix", in: []byte(`{}&&{"version":1}`), want: []byte(`{"version":1}`)},
		{name: "without prefix", in: []byte(`{"version":1}`), want: []byte(`{"version":1}`)},
		{name: "short input", in: []byte(`{}`), want: []byte(`{}`)},
		{name: "empty", in: []byte{}, want: []byte{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := StripStingrayPrefix(c.in)
			if !bytes.Equal(got, c.want) {
				t.Errorf("StripStingrayPrefix(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestBuildSearchParams_DropsZeros(t *testing.T) {
	p := BuildSearchParams(SearchOptions{
		RegionID:   30772,
		RegionType: 6,
		Status:     1,
		BedsMin:    3,
		PriceMax:   600000,
	})
	if p["region_id"] != "30772" {
		t.Errorf("region_id = %q, want 30772", p["region_id"])
	}
	if p["region_type"] != "6" {
		t.Errorf("region_type = %q, want 6", p["region_type"])
	}
	if p["min_beds"] != "3" {
		t.Errorf("min_beds = %q, want 3", p["min_beds"])
	}
	if p["max_price"] != "600000" {
		t.Errorf("max_price = %q, want 600000", p["max_price"])
	}
	if _, ok := p["min_baths"]; ok {
		t.Errorf("min_baths should be dropped when zero")
	}
	if p["num_homes"] != "50" {
		t.Errorf("num_homes default = %q, want 50", p["num_homes"])
	}
	if p["page_number"] != "1" {
		t.Errorf("page_number default = %q, want 1", p["page_number"])
	}
}

func TestBuildSearchParams_NumHomesCapped(t *testing.T) {
	p := BuildSearchParams(SearchOptions{NumHomes: 9999})
	if p["num_homes"] != "350" {
		t.Errorf("num_homes cap = %q, want 350", p["num_homes"])
	}
}

func TestBuildSearchParams_UIPropertyTypes(t *testing.T) {
	p := BuildSearchParams(SearchOptions{UIPropertyTypes: []int{1, 2, 3}})
	if p["uipt"] != "1,2,3" {
		t.Errorf("uipt = %q, want 1,2,3", p["uipt"])
	}
}

const fixtureSearchResponse = `{}&&{"version":1,"errorMessage":"Success","resultCode":0,"payload":{"homes":[
  {"propertyId":12345,"listingId":67890,"url":"/TX/Austin/123-Main-St-78704/home/12345","mlsStatus":"Active",
   "price":{"value":499000,"level":1},"beds":3,"baths":2,"sqFt":{"value":1500,"level":1},
   "yearBuilt":{"value":1995,"level":1},"hoa":{"value":0,"level":1},"dom":{"value":7,"level":1},
   "streetLine":{"value":"123 Main St"},"city":"Austin","state":"TX","postalCode":"78704",
   "latitude":30.25,"longitude":-97.75,"timeOnRedfin":"P7D","mlsId":{"value":"MLS-1"}},
  {"propertyId":99,"listingId":1,"url":"/TX/Austin/foo/home/99","mlsStatus":"Sold",
   "price":{"value":600000,"level":1},"beds":4,"baths":3,
   "streetLine":{"value":"99 Foo Ln"},"city":"Austin","state":"TX","postalCode":"78704"}
]}}`

func TestParseSearchResponse(t *testing.T) {
	listings, err := ParseSearchResponse([]byte(fixtureSearchResponse))
	if err != nil {
		t.Fatalf("ParseSearchResponse: %v", err)
	}
	if len(listings) != 2 {
		t.Fatalf("len = %d, want 2", len(listings))
	}
	first := listings[0]
	if first.PropertyID != 12345 {
		t.Errorf("PropertyID = %d, want 12345", first.PropertyID)
	}
	if !strings.Contains(first.URL, "123-Main-St") {
		t.Errorf("URL = %q, want substring 123-Main-St", first.URL)
	}
	if first.Price != 499000 {
		t.Errorf("Price = %d, want 499000", first.Price)
	}
	if first.Beds != 3 {
		t.Errorf("Beds = %v, want 3", first.Beds)
	}
	if first.Sqft != 1500 {
		t.Errorf("Sqft = %d, want 1500", first.Sqft)
	}
	if first.Status != "Active" {
		t.Errorf("Status = %q, want Active", first.Status)
	}
	if first.Address.City != "Austin" {
		t.Errorf("City = %q, want Austin", first.Address.City)
	}
	if first.Address.Latitude == 0 {
		t.Errorf("Latitude should be populated, got 0")
	}
	if first.MLS != "MLS-1" {
		t.Errorf("MLS = %q, want MLS-1", first.MLS)
	}
	if listings[1].Status != "Sold" {
		t.Errorf("Status = %q, want Sold", listings[1].Status)
	}
}

const fixtureInitialInfo = `{}&&{"version":1,"errorMessage":"Success","resultCode":0,"payload":{
  "propertyId":12345,"listingId":67890,"url":"/TX/Austin/123-Main-St/home/12345",
  "streetAddress":{"streetLine":"123 Main St","city":"Austin","state":"TX","postalCode":"78704",
                   "latitude":30.25,"longitude":-97.75}
}}`

const fixtureAboveTheFold = `{}&&{"version":1,"errorMessage":"Success","resultCode":0,"payload":{
  "mainHouseInfo":{
    "propertyId":12345,"listingId":67890,"mlsStatus":"Active","mlsDisplayName":"ACTRIS",
    "price":{"value":499000,"level":1},"beds":3,"baths":2.5,"sqFt":{"value":1500,"level":1},
    "propertyType":"Single Family","listingAddedDate":"2025-01-15",
    "hoaDues":{"value":50,"level":1},"dom":{"value":7,"level":1},
    "lastSaleData":{"lastSoldDate":""}
  },
  "mediaBrowserInfoBySourceId":{"photos":[
    {"photoUrls":{"fullScreenPhotoUrl":"https://photos.redfin.com/p1.jpg"}},
    {"photoUrls":{"fullScreenPhotoUrl":"https://photos.redfin.com/p2.jpg"}}
  ]}
}}`

const fixtureBelowTheFold = `{}&&{"version":1,"errorMessage":"Success","resultCode":0,"payload":{
  "publicRecordsInfo":{"basicInfo":{"yearBuilt":1995,"lotSquareFootage":7200,"propertyType":"SFR"}},
  "propertyHistoryInfo":{"events":[
    {"eventDate":"2025-01-15","eventDescription":"Listed","price":{"value":499000},"source":"MLS"},
    {"eventDate":"2024-09-01","eventDescription":"Sold","price":{"value":420000},"source":"Public"}
  ]},
  "schoolsAndDistrictsInfo":{"servingThisHomeSchools":[
    {"name":"Travis Elementary","gradeRange":"K-5","greatSchoolsRating":8}
  ]},
  "avmInfo":{"predictedValue":{"value":510000,"level":1}}
}}`

func TestParseListingDetail(t *testing.T) {
	l, err := ParseListingDetail([]byte(fixtureInitialInfo), []byte(fixtureAboveTheFold), []byte(fixtureBelowTheFold))
	if err != nil {
		t.Fatalf("ParseListingDetail: %v", err)
	}
	if l.PropertyID != 12345 {
		t.Errorf("PropertyID = %d, want 12345", l.PropertyID)
	}
	if l.ListingID != 67890 {
		t.Errorf("ListingID = %d, want 67890", l.ListingID)
	}
	if l.Price != 499000 {
		t.Errorf("Price = %d, want 499000", l.Price)
	}
	if l.Beds != 3 {
		t.Errorf("Beds = %v, want 3", l.Beds)
	}
	if l.Baths != 2.5 {
		t.Errorf("Baths = %v, want 2.5", l.Baths)
	}
	if l.Sqft != 1500 {
		t.Errorf("Sqft = %d, want 1500", l.Sqft)
	}
	if l.YearBuilt != 1995 {
		t.Errorf("YearBuilt = %d, want 1995", l.YearBuilt)
	}
	if l.LotSize != 7200 {
		t.Errorf("LotSize = %d, want 7200", l.LotSize)
	}
	if l.HOA != 50 {
		t.Errorf("HOA = %d, want 50", l.HOA)
	}
	if l.Estimate != 510000 {
		t.Errorf("Estimate = %d, want 510000", l.Estimate)
	}
	if len(l.Photos) != 2 {
		t.Errorf("Photos len = %d, want 2", len(l.Photos))
	}
	if len(l.PriceHistory) != 2 {
		t.Errorf("PriceHistory len = %d, want 2", len(l.PriceHistory))
	}
	if len(l.Schools) != 1 || l.Schools[0].Name != "Travis Elementary" {
		t.Errorf("Schools = %+v, want Travis Elementary", l.Schools)
	}
	if l.Address.City != "Austin" {
		t.Errorf("Address.City = %q, want Austin", l.Address.City)
	}
}

const fixtureTrends = `{}&&{"version":1,"errorMessage":"Success","resultCode":0,"payload":{
  "months":[
    {"period":"2025-01","medianSalePrice":500000,"medianListPrice":510000,"medianDom":15,
     "monthsOfSupply":2.1,"medianSaleListRatio":0.98,"activeListings":120},
    {"period":"2025-02","medianSalePrice":505000,"medianListPrice":515000,"medianDom":12,
     "monthsOfSupply":1.9,"medianSaleListRatio":0.99,"activeListings":135}
  ]
}}`

func TestParseTrendsResponse(t *testing.T) {
	rows, err := ParseTrendsResponse([]byte(fixtureTrends), "Austin, TX", 30772)
	if err != nil {
		t.Fatalf("ParseTrendsResponse: %v", err)
	}
	// 2 months × 6 metrics = 12 rows
	if len(rows) != 12 {
		t.Fatalf("rows len = %d, want 12", len(rows))
	}
	for _, r := range rows {
		if r.Region != "Austin, TX" {
			t.Errorf("Region = %q, want Austin, TX", r.Region)
		}
		if r.RegionID != 30772 {
			t.Errorf("RegionID = %d, want 30772", r.RegionID)
		}
		if r.Month == "" {
			t.Errorf("Month should not be empty")
		}
	}
}

func TestValueOrInt(t *testing.T) {
	if v := valueOrInt([]byte(`{"value":42,"level":1}`)); v != 42 {
		t.Errorf("boxed: got %d, want 42", v)
	}
	if v := valueOrInt([]byte(`42`)); v != 42 {
		t.Errorf("scalar: got %d, want 42", v)
	}
	if v := valueOrInt(nil); v != 0 {
		t.Errorf("nil: got %d, want 0", v)
	}
}
