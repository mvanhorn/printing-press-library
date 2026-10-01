package ecbo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTimesAndCounts(t *testing.T) {
	for _, p := range [][2]string{{"2026-02-30T12:00", "2026-03-01T12:00"}, {"2026-10-03T12:00", "2026-10-03T12:00"}, {"2026-10-03T12:00Z", "2026-10-03T13:00"}, {"2026-10-03T12:00", "2026-11-03T12:00"}} {
		if _, _, e := ParseTimes(p[0], p[1]); e == nil {
			t.Fatal(p)
		}
	}
	a, b, e := ParseTimes("2026-10-03T23:00", "2026-10-04T01:00")
	if e != nil || b.Sub(a) != 2*time.Hour || a.Format("-07:00") != "+09:00" {
		t.Fatal(a, b, e)
	}
	for _, p := range [][2]int{{-1, 2}, {0, 0}, {51, 0}} {
		if ValidateCounts(p[0], p[1]) == nil {
			t.Fatal(p)
		}
	}
}
func TestIdentityAndProjection(t *testing.T) {
	for _, s := range []string{"../../foo", "https://evil.example/spaces/0c3fb1e9-ad5a-42de-bfda-3027ebe4921e", "https://cloak.ecbo.io/bookings/GBy4uBrI"} {
		if _, e := ID(s); e == nil {
			t.Fatal(s)
		}
	}
	s, e := ID("https://cloak.ecbo.io/en/spaces/0c3fb1e9-ad5a-42de-bfda-3027ebe4921e?foo=bar")
	if e != nil || s == "" {
		t.Fatal(s, e)
	}
	p, e := Project(map[string]any{"quote": map[string]any{"price": 1300.0}, "cutoff": nil}, "quote.price,cutoff")
	if e != nil || p["cutoff"] != nil {
		t.Fatal(p, e)
	}
	if _, e = Project(p, "quote.nope"); e == nil {
		t.Fatal("unknown field accepted")
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func testClient(t *testing.T, f transport) *Client {
	c := New(t.TempDir(), "ja", time.Second, false, false)
	c.limiter = nil
	c.HTTP = &http.Client{Transport: f}
	return c
}
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}
func TestSourceCacheAndRateLimit(t *testing.T) {
	n := 0
	c := testClient(t, func(r *http.Request) (*http.Response, error) { n++; return response(200, `{"space_id":"abc"}`), nil })
	u := "https://api.ecbo.io/api/web/spaces/abc"
	for i := 0; i < 2; i++ {
		if _, e := c.Request(context.Background(), "GET", u, nil, time.Hour); e != nil {
			t.Fatal(e)
		}
	}
	if n != 1 || c.Meta.CacheHits != 1 {
		t.Fatal(n, c.Meta)
	}
	c.Refresh = true
	if _, e := c.Request(context.Background(), "GET", u, nil, time.Hour); e != nil || n != 2 {
		t.Fatal(n, e)
	}
	c = testClient(t, func(r *http.Request) (*http.Response, error) { return response(429, `{"errors":["slow down"]}`), nil })
	_, e := c.Request(context.Background(), "GET", u, nil, 0)
	if e == nil || !strings.Contains(e.Error(), "rate limited") {
		t.Fatal(e)
	}
}
func TestOfferSeparatesPriceFromValidation(t *testing.T) {
	for _, valid := range []bool{true, false} {
		c := testClient(t, func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/web/spaces/0c3fb1e9-ad5a-42de-bfda-3027ebe4921e":
				return response(200, `{"space_id":"0c3fb1e9-ad5a-42de-bfda-3027ebe4921e","name":"test","prices":[{"size":"small","price":500}],"capacity":[{"size":"small","quantity":30}],"can_reserve_multiple_days":true}`), nil
			case "/api/web/reservations/price":
				var p map[string]any
				json.NewDecoder(r.Body).Decode(&p)
				if p["from"] != "2026-10-03 23:00" {
					t.Fatal(p)
				}
				return response(200, `{"price":1000,"currency":"JPY"}`), nil
			case "/api/web/reservations/validate":
				b, _ := json.Marshal(map[string]bool{"valid": valid})
				return response(200, string(b)), nil
			}
			return nil, errors.New("unexpected endpoint")
		})
		o, e := c.Offer(context.Background(), "0c3fb1e9-ad5a-42de-bfda-3027ebe4921e", "2026-10-03T23:00", "2026-10-04T01:00", 1, 0)
		if e != nil {
			t.Fatal(e)
		}
		if o["confirmed_available_capacity"] != nil {
			t.Fatal("inferred inventory")
		}
		if !valid && o["availability"] != "source_rejected" {
			t.Fatal(o)
		}
	}
}
func TestMalformedSourceCannotBecomeEmptyResults(t *testing.T) {
	for _, body := range []string{`{}`, `{"hits":{"hits":[]},"timed_out":true}`, `{"hits":{"hits":[{"_id":"GBy4uBrI","_source":{"name":"Tokyo"}}]}}`} {
		c := testClient(t, func(r *http.Request) (*http.Response, error) { return response(200, body), nil })
		if _, e := c.Near(context.Background(), NearOptions{Lat: 35.6812, Lon: 139.7671, Radius: 5, Limit: 10}); e == nil {
			t.Fatal(body)
		}
	}
}
func TestInventoryNoAutomaticRefresh(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) { t.Fatal("offline requested network"); return nil, nil })
	v, e := c.Inventory("", 10, 0)
	if e != nil || len(v["results"].([]any)) != 0 {
		t.Fatal(v, e)
	}
	if e = c.SaveInventory(map[string]any{"results": []any{map[string]any{"id": "GBy4uBrI", "name": "Tokyo", "name_ja": "東京"}}}); e != nil {
		t.Fatal(e)
	}
	v, e = c.Inventory("東京", 1, 0)
	if e != nil || len(v["results"].([]any)) != 1 {
		t.Fatal(v, e)
	}
}

