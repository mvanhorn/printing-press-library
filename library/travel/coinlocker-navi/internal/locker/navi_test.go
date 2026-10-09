// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/cliutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseListingSearch(t *testing.T) {
	rows, err := ParseListing(fixture(t, "search.html"), DefaultNaviBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 {
		t.Fatalf("rows = %d, want 10", len(rows))
	}
	first := rows[0]
	if first.ID != "3" || first.Name != "JR東京駅中央線" || first.Service != "coin_locker" {
		t.Fatalf("first = %+v", first)
	}
	if first.Lat == nil || *first.Lat != 35.681752 || first.DistanceM != nil {
		t.Fatalf("coords/distance wrong: %v %v", first.Lat, first.DistanceM)
	}
	if len(first.Sizes) != 1 || *first.Sizes[0].Label != "M" || *first.Sizes[0].PriceYen != 400 || *first.Sizes[0].Count != 39 {
		t.Fatalf("sizes = %+v", first.Sizes)
	}
	if first.Hours.Kind != "first_to_last_train" {
		t.Fatalf("hours = %+v", first.Hours)
	}
	for _, r := range rows {
		if !strings.HasPrefix(r.URL, DefaultNaviBase+"/cl/") {
			t.Errorf("bad url %s", r.URL)
		}
		if r.SizesKnown != (len(r.Sizes) > 0) {
			t.Errorf("%s: sizes_known mismatch", r.ID)
		}
	}
}

func TestParseListingAreaHasEcboAndDistance(t *testing.T) {
	rows, err := ParseListing(fixture(t, "area_tokyo.html"), DefaultNaviBase)
	if err != nil {
		t.Fatal(err)
	}
	var ecbo, withDist int
	for _, r := range rows {
		if r.Service == "ecbo_cloak" {
			ecbo++
			if r.WalkUp == nil || *r.WalkUp {
				t.Errorf("ecbo row must not be walk-up")
			}
		}
		if r.DistanceM != nil {
			withDist++
		}
	}
	if ecbo != 1 || withDist != len(rows) || len(rows) < 5 {
		t.Fatalf("rows=%d ecbo=%d withDist=%d", len(rows), ecbo, withDist)
	}
	if *rows[0].DistanceM != 8 {
		t.Fatalf("first distance = %d, want 8", *rows[0].DistanceM)
	}
}

func TestParseListingGPS(t *testing.T) {
	rows, err := ParseListing(fixture(t, "gps_nearest.html"), DefaultNaviBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || rows[0].DistanceM == nil {
		t.Fatalf("gps rows = %+v", rows)
	}
	for i := 1; i < len(rows); i++ {
		if *rows[i].DistanceM < *rows[i-1].DistanceM {
			t.Fatalf("not sorted by distance")
		}
	}
}

func TestParseDetail(t *testing.T) {
	d, err := ParseDetail(fixture(t, "detail_3366.html"), "3366", DefaultNaviBase)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "一般国道20号高架下" || d.Service != "coin_locker" {
		t.Fatalf("name/service = %q %q", d.Name, d.Service)
	}
	if len(d.Sizes) != 4 || *d.Sizes[3].Label != "XL" || *d.Sizes[3].PriceYen != 1000 {
		t.Fatalf("sizes = %+v", d.Sizes)
	}
	if d.Hours.Kind != "clock_range" || !d.Hours.Overnight {
		t.Fatalf("hours = %+v", d.Hours)
	}
	if d.ICCard == nil || !*d.ICCard || len(d.Payment) != 6 {
		t.Fatalf("payment = %v ic=%v", d.Payment, d.ICCard)
	}
	if d.ChangeMachine == nil || *d.ChangeMachine {
		t.Fatalf("change machine = %v", d.ChangeMachine)
	}
	if d.Lat == nil || d.Lon == nil || d.Note == nil || !strings.Contains(*d.Note, "特中") {
		t.Fatalf("coords/note missing: %v %v %v", d.Lat, d.Lon, d.Note)
	}
	if len(d.Neighbours) != 5 {
		t.Fatalf("neighbours = %d", len(d.Neighbours))
	}

	u, err := ParseDetail(fixture(t, "detail_2685.html"), "2685", DefaultNaviBase)
	if err != nil {
		t.Fatal(err)
	}
	if u.SizesKnown || u.Hours.Kind != "unknown" || u.ChangeMachine != nil || u.Hours.Raw != nil {
		t.Fatalf("unknown fields must stay null: %+v", u.Locker)
	}
}

func TestParseDetailShapeChange(t *testing.T) {
	if _, err := ParseDetail([]byte("<html><h1>x</h1></html>"), "1", DefaultNaviBase); err == nil {
		t.Fatal("want error when the detail table is missing")
	}
}

func testClient(srv *httptest.Server) *Client {
	c := New(5 * time.Second)
	c.NaviBase, c.EkicubeBase = srv.URL, srv.URL
	c.rate = 200
	return c
}

func wrapHTML(ids ...int) string {
	var b strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&b, `<div class="nearest-wrap"><div class="nearest-head"><p class="title"><a href="/cl/%d">L%d</a></p></div></div>`, id, id)
	}
	return b.String()
}

