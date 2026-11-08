package consumer

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/session"
)

func validCheckout() CheckoutRequest {
	return CheckoutRequest{
		TotalPrice: 100, Currency: "usd", Email: "user@example.com",
		Payment: CheckoutPayment{Cards: []CheckoutCard{{CardExternalID: "card"}}},
		Items: []CheckoutItem{{ItemType: "rental", Price: 100, Key: "k", AddOns: []any{}, ItemContext: CheckoutItemContext{
			Facility: 1, Starts: "2099-01-15T09:00:00-06:00", Ends: "2099-01-15T12:00:00-06:00",
			PhoneNumber: "+14155550100", QuoteToken: "qt", RentalSourceTitle: "web",
		}}},
	}
}

func clientFor(srv *httptest.Server) *Client {
	c := New(&session.Session{Cookies: map[string]string{"AUTH_TOKEN": "x"}})
	c.BaseURL, c.CraigBaseURL, c.HTTP = srv.URL, srv.URL, srv.Client()
	return c
}

// The worst failure here is a double charge, so a server error after the request was
// sent must surface as ambiguous, with exactly one attempt.
func TestCheckoutServerErrorIsAmbiguousAndNotRetried(t *testing.T) {
	posts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	_, err := clientFor(srv).Checkout(validCheckout())
	var amb *AmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("err = %v, want ambiguous", err)
	}
	if posts != 1 {
		t.Fatalf("posts = %d, want exactly 1", posts)
	}
}

func TestCheckoutDroppedConnectionIsAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()
	_, err := clientFor(srv).Checkout(validCheckout())
	var amb *AmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("err = %v, want ambiguous", err)
	}
}

