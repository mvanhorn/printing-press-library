package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDirectSearchQueryUsesTitleBrandCityWithoutQuotedPhrases(t *testing.T) {
	query := directSearchQuery("Tahoe Keys Breeze-Boat Dock, Hot Tub, BBQ", "RnR Vacation Rentals", "South Lake Tahoe")

	for _, want := range []string{"Tahoe Keys Breeze-Boat Dock", "RnR Vacation Rentals", "South Lake Tahoe", "vacation rental", "direct booking"} {
		if !strings.Contains(query, want) {
			t.Fatalf("query %q missing %q", query, want)
		}
	}
	if strings.Contains(query, `"`) {
		t.Fatalf("query should not contain exact-phrase quotes: %q", query)
	}
	if strings.Contains(query, "Hot") {
		t.Fatalf("query should truncate title to first five words: %q", query)
	}
}

func TestScanDirectPriceReportsBlockedNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked", http.StatusForbidden)
	}))
	defer srv.Close()

	total, note := scanDirectPrice(context.Background(), srv.URL, "", "")
	if total != 0 || note != "found_site_blocked" {
		t.Fatalf("scanDirectPrice = (%v, %q), want (0, found_site_blocked)", total, note)
	}
}