func TestSearchFollowsMorePages(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			fmt.Fprint(w, `<script>xhr.setRequestHeader('X-CSRF-Token', "abc123");</script>`+wrapHTML(1, 2, 3, 4, 5, 6, 7, 8, 9, 10))
		case "/search/search_more":
			posts++
			if r.Header.Get("X-CSRF-Token") != "abc123" || r.FormValue("cnt") != "10" {
				w.WriteHeader(400)
				return
			}
			fmt.Fprint(w, wrapHTML(11, 12))
		}
	}))
	defer srv.Close()
	rows, more, err := testClient(srv).Search(context.Background(), "東京駅", 30, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 12 || more || posts != 1 {
		t.Fatalf("rows=%d more=%v posts=%d", len(rows), more, posts)
	}
}

func TestNearestUsesCSRF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/gps/":
			http.SetCookie(w, &http.Cookie{Name: "csrfToken", Value: "c"})
			fmt.Fprint(w, `X-CSRF-Token', "beef09"`)
		case "/search/gps/nearest_cl":
			if ck, err := r.Cookie("csrfToken"); err != nil || ck.Value != "c" || r.Header.Get("X-CSRF-Token") != "beef09" || r.FormValue("location_lat") == "" {
				w.WriteHeader(403)
				return
			}
			fmt.Fprint(w, wrapHTML(7))
		}
	}))
	defer srv.Close()
	rows, err := testClient(srv).Nearest(context.Background(), 35.68, 139.76)
	if err != nil || len(rows) != 1 || rows[0].ID != "7" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestClientErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cl/404":
			w.WriteHeader(404)
		case "/cl/429":
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	c := testClient(srv)
	if _, err := c.Detail(context.Background(), "404"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("404: %v", err)
	}
	var rl *cliutil.RateLimitError
	if _, err := c.Detail(context.Background(), "429"); !errors.As(err, &rl) {
		t.Fatalf("429: %v", err)
	}
	var he *HTTPError
	if _, err := c.Detail(context.Background(), "500"); !errors.As(err, &he) || he.Status != 500 {
		t.Fatalf("500: %v", err)
	}
	if c.Requests() != 3 || len(c.SourceURLs()) != 3 {
		t.Fatalf("requests=%d urls=%v", c.Requests(), c.SourceURLs())
	}
}

func TestRedirectsStaySameHost(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "other host")
	}))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cl/1":
			http.Redirect(w, r, "/cl/2", http.StatusFound)
		case "/cl/2":
			http.Redirect(w, r, other.URL+"/x", http.StatusFound)
		}
	}))
	defer srv.Close()
	_, err := testClient(srv).fetch(context.Background(), srv.URL+"/cl/1", nil)
	if err == nil || !strings.Contains(err.Error(), "another host") {
		t.Fatalf("want cross-host redirect refused, got %v", err)
	}
}

func TestSelfSignedTLSRejected(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()
	// New() uses the default TLS config; the test server's self-signed
	// certificate must fail verification.
	_, err := testClient(srv).fetch(context.Background(), srv.URL+"/", nil)
	if err == nil {
		t.Fatal("self-signed certificate must be rejected")
	}
}

func TestOversizeBodyRejected(t *testing.T) {
	big := strings.Repeat("x", maxBody+10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, big)
	}))
	defer srv.Close()
	if _, err := testClient(srv).fetch(context.Background(), srv.URL+"/", nil); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("want oversize error, got %v", err)
	}
}
