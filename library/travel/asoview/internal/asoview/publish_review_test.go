package asoview

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Synthetic public-contract responses exercise complete domain reads; they are
// regression evidence, not live-source acceptance.
func TestAvailabilityPreservesUniformSlotStates(t *testing.T) {
	closed := Object{"isClosed": true, "remainReserveNumber": 100}
	soldOut := Object{"remainReserveNumber": 0}
	unknown := Object{}
	request := Object{"isRequest": true}
	available := Object{"remainReserveNumber": 10}
	cases := []struct {
		name     string
		slots    []Object
		quantity int
		want     string
	}{
		{"closed", []Object{closed, closed}, 1, "closed"},
		{"sold_out", []Object{soldOut, soldOut}, 1, "sold_out"},
		{"deadline", []Object{{"isRequestDeadlinePassed": true}, {"isRequestDeadlinePassed": true}}, 1, "deadline_passed"},
		{"quantity", []Object{{"remainReserveNumber": 1}, {"remainReserveNumber": 1}}, 2, "insufficient_quantity"},
		{"party_limits", []Object{{"minimumReservableQuantity": 3}, {"maximumReservableQuantity": 1}}, 2, "party_outside_limits"},
		{"mixed_unavailable", []Object{closed, soldOut}, 1, "unavailable"},
		{"unknown_stock", []Object{soldOut, unknown}, 1, "unknown"},
		{"unknown_closed", []Object{unknown, closed}, 1, "unknown"},
		{"request_closed", []Object{closed, request}, 1, "request_only"},
		{"request_unknown", []Object{request, unknown}, 1, "request_only"},
		{"available_closed", []Object{closed, available}, 1, "available"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := make([]Object, len(tc.slots))
			for i, slot := range tc.slots {
				rows[i] = Object{"id": i + 1}
				for key, value := range slot {
					rows[i][key] = value
				}
			}
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			c := NewClient("", time.Second, true, false, false)
			c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case "/item/activity/pln3000044589/":
					return fakeResponse(200, `<script>var ASOVIEW_DATASOURCE = {"plan":{"planCode":"pln3000044589","title":"Synthetic activity"}};</script>`), nil
				case "/stocks/calendars":
					return fakeResponse(200, `{"years":[{"months":[{"weeks":[{"days":[{"date":"2026-10-02","isRemaining":true}]}]}]}]}`), nil
				case "/stocks/courses":
					return fakeResponse(200, string(body)), nil
				default:
					return nil, fmt.Errorf("unexpected source request: %s", r.URL.Path)
				}
			})
			out, err := c.Availability(context.Background(), "pln3000044589", "2026-10-02", "", tc.quantity)
			if err != nil || out["status"] != tc.want {
				t.Fatalf("headline status: got %v, want %s; error %v", out, tc.want, err)
			}
		})
	}
}

func TestDiscoveryRecommendationsPreservePageContinuation(t *testing.T) {
	calls := 0
	c := NewClient("", time.Second, true, false, false)
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		page := r.URL.Query().Get("page")
		id, name, next := "ticket0000049223", "First match", `<a href="/search/?page=2">2</a>`
		if page == "2" {
			id, name, next = "ticket0000034693", "Second match", ""
		}
		body := fmt.Sprintf(`<script>var ASOVIEW_DATASOURCE = {"query":{"page":"%s","adultQuantity":"1","childQuantity":"0"},"displayFilterTotalCount":"3","caughtBasePlanPriceList":[{"goodsId":"%s","sellingPrice":700}],"raiseBasePlanPriceList":[{"goodsId":"ticket0000012233","sellingPrice":900}]};</script><li class="search-result-list__item"><a class="search-result-list__image-link" href="/base/1/"></a><a class="search-result-list__plan-link" href="/item/ticket/%s/"><b class="search-result-list__plan-name">%s</b></a><a class="search-result-list__plan-link" href="/item/ticket/ticket0000012233/"><b class="search-result-list__plan-name">Recommendation</b></a></li>%s`, page, id, id, name, next)
		return fakeResponse(200, body), nil
	})
	out, err := c.Discover(context.Background(), SearchParams{Page: 1, Pages: 1, Limit: 10, Adults: 1})
	if err != nil || object(out["coverage"])["partial"] != true || object(out["coverage"])["next_cursor"] != "2:0" {
		t.Fatal("recommendations hid continuation", out, err)
	}
	out, err = c.Discover(context.Background(), SearchParams{Page: 1, Pages: 2, Limit: 10, Adults: 1})
	if err != nil {
		t.Fatal(err)
	}
	rows := out["results"].([]Object)
	if calls != 3 || len(rows) != 2 || rows[1]["id"] != "ticket0000034693" || object(out["coverage"])["partial"] != false {
		t.Fatal("later matches were skipped or recommendations leaked", calls, out)
	}
}

func TestDiscoveryVenueTotalDoesNotCountProducts(t *testing.T) {
	raw := []byte(`<script>var ASOVIEW_DATASOURCE = {"displayFilterTotalCount":"2","caughtBasePlanPriceList":[{"goodsId":"ticket0000049223"},{"goodsId":"ticket0000034693"}]};</script><li class="search-result-list__item"><a class="search-result-list__image-link" href="/base/1/"></a><a class="search-result-list__plan-link" href="/item/ticket/ticket0000049223/"><b class="search-result-list__plan-name">First product</b></a><a class="search-result-list__plan-link" href="/item/ticket/ticket0000034693/"><b class="search-result-list__plan-name">Second product, same venue</b></a></li><a href="/search/?page=2">2</a>`)
	ds, doc, err := datasource(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows, next := parseCards(doc, ds, 1)
	if len(rows) != 2 || !next {
		t.Fatal("product count falsely exhausted venue pagination", rows, next)
	}
}

func TestOptionalCacheFailurePreservesLiveResult(t *testing.T) {
	blockedDir := filepath.Join(t.TempDir(), "cache-is-a-file")
	if err := os.WriteFile(blockedDir, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	c := NewClient(blockedDir, time.Second, false, false, false)
	calls := 0
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return fakeResponse(200, `{"source":"live"}`), nil
	})
	body, err := c.Get(context.Background(), "/search/", nil, time.Hour)
	if err != nil || string(body) != `{"source":"live"}` || c.Metrics().CacheWriteFailures != 1 {
		t.Fatal("optional persistence discarded the live result", string(body), err, c.Metrics())
	}
	if len(c.Sources) != 1 || c.Sources[0].Cached || c.Sources[0].FetchedAt == "" {
		t.Fatal("live provenance was lost", c.Sources)
	}
	c.Offline = true
	if _, err := c.Get(context.Background(), "/search/", nil, time.Hour); err == nil || !strings.Contains(err.Error(), "no fresh cached response") {
		t.Fatal("failed persistence was claimed as a usable local cache", err)
	}
	if calls != 1 {
		t.Fatal("local mode attempted network", calls)
	}
}