func TestReviewFailureContracts(t *testing.T) {
	for _, body := range []string{`{}`, `{"errors":{}}`, `{"errors":{"unknown":["something"]}}`, `{"errors":"down"}`} {
		c := testClient(t, func(r *http.Request) (*http.Response, error) { return response(422, body), nil })
		_, e := c.Request(context.Background(), "POST", "https://api.ecbo.io/api/web/reservations/validate", map[string]any{}, 0)
		if e == nil {
			t.Fatal("arbitrary rejection accepted", body)
		}
	}
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		return response(422, `{"errors":{"reservation_items":["only 20 slots available"]}}`), nil
	})
	v, e := c.Request(context.Background(), "POST", "https://api.ecbo.io/api/web/reservations/validate", map[string]any{}, 0)
	if e != nil || v["valid"] != false {
		t.Fatal(v, e)
	}
	c = testClient(t, func(r *http.Request) (*http.Response, error) {
		if strings.HasPrefix(r.URL.Path, "/api/web/spaces/") {
			return response(200, `{"space_id":"0c3fb1e9-ad5a-42de-bfda-3027ebe4921e","name":"test"}`), nil
		}
		if strings.HasSuffix(r.URL.Path, "/price") {
			return response(200, `{"price":800,"currency":"JPY"}`), nil
		}
		return response(503, `{"errors":["down"]}`), nil
	})
	if _, e = c.Offer(context.Background(), "0c3fb1e9-ad5a-42de-bfda-3027ebe4921e", "2026-10-03T19:00", "2026-10-03T21:00", 0, 1); e == nil {
		t.Fatal("validation failure became success")
	}
}
func TestReviewInventoryShapeAndEmptyProjection(t *testing.T) {
	c := testClient(t, nil)
	for _, body := range []string{`{}`, `{"data":{"results":[1]},"refreshed_at":"2026-10-01T00:00:00Z"}`, `{"data":{"results":null},"refreshed_at":"2026-10-01T00:00:00Z"}`} {
		if e := os.WriteFile(filepath.Join(c.CacheDir, "inventory.json"), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := c.Inventory("", 10, 0); e == nil {
			t.Fatal("malformed snapshot accepted", body)
		}
	}
	if e := ValidateListProjection("invented_field"); e == nil {
		t.Fatal("unknown empty-list projection accepted")
	}
	if e := ValidateListProjection("id,name_ja,listed_hours.from"); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(filepath.Join(c.CacheDir, "inventory.json")); e != nil {
		t.Fatal(e)
	}
	if e := os.Mkdir(filepath.Join(c.CacheDir, "inventory.json"), 0700); e != nil {
		t.Fatal(e)
	}
	e := c.SaveInventory(map[string]any{"results": []any{}})
	var typed *Error
	if !errors.As(e, &typed) || typed.Code != 10 {
		t.Fatal("filesystem failure not cache exit", e)
	}
}
func TestReviewCacheOwnsOnlyItsNamespace(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) { return response(200, `{"space_id":"test"}`), nil })
	for i := 0; i < maxCacheFiles; i++ {
		if e := os.WriteFile(filepath.Join(c.CacheDir, fmt.Sprintf("user-data-%d.json", i)), []byte("{}"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := c.Request(context.Background(), "GET", "https://api.ecbo.io/api/web/spaces/test", nil, time.Hour); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < maxCacheFiles; i++ {
		if _, e := os.Stat(filepath.Join(c.CacheDir, fmt.Sprintf("user-data-%d.json", i))); e != nil {
			t.Fatal("unrelated cache-dir file lost", e)
		}
	}
}
func TestReviewSearchIdentity(t *testing.T) {
	for _, body := range []string{`{"hits":{"total":1,"hits":[{"_id":"GBy4uBrI","_source":{},"fields":{"location":[{"lat":35.68,"lon":139.76}]}}]}}`, `{"hits":{"hits":[{"_id":"GBy4uBrI","_source":{"name":"東京"},"fields":{"location":[{"lat":35.68,"lon":139.76}]}}]}}`} {
		c := testClient(t, func(r *http.Request) (*http.Response, error) { return response(200, body), nil })
		if _, e := c.Near(context.Background(), NearOptions{Lat: 35.6812, Lon: 139.7671, Radius: 5, Limit: 10}); e == nil {
			t.Fatal("missing search identity accepted")
		}
	}
}

func TestCanonicalWebsiteLanguage(t *testing.T) {
	if websiteLocale("zh-CN") != "ja" || websiteLocale("zh-TW") != "zh-TW" {
		t.Fatal("canonical locale fallback")
	}
}
