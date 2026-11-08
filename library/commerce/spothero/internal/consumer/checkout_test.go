package consumer

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/session"
)

//go:embed testdata/checkout_request_live_shape.json
var checkoutRequestFixture []byte

//go:embed testdata/users_me_live_shape.json
var usersMeFixture []byte

//go:embed testdata/users_vehicles_live_shape.json
var usersVehiclesFixture []byte

//go:embed testdata/users_credit_cards_live_shape.json
var usersCreditCardsFixture []byte

//go:embed testdata/search_transient_facility_live_shape.json
var facilityQuoteFixture []byte

//go:embed testdata/search_transient_facility_outside_hours_shape.json
var facilityOutsideHoursFixture []byte

func envelopeData(data any) []byte {
	b, _ := json.Marshal(Envelope{Data: data})
	return b
}

func TestBuildCheckoutLiveShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search/transient/6698":
			_, _ = w.Write(facilityQuoteFixture)
		case "/users/me/":
			_, _ = w.Write(envelopeData(json.RawMessage(usersMeFixture)))
		case "/users/42/credit-cards/":
			_, _ = w.Write(envelopeData(json.RawMessage(usersCreditCardsFixture)))
		case "/users/42/vehicles/":
			_, _ = w.Write(envelopeData(json.RawMessage(usersVehiclesFixture)))
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	c := New(&session.Session{
		Cookies: map[string]string{"sessionid": "abc", "csrftoken": "csrf"},
	})
	c.CraigBaseURL = srv.URL
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()

	req, _, err := c.BuildCheckout(BookPlaceInput{
		FacilityID: 6698,
		Starts:     "2099-09-15T09:00",
		Ends:       "2099-09-15T17:00",
		CitySlug:   "san-francisco",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var gotMap map[string]any
	if err := json.Unmarshal(got, &gotMap); err != nil {
		t.Fatal(err)
	}
	if gotMap["currency"] != "usd" || gotMap["total_price"].(float64) != 2968 {
		t.Fatalf("currency/total=%v", gotMap)
	}
	payment := gotMap["payment"].(map[string]any)
	if payment["use_spothero_credit"] != false {
		t.Fatalf("use_spothero_credit=%v", payment["use_spothero_credit"])
	}
	cards := payment["cards"].([]any)
	if cards[0].(map[string]any)["card_external_id"] != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("cards=%v", payment["cards"])
	}
	if _, ok := gotMap["cards"]; ok {
		t.Fatal("unexpected top-level cards")
	}
	items := gotMap["items"].([]any)
	item := items[0].(map[string]any)
	// The site moved the quote binding into item_context. Sending the old item level
	// fields is an unknown contract, so assert they are gone.
	for _, gone := range []string{"rate_id", "quote_token", "quote_mac"} {
		if _, ok := item[gone]; ok {
			t.Fatalf("item still carries %s", gone)
		}
	}
	if item["key"] == "" || item["key"] == nil {
		t.Fatal("item needs a client key")
	}
	if addOns, ok := item["add_ons"].([]any); !ok || len(addOns) != 0 {
		t.Fatalf("add_ons must be an empty array, got %v", item["add_ons"])
	}
	ctx := item["item_context"].(map[string]any)
	if ctx["facility"].(float64) != 6698 || ctx["vehicle_profile_id"].(float64) != 20000001 {
		t.Fatalf("context=%v", ctx)
	}
	if ctx["phone_number"] != "+14155550100" || ctx["rental_source_title"] != "web" {
		t.Fatalf("contact/context=%v", ctx)
	}
	if ctx["quote_token"] != "REDACTED-QUOTE-TOKEN" || ctx["search_id"] != "REDACTED-SEARCH-ID" {
		t.Fatalf("tracking/context=%v", ctx)
	}
}

func TestBuildCheckoutOutsideHours(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/search/transient/144293" {
			_, _ = w.Write(facilityOutsideHoursFixture)
			return
		}
		t.Fatalf("unexpected %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	c := New(&session.Session{Cookies: map[string]string{"sessionid": "abc"}})
	c.CraigBaseURL = srv.URL
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()

	_, _, err := c.BuildCheckout(BookPlaceInput{
		FacilityID: 144293,
		Starts:     "2099-09-14T09:00",
		Ends:       "2099-09-14T17:00",
		CitySlug:   "chicago",
	})
	if err == nil {
		t.Fatal("expected outside hours error")
	}
	if !strings.Contains(err.Error(), "facility not available: Outside Hours") {
		t.Fatalf("err=%q", err.Error())
	}
}

func TestBuildCheckoutUnknownRateID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search/transient/6698":
			_, _ = w.Write(facilityQuoteFixture)
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	c := New(&session.Session{Cookies: map[string]string{"sessionid": "abc"}})
	c.CraigBaseURL = srv.URL
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()

	_, _, err := c.BuildCheckout(BookPlaceInput{
		FacilityID:   6698,
		Starts:       "2099-09-15T09:00",
		Ends:         "2099-09-15T17:00",
		CitySlug:     "san-francisco",
		SelectRateID: "does-not-exist",
	})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestCheckoutSetsCSRFAndVersion(t *testing.T) {
	var gotCSRF, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkout/" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		gotCSRF = r.Header.Get("X-CSRFToken")
		gotVersion = r.Header.Get("SpotHero-Version")
		_, _ = w.Write(envelopeData(map[string]any{"reservation_id": 1}))
	}))
	t.Cleanup(srv.Close)
	c := New(&session.Session{Cookies: map[string]string{"csrftoken": "tok"}})
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	_, err := c.Checkout(CheckoutRequest{
		TotalPrice: 100,
		Currency:   "usd",
		Email:      "a@b.com",
		Payment: CheckoutPayment{
			UseSpotHeroCredit: false,
			Cards:             []CheckoutCard{{CardExternalID: "00000000-0000-4000-8000-000000000009"}},
		},
		Items: []CheckoutItem{{
			ItemType: "rental",
			Price:    100,
			Key:      "k",
			AddOns:   []any{},
			ItemContext: CheckoutItemContext{
				Facility:          1,
				Starts:            "2099-09-15T09:00:00-07:00",
				Ends:              "2099-09-15T17:00:00-07:00",
				PhoneNumber:       "+14155550100",
				QuoteToken:        "qt",
				RentalSourceTitle: "web",
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotCSRF != "tok" || gotVersion != ConsumerCheckoutVersion {
		t.Fatalf("csrf=%q version=%q", gotCSRF, gotVersion)
	}
}
