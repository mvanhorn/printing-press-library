package consumer_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/exitcode"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/session"
)

func envelope(data any) []byte {
	b, _ := json.Marshal(consumer.Envelope{Data: data})
	return b
}

func testClient(t *testing.T, handler http.HandlerFunc) *consumer.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	sess := &session.Session{RawCookieHeader: "sessionid=abc"}
	c := consumer.New(sess)
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	return c
}

func TestSearchParams(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search-params/" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write(envelope(map[string]any{
			"latitude": 41.88, "longitude": -87.62,
			"starts": "2099-09-11T09:30", "ends": "2099-09-11T12:30",
			"sort": "distance", "sort_order": "asc", "distance_lt": 1609.0,
		}))
	})
	params, err := c.GetSearchParams(consumer.SearchQuery{
		Latitude: 41.88, Longitude: -87.62,
		Starts: "2099-09-11T09:30", Ends: "2099-09-11T12:30",
	})
	if err != nil {
		t.Fatal(err)
	}
	if params.Latitude != 41.88 {
		t.Fatalf("lat=%v", params.Latitude)
	}
}

func TestNotAuthenticated(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(envelope(map[string]any{
			"errors": []map[string]any{{
				"code": "not_authenticated", "messages": []string{"Authentication credentials were not provided."},
			}},
		}))
	})
	_, err := c.GetUser()
	if err == nil {
		t.Fatal("expected error")
	}
	var ex *exitcode.Error
	if !exitcode.As(err, &ex) || ex.Code != exitcode.Auth {
		t.Fatalf("code=%v err=%v", ex, err)
	}
}

func TestCheckoutDryRun(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not hit network in dry-run")
	})
	c.DryRun = true
	_, err := c.Checkout(consumer.CheckoutRequest{
		TotalPrice: 100,
		Currency:   "usd",
		Email:      "a@b.com",
		Payment: consumer.CheckoutPayment{
			UseSpotHeroCredit: false,
			Cards:             []consumer.CheckoutCard{{CardExternalID: "test-card-uuid"}},
		},
		Items: []consumer.CheckoutItem{{
			ItemType: "rental", Price: 100, Key: "k",
			ItemContext: consumer.CheckoutItemContext{
				Facility: 1, Starts: "2099-09-15T09:00:00-07:00", Ends: "2099-09-15T17:00:00-07:00",
				PhoneNumber: "+14155550100", RentalSourceTitle: "web", QuoteToken: "qt",
			},
		}},
	})
	if err == nil {
		t.Fatal("expected dry-run error")
	}
}

func TestListReservationsLiveShape(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reservations/" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		_, _ = w.Write(envelope(map[string]any{
			"results": []map[string]any{{
				"rental_id":      100000001,
				"display_id":     "100000001",
				"price":          1908,
				"status":         "success",
				"is_cancellable": true,
				"facility":       map[string]any{"title": "Garage"},
			}},
		}))
	})
	list, err := c.ListReservations()
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ID != "100000001" || list[0].PriceCents != 1908 {
		t.Fatalf("%+v", list[0])
	}
}

func TestProbeSessionAuthUsesPageSize(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page_size") != "1" {
			t.Fatalf("page_size=%q", r.URL.Query().Get("page_size"))
		}
		_, _ = w.Write(envelope(map[string]any{"results": []any{}}))
	})
	if err := c.ProbeSessionAuth(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchIntegration(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search-params/":
			_, _ = w.Write(envelope(map[string]any{
				"google_place": map[string]any{"place_id": "p"}, "latitude": 41.88, "longitude": -87.62,
				"starts": "2099-09-11T09:30", "ends": "2099-09-11T12:30",
				"sort": "distance", "sort_order": "asc", "distance_lt": 1609.0,
				"page_info": map[string]any{"setup": map[string]any{"city": map[string]any{"slug": "chicago"}}},
			}))
		case "/search/transient":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results":[{"distance":{"walking_meters":100},"rates":[{"quote":{"total_price":{"value":1200}}}],"availability":{"available":true},"facility":{"common":{"id":"99","title":"Lot","status":"on_sales_allowed","addresses":[]}}}]}`))
		default:
			t.Fatalf("unexpected %s", r.URL.Path)
		}
	})
	c.CraigBaseURL = c.BaseURL
	res, err := c.Search(consumer.SearchQuery{
		Address: "Chicago, IL",
		Starts:  "2099-09-11T09:30",
		Ends:    "2099-09-11T12:30",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Params.Latitude != 41.88 || len(res.Results) != 1 {
		t.Fatalf("params=%+v results=%d", res.Params, len(res.Results))
	}
}