// A validation refusal is a definite no, and must not read as "maybe charged".
func TestCheckoutClientErrorIsNotAmbiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"data":{"errors":[{"code":"invalid","messages":["quote expired"]}]}}`))
	}))
	defer srv.Close()
	_, err := clientFor(srv).Checkout(validCheckout())
	var amb *AmbiguousError
	if err == nil || errors.As(err, &amb) {
		t.Fatalf("err = %v, want a definite failure", err)
	}
}

// Pins the body the site's cancel dialog sends. The first version of this CLI posted
// an empty body, which predates the current contract.
func TestRefundBodyMatchesSite(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reservations/5/refund/" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	if _, err := clientFor(srv).Refund("5", RefundRequest{Reason: "No longer need parking"}); err != nil {
		t.Fatal(err)
	}
	if body["reason"] != "No longer need parking" || body["refund_as_credit"] != false {
		t.Fatalf("body = %v", body)
	}
	if _, ok := body["access_key"]; ok {
		t.Fatal("empty access_key should be omitted")
	}
}

func TestRefundEligibilityParsesLiveShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("SpotHero-Version") != ConsumerAccountVersion {
			t.Errorf("version = %q", r.Header.Get("SpotHero-Version"))
		}
		_, _ = w.Write([]byte(`{"data":{"original_source_refund":{"is_eligible":false},"credit_only_refund":{"is_eligible":true},"self_service_rebook":{},"cancellation_reasons":[{"id":1,"reason":"Found street parking","reason_displayed_to_customer":"Found street parking"}]}}`))
	}))
	defer srv.Close()
	e, err := clientFor(srv).GetRefundEligibility("5")
	if err != nil {
		t.Fatal(err)
	}
	if e.OriginalSource || !e.CreditOnly || !e.Eligible() || len(e.Reasons) != 1 {
		t.Fatalf("%+v", e)
	}
}

// Craig is public: the session stays on spothero.com.
func TestCraigRequestsCarryNoSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			t.Errorf("session cookie sent to the inventory host")
		}
		_, _ = w.Write([]byte(`{"tracking":{},"result":{"rates":[]}}`))
	}))
	defer srv.Close()
	_, _ = clientFor(srv).searchTransientFacilityGET(1, "2099-01-15T15:00:00Z", "2099-01-15T18:00:00Z")
}

// Live payloads nest the plate in an object. This shape broke the vehicle list outright.
func TestParseVehiclesLiveShape(t *testing.T) {
	got, err := parseVehicles(json.RawMessage(`{"results":[{"id":7,"license_plate":{"id":1,"value":"ABC123","state":"CA"},"info":{"display_description":"Make Model"},"is_default":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != 7 || got[0].PlateLast2 != "23" || got[0].LicensePlateState != "CA" || got[0].Description != "Make Model" {
		t.Fatalf("%+v", got[0])
	}
}

// cancellation_allowed is a string on the live API; a bool here failed every list.
func TestParseReservationsLiveTypes(t *testing.T) {
	got, err := parseReservationsPayload(json.RawMessage(`{"results":[{"rental_id":5,"facility_id":9,"reservation_status":"valid","status":"success","cancellation_allowed":"customer","price":100,"facility":{"id":9,"title":"G"},"starts":"2099-01-15T15:00:00Z"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Status != "valid" || !got[0].Cancellable || got[0].FacilityTitle != "G" {
		t.Fatalf("%+v", got[0])
	}
}

// An id that matches nothing must not fall through to charging the default card.
func TestUnknownCardIDFails(t *testing.T) {
	me := UserAccount{DefaultCardID: 1, CreditCards: []CreditCard{{CardID: 1, CardExternalID: "default", IsDefault: true}}}
	if _, err := resolveCardExternalID(me, "", 99); err == nil {
		t.Fatal("unknown card id charged a different card")
	}
	if got, err := resolveCardExternalID(me, "", 0); err != nil || got != "default" {
		t.Fatalf("default: %q %v", got, err)
	}
}

func TestFacilityTimeZoneDrivesNaiveTimes(t *testing.T) {
	var gotStarts string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s := r.URL.Query().Get("starts"); s != "" {
			gotStarts = s
		}
		_, _ = w.Write([]byte(`{"tracking":{},"result":{"facility":{"common":{"addresses":[{"time_zone":"America/Chicago"}]}},"rates":[]}}`))
	}))
	defer srv.Close()
	if _, err := clientFor(srv).GetFacilityRates(FacilityRateQuery{FacilityID: 1, Starts: "2099-01-15T09:00", Ends: "2099-01-15T12:00"}); err != nil {
		t.Fatal(err)
	}
	if gotStarts != "2099-01-15T15:00:00Z" {
		t.Fatalf("starts = %q", gotStarts)
	}
}

func TestCheckoutSucceeded(t *testing.T) {
	for _, tc := range []struct {
		in   map[string]any
		want bool
	}{
		{map[string]any{}, false},
		{map[string]any{"order_id": ""}, false},
		{map[string]any{"order_id": "o"}, true},
		{map[string]any{"purchases": []any{map[string]any{"rental_id": 1}}}, true},
		{map[string]any{"purchases": []any{}}, false},
	} {
		if CheckoutSucceeded(tc.in) != tc.want {
			t.Errorf("%v", tc.in)
		}
	}
}

func TestValidateWindow(t *testing.T) {
	if err := validateWindow([]SearchPeriod{{Starts: "2099-01-02T00:00:00Z", Ends: "2099-01-01T00:00:00Z"}}); err == nil {
		t.Fatal("reversed window accepted")
	}
	if err := validateWindow([]SearchPeriod{{Starts: "2001-01-01T00:00:00Z", Ends: "2001-01-02T00:00:00Z"}}); err == nil {
		t.Fatal("past window accepted")
	}
	if err := validateWindow([]SearchPeriod{{Starts: "2099-01-01T00:00:00Z", Ends: "2099-01-02T00:00:00Z"}}); err != nil {
		t.Fatal(err)
	}
}

func TestLocationWarning(t *testing.T) {
	if locationWarning("233 S Wacker Dr, Chicago, IL", "chicago") != "" {
		t.Fatal("false warning")
	}
	if locationWarning("Toronto", "pittsburgh") == "" {
		t.Fatal("wrong city passed silently")
	}
	if locationWarning("", "chicago") != "" {
		t.Fatal("coordinates cannot be compared")
	}
}

// A next link comes from a response, so it must not steer this client off the host.
func TestSearchNextPageStaysOnInventoryHost(t *testing.T) {
	c := New(nil)
	c.CraigBaseURL = "https://inventory.example/v2"
	if _, err := c.searchNextPage("https://elsewhere.example/v2/search/transient?page=2"); err == nil {
		t.Fatal("followed a link off the inventory host")
	}
}
