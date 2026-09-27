package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil/testenv"
)

func blockPublicTravelCache(t *testing.T) {
	t.Helper()
	cacheDir, err := cliutil.CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "public-travel"), []byte("fixture blocks cache directory"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPublicTravelCacheWriteFailureKeepsLiveJSON(t *testing.T) {
	in, _ := futureTravelDates()
	cases := []struct {
		name     string
		args     []string
		requests int
		verify   func(*testing.T, map[string]any)
	}{
		{"areas", []string{"areas", "list", "--parent", "tokyo"}, 1, func(t *testing.T, p map[string]any) {
			rows := travelRows(t, p["results"])
			if len(rows) != 1 || travelObject(t, rows[0])["id"] != "tokyo/E" {
				t.Fatalf("live area result lost: %#v", p)
			}
		}},
		{"hotel search", []string{"hotels", "search", "--query", "品川", "--limit", "1"}, 1, func(t *testing.T, p map[string]any) {
			rows := travelRows(t, p["results"])
			if len(rows) != 1 || travelObject(t, rows[0])["hotel_id"] != "51870" {
				t.Fatalf("live search result lost: %#v", p)
			}
		}},
		{"hotel details", []string{"hotels", "show", "--hotel", "51870"}, 2, func(t *testing.T, p map[string]any) {
			hotel := travelObject(t, p["results"])
			if hotel["hotel_id"] != "51870" || len(travelRows(t, hotel["hotel_amenities"])) == 0 {
				t.Fatalf("live property details lost: %#v", p)
			}
		}},
		{"offer search", append(offerCLIArgs("search"), "--inventory-cache-seconds", "60", "--limit", "1"), 1, func(t *testing.T, p map[string]any) {
			offer := travelObject(t, travelRows(t, p["results"])[0])
			if offer["plan_id"] != "3951989" || travelObject(t, offer["price"])["per_room_whole_stay_jpy"] != float64(20720) {
				t.Fatalf("live priced offer lost: %#v", p)
			}
		}},
		{"offer inspection", append(offerCLIArgs("show"), "--plan", "3951989", "--room", "s-double-", "--inventory-cache-seconds", "60"), 3, func(t *testing.T, p map[string]any) {
			inspection := travelObject(t, p["results"])
			if travelObject(t, inspection["offer"])["room_id"] != "s-double-" || travelObject(t, inspection["property"])["hotel_id"] != "51870" {
				t.Fatalf("live joined inspection lost: %#v", p)
			}
		}},
		{"comparison", []string{"compare", "--hotels", "51870", "--checkins", in, "--nights", "2", "--rooms", "1", "--adults-per-room", "2", "--inventory-cache-seconds", "60"}, 1, func(t *testing.T, p map[string]any) {
			comparison := travelObject(t, p["results"])
			cells := travelRows(t, comparison["cells"])
			if len(cells) != 1 || travelObject(t, cells[0])["status"] != "ok" || len(travelRows(t, comparison["fetch_failures"])) != 0 {
				t.Fatalf("cache failure became a fetch failure: %#v", p)
			}
		}},
	}
	for _, tc := range cases {
		for _, noCache := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/no-cache=%t", tc.name, noCache), func(t *testing.T) {
				testenv.Isolate(t)
				blockPublicTravelCache(t)
				requests := 0
				injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
					requests++
					body := hotelBasicCLIHTML
					switch {
					case strings.Contains(request.URL.Path, "/hotelinfo/plan/"):
						body = offerCLIHTML(request, []cliRoomFixture{{"3951989", "s-double-", 20720}}, false)
					case strings.HasSuffix(request.URL.Path, "_std.html"):
						body = hotelDetailsCLIHTML
					case request.URL.Hostname() == "kw.travel.rakuten.co.jp":
						body = hotelSearchCLIHTML
					case strings.HasSuffix(request.URL.Path, "/tokyo/map.html"):
						body = `<html><body><a href="/yado/tokyo/E.html">品川</a></body></html>`
					}
					return travelHTTPResponse(request, 200, body), nil
				})
				args := append(append([]string{}, tc.args...), "--agent")
				if noCache {
					args = append(args, "--no-cache")
				}
				out, diagnostics, err := runPublicTravelCLI(args...)
				if err != nil {
					t.Fatalf("cache persistence masked valid source result: %v; stdout=%s; stderr=%s", err, out, diagnostics)
				}
				payload := decodeTravelOutput(t, out)
				if len(payload) != 2 || travelObject(t, payload["meta"])["status"] != "ok" {
					t.Fatalf("live JSON envelope changed: %s", out)
				}
				tc.verify(t, payload)
				if requests != tc.requests {
					t.Fatalf("valid result did not retain complete request sequence: got=%d want=%d", requests, tc.requests)
				}
				if noCache {
					if diagnostics != "" {
						t.Fatalf("--no-cache should skip persistence and its warning: %s", diagnostics)
					}
				} else if !strings.Contains(diagnostics, "warning: cache write failed:") {
					t.Fatalf("cache write failure needs stderr diagnostic: %q", diagnostics)
				}
				if strings.Contains(out, "cache write failed") {
					t.Fatalf("cache diagnostic polluted JSON stdout: %s", out)
				}
			})
		}
	}
}

func TestPublicTravelCacheWarningDoesNotMaskUpstreamFailure(t *testing.T) {
	testenv.Isolate(t)
	blockPublicTravelCache(t)
	requests := 0
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		requests++
		return travelHTTPResponse(request, 503, "<html><title>Service unavailable</title></html>"), nil
	})
	out, diagnostics, err := runPublicTravelCLI("hotels", "search", "--query", "品川", "--agent")
	if err == nil || ExitCode(err) != 5 || !strings.Contains(diagnostics, "upstream_error") {
		t.Fatalf("upstream failure became success: err=%v stdout=%s stderr=%s", err, out, diagnostics)
	}
	if out != "" || strings.Contains(diagnostics, "cache write failed") || requests != 3 {
		t.Fatalf("upstream failure was misreported as cache failure: requests=%d stdout=%s stderr=%s", requests, out, diagnostics)
	}
}
