package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil/testenv"
	"net/http"
	"testing"
)

func TestRawPublicAbsoluteRoutesPreserveQueryOnActualHTTPRequest(t *testing.T) {
	testenv.Isolate(t)
	original := clientHooks
	t.Cleanup(func() { clientHooks = original })
	cases := []struct {
		name       string
		args       []string
		host, path string
		query      map[string]string
	}{
		{"keyword", []string{"keyword", "--charset", "utf-8", "--query", "品川", "--page-size", "3", "--page", "1", "--no-cache", "--json"}, "kw.travel.rakuten.co.jp", "/keyword/Search.do", map[string]string{"charset": "utf-8", "f_query": "品川", "f_max": "3", "f_next": "1"}},
		{"dated plan", []string{"hotelinfo", "51870", "--f-flg", "PLAN", "--rooms", "2", "--adults-per-room", "2", "--infant-none", "1", "--checkin-year", "2026", "--checkin-month", "11", "--checkin-day", "8", "--checkout-year", "2026", "--checkout-month", "11", "--checkout-day", "10", "--page", "1", "--no-cache", "--json"}, "hotel.travel.rakuten.co.jp", "/hotelinfo/plan/51870", map[string]string{"f_flg": "PLAN", "f_heya_su": "2", "f_otona_su": "2", "f_y4": "1", "f_nen1": "2026", "f_tuki1": "11", "f_hi1": "8", "f_nen2": "2026", "f_tuki2": "11", "f_hi2": "10", "f_page_no": "1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			clientHooks = append(append([]func(*client.Client) error{}, original...), func(c *client.Client) error {
				c.HTTPClient.Transport = travelCLIRoundTripper(func(request *http.Request) (*http.Response, error) {
					calls++
					if request.URL.Hostname() != tc.host || request.URL.Path != tc.path {
						t.Errorf("wrong raw route: %s", request.URL)
					}
					for key, want := range tc.query {
						if got := request.URL.Query().Get(key); got != want {
							t.Errorf("wire query %s=%q, want %q; URL=%s", key, got, want, request.URL)
						}
					}
					if got := request.Header.Get("Accept"); got != "text/html,application/xhtml+xml" {
						t.Errorf("public source wire Accept=%q; must negotiate the HTML document", got)
					}
					t.Logf("actual wire URL=%s Accept=%q User-Agent=%q", request.URL, request.Header.Get("Accept"), request.Header.Get("User-Agent"))
					return travelHTTPResponse(request, 200, "<html><title>Rakuten fixture</title><body>public raw query fixture</body></html>"), nil
				})
				return nil
			})
			out, diagnostics, err := runPublicTravelCLI(tc.args...)
			if err != nil {
				t.Fatalf("raw source fixture: err=%v stdout=%s stderr=%s", err, out, diagnostics)
			}
			if calls != 1 {
				t.Fatalf("expected one real client transport request; calls=%d", calls)
			}
		})
	}
}
