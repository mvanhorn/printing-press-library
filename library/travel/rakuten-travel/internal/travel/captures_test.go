package travel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Optional offline acceptance against uncommitted discovery captures. Ordinary
// tests use focused synthetic fixtures and never contact a remote service.
func TestLocalCapturedPages(t *testing.T) {
	dir := os.Getenv("TRAVEL_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set TRAVEL_CAPTURE_DIR to verify discovery HTML offline")
	}
	read := func(t *testing.T, name, url string) document {
		t.Helper()
		body, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		return doc(body, url)
	}
	t.Run("property3", func(t *testing.T) {
		h, e := parseHotel(read(t, "public-sample-3.html", "https://travel.rakuten.co.jp/HOTEL/51870/51870.html"), "51870", false)
		if e != nil || h.Name == nil || !strings.Contains(*h.Name, "ハートン") || h.Rating == nil || h.Rating.Score != 4.27 || h.Coordinates != nil {
			t.Fatalf("property %#v %v", h, e)
		}
	})
	t.Run("facilities9", func(t *testing.T) {
		h, e := parseHotel(read(t, "public-sample-9-facilities.html", "https://travel.rakuten.co.jp/HOTEL/51870/51870_std.html"), "51870", true)
		if e != nil || len(h.HotelAmenities) < 5 || len(h.RoomAmenities) < 5 || len(h.Access) == 0 || len(h.Parking) == 0 || len(h.Notes) == 0 || len(h.CancellationPolicy) == 0 || h.PolicyCaveat == nil {
			t.Fatalf("facilities %#v %v", h, e)
		}
		if !strings.Contains(strings.Join(h.Notes, " "), "宿泊税を含んでいません") {
			t.Fatal("accommodation-tax exclusion lost")
		}
	})
	for _, tc := range []struct{ name, file, query string }{{"Japanese4", "public-sample-4.html", "品川"}, {"English5", "public-sample-5.html", "Shinagawa"}} {
		t.Run(tc.name, func(t *testing.T) {
			q := HotelQuery{Query: tc.query, Page: 1, Limit: 5}
			r, e := parseHotelSearch(read(t, tc.file, "https://kw.travel.rakuten.co.jp/keyword/Search.do"), q)
			if e != nil || len(r.Hotels) == 0 || r.Hotels[0].Name == nil || r.Page.SourceTotal == nil || !r.Page.HasMore {
				t.Fatalf("search %#v %v", r, e)
			}
		})
	}
	t.Run("area11", func(t *testing.T) {
		q := HotelQuery{Area: "tokyo/E", Page: 2, Limit: 5}
		r, e := parseHotelSearch(read(t, "public-sample-11-area-page2.html", "https://search.travel.rakuten.co.jp/ds/yado/tokyo/E-p2"), q)
		if e != nil || len(r.Hotels) != 5 || !r.Page.HasMore || r.Page.SourceTotal == nil {
			t.Fatalf("area2 %#v %v", r, e)
		}
	})
	for _, tc := range []struct {
		name, file, checkout string
		rooms, infant        int
		want                 int64
	}{{"one-night6", "public-sample-6.html", "2026-11-09", 1, 0, 10160}, {"two-night7", "public-sample-7.html", "2026-11-10", 1, 0, 20720}, {"multiroom8", "public-sample-8-multiroom.html", "2026-11-10", 2, 0, 20720}, {"infant10", "public-sample-10-infant.html", "2026-11-10", 1, 1, 20720}} {
		t.Run(tc.name, func(t *testing.T) {
			q := sampleQuery()
			q.Checkout = tc.checkout
			q.Rooms = tc.rooms
			q.Children.InfantNone = tc.infant
			r, e := parseOffers(read(t, tc.file, offerURL(q)), q)
			if e != nil || len(r.Offers) == 0 {
				t.Fatalf("captured offer %v %s", e, r.Status)
			}
			o := r.Offers[0]
			if o.PlanID != "3951989" || o.Price.PerRoomStayJPY != tc.want || o.PlanName == nil || o.Description == nil || o.Meals.Breakfast == nil || *o.Meals.Breakfast || o.Meals.Dinner == nil || *o.Meals.Dinner || o.Query.Rooms != tc.rooms || o.Query.Children.InfantNone != tc.infant {
				t.Fatalf("captured semantics %#v", o)
			}
			if tc.rooms == 2 && o.RoomID != "ns-semi-db" {
				t.Fatal("insufficient remaining room variant was included", o.RoomID)
			}
		})
	}
}
