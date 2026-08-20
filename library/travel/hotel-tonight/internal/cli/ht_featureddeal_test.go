package cli

import "testing"

// ssrFixture mimics the relevant slice of a results page: the embedded
// (backslash-escaped) featured_deal object with a nested rating_options array,
// plus a hotel object carrying the selected_hotel_id and its name.
const ssrFixture = `<html>...` +
	`\"id\":18,\"name\":\"Hotel Griffon\",\"slug\":\"hotel-griffon-in-san-francisco-ca\"` +
	`...other hotels...` +
	`\"featured_deal\":{\"type\":\"DAILY_DROP\",\"action_text\":\"Slide to unlock your deal\",` +
	`\"description\":\"1 unlock per day\",\"expires_at\":null,\"ht_price\":137,` +
	`\"image_url\":\"https://x/thunder_deal.webp\",\"savings_amount\":58,\"state\":\"available\",` +
	`\"title\":\"Today's Daily Drop\",` +
	`\"unlock_url\":\"https://api.hoteltonight.com/v6/featured_deal?selected_hotel_id=18&start_date=2026-05-20\",` +
	`\"rating_options\":[{\"id\":\"price_too_high\",\"message\":\"Price is too high\"}]}` +
	`,\"more\":true</html>`

func TestExtractEscapedJSONObject(t *testing.T) {
	obj := extractEscapedJSONObject(ssrFixture, `\"featured_deal\":`)
	if obj == "" {
		t.Fatal("expected to extract featured_deal object")
	}
	// Must include the nested rating_options closing brace AND the outer close,
	// i.e. brace-counting did not stop at the first inner `}`.
	if obj[len(obj)-1] != '}' {
		t.Errorf("object should end at outer brace, got tail %q", obj[len(obj)-20:])
	}
	if !contains(obj, "rating_options") {
		t.Error("expected nested rating_options to be included in the object")
	}
	// Must not bleed into the trailing ,"more":true.
	if contains(obj, "more") {
		t.Error("object should not include content past its closing brace")
	}
}

func TestParseFeaturedDeal(t *testing.T) {
	fd, ok, err := parseFeaturedDeal(ssrFixture)
	if err != nil {
		t.Fatalf("parseFeaturedDeal err = %v", err)
	}
	if !ok {
		t.Fatal("expected an available Daily Drop")
	}
	if fd.Type != "DAILY_DROP" {
		t.Errorf("type = %q, want DAILY_DROP", fd.Type)
	}
	if fd.HotelID != "18" {
		t.Errorf("hotel id = %q, want 18", fd.HotelID)
	}
	if fd.HotelName != "Hotel Griffon" {
		t.Errorf("hotel name = %q, want Hotel Griffon", fd.HotelName)
	}
	view := fd.view()
	if view.Price != 137 {
		t.Errorf("price = %v, want 137", view.Price)
	}
	if view.Was != 195 { // 137 + 58
		t.Errorf("was = %v, want 195", view.Was)
	}
	if view.PctOff != 30 { // round(58/195*100)
		t.Errorf("pct_off = %v, want 30", view.PctOff)
	}
}

func TestParseFeaturedDealUnavailable(t *testing.T) {
	page := `\"featured_deal\":{\"type\":\"DAILY_DROP\",\"state\":\"unavailable\",\"ht_price\":0,\"unlock_url\":\"x\"}`
	_, ok, err := parseFeaturedDeal(page)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if ok {
		t.Error("unavailable deal should report ok=false")
	}
}

func TestParseFeaturedDealMissing(t *testing.T) {
	_, ok, err := parseFeaturedDeal(`<html>no deal here</html>`)
	if err != nil || ok {
		t.Errorf("missing deal: ok=%v err=%v, want ok=false err=nil", ok, err)
	}
}

// TestParseFeaturedDealBraceInString guards the string-aware brace counter: a
// `{` or `}` inside a string value must not be mistaken for an object boundary.
func TestParseFeaturedDealBraceInString(t *testing.T) {
	page := `x\"featured_deal\":{\"type\":\"DAILY_DROP\",\"state\":\"available\",` +
		`\"title\":\"Tonight {only} deal }\",\"ht_price\":100,\"savings_amount\":25,` +
		`\"unlock_url\":\"https://x?selected_hotel_id=7\"}` +
		`,\"id\":7,\"name\":\"The Curly Hotel\"x`
	fd, ok, err := parseFeaturedDeal(page)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !ok {
		t.Fatal("expected available deal despite braces in the title string")
	}
	if numF(fd.HTPrice) != 100 {
		t.Errorf("ht_price = %v, want 100 (object was truncated at an in-string brace)", numF(fd.HTPrice))
	}
	if fd.HotelName != "The Curly Hotel" {
		t.Errorf("hotel name = %q, want The Curly Hotel", fd.HotelName)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
