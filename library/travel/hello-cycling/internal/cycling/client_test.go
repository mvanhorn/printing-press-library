package cycling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cliutil"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPriceAndRulesReadThrough(t *testing.T) {
	index, _ := os.ReadFile("testdata/price-index.html")
	tokyo, _ := os.ReadFile("testdata/price-tokyo.html")
	types, _ := os.ReadFile("testdata/vehicle_types.json")
	for _, tc := range []struct {
		name, area   string
		rules        bool
		wantRequests int
		ok           bool
	}{{"Tokyo", "tokyo", false, 2, true}, {"not advertised", "unknown", false, 1, false}, {"vehicle rules", "", true, 2, true}} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			c := NewClient()
			c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests++
				var b []byte
				switch r.URL.Path {
				case "/price/":
					b = index
				case "/price/tokyo/":
					b = tokyo
				case "/api/v4/gbfs/hellocycling/vehicle_types.json":
					b = types
				case "/getting-started/types-and-stations/":
					b = []byte("交通ルールテストへの合格 16歳以上 年齢確認書類の提出 HELLO CYCLINGアプリからしか")
				default:
					t.Fatalf("unexpected source URL %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(b))), Header: make(http.Header)}, nil
			})
			var result map[string]any
			var e error
			if tc.rules {
				result, e = c.VehicleRules(context.Background())
			} else {
				result, e = c.Prices(context.Background(), tc.area)
			}
			if (e == nil) != tc.ok || requests != tc.wantRequests {
				t.Fatalf("err=%v requests=%d", e, requests)
			}
			if tc.ok && result["results"] == nil {
				t.Fatal("empty typed results")
			}
		})
	}
}

func TestAdvertisedFeedsAndPartialFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		statusCode int
		wantErr    bool
		state      string
		empty      bool
	}{{"live", 200, false, "available", false}, {"partial missing", 500, true, "source_missing", false}, {"empty status schema", 200, true, "source_missing", true}, {"throttled", 429, true, "", false}, {"access denied", 403, true, "", false}} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			now := time.Now().UTC()
			s := sample(now)
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("credentials sent")
				}
				var data any
				switch r.URL.Path {
				case "/gbfs.json":
					data = map[string]any{"ja": map[string]any{"feeds": []map[string]string{{"name": "station_information", "url": server.URL + "/station_information.json"}, {"name": "station_status", "url": server.URL + "/station_status.json"}, {"name": "vehicle_types", "url": server.URL + "/vehicle_types.json"}}}}
				case "/station_information.json":
					data = map[string]any{"stations": s.Information}
				case "/station_status.json":
					if tc.statusCode != 200 {
						w.WriteHeader(tc.statusCode)
						return
					}
					data = map[string]any{"stations": s.Statuses}
					if tc.empty {
						data = map[string]any{"stations": []Status{}}
					}
				case "/vehicle_types.json":
					data = map[string]any{"vehicle_types": s.Vehicles}
				default:
					t.Fatal("unexpected request")
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"version": "2.3", "last_updated": now.Unix(), "ttl": 60, "data": data})
			}))
			defer server.Close()
			c := NewClient()
			c.indexURL = server.URL + "/gbfs.json"
			out, e := c.Fetch(context.Background())
			if (e != nil) != tc.wantErr {
				t.Fatalf("err=%v", e)
			}
			if requests.Load() != 4 {
				t.Fatalf("requests=%d", requests.Load())
			}
			if tc.statusCode == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) {
					t.Fatalf("not typed rate limit: %T", e)
				}
			}
			if tc.statusCode == 500 || tc.empty {
				var partial *StatusFeedError
				if !errors.As(e, &partial) || len(out.Information) == 0 || len(out.Statuses) != 0 {
					t.Fatalf("partial observation did not retain discovery/error boundary: %#v %v", out, e)
				}
			}
			if tc.state != "" {
				r := out.Stations(now, 5*time.Minute, "2")[0]
				if r.RentalState != tc.state {
					t.Fatal(r.RentalState)
				}
			}
		})
	}
}
func TestHTTPAndFeedBounds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		ok      bool
	}{{"small", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "123") }, true}, {"too large", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "123456789") }, false}, {"redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://evil.example/", 302) }, false}, {"source missing", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }, false}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			_, e := NewClient().get(context.Background(), srv.URL, 8)
			if (e == nil) != tc.ok {
				t.Fatal(e)
			}
		})
	}
	c := NewClient()
	for _, u := range []string{"https://evil.example/station_status.json", "https://api-public.odpt.org/api/v4/gbfs/hellocycling/station_status.json?key=secret", "https://api-public.odpt.org/api/v4/gbfs/hellocycling/../station_status.json"} {
		if c.allowedFeed(u, "station_status") {
			t.Fatal("untrusted feed URL allowed")
		}
	}
	if !c.allowedFeed("https://api-public.odpt.org/api/v4/gbfs/hellocycling/station_status.json", "station_status") {
		t.Fatal("advertised URL rejected")
	}
}
