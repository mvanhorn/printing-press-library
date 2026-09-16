package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/config"
)

func TestImmowebQueryKeepsCommas(t *testing.T) {
	v := url.Values{}
	v.Set("epcScores", "F,G")
	v.Set("postalCodes", "BE-1030,BE-1050")
	got := pathWithQueryValues("/en/search-results", v)
	if got != "/en/search-results?epcScores=F,G&postalCodes=BE-1030,BE-1050" {
		t.Fatalf("commas must stay literal for Immoweb: %s", got)
	}
}

// The map-parameter path used by find/search/MCP must emit the same literal
// commas on the wire: Immoweb ignores epcScores=F%2CG.
func TestGetSendsListFiltersWithLiteralCommas(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "epcScores=F,G&postalCodes=BE-1030,BE-1050" {
			t.Fatalf("raw query = %q, want literal commas", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"classifiedsCount":521}`))
	}))
	t.Cleanup(server.Close)

	c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true
	if _, err := c.Get(context.Background(), "/en/search-results-count", map[string]string{"epcScores": "F,G", "postalCodes": "BE-1030,BE-1050"}); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
}
