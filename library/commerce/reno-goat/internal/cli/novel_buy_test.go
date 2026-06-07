package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeBuySKUCanonicalizesRetailSuffixes(t *testing.T) {
	tests := map[string]string{
		"6x Lutron DVRFW-6L-WH": "DVRF-6L-WH",
		"DVRF-6L-WH-R":          "DVRF-6L-WH",
		"lutron dvrfw-6l-wh-a":  "DVRF-6L-WH",
		"Caseta DVRF-6LS-WH-3":  "DVRF-6LS-WH-3",
		"not a dimmer":          "",
	}

	for input, want := range tests {
		if got := normalizeBuySKU(input); got != want {
			t.Fatalf("normalizeBuySKU(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExtractRetailPricePrefersExactProductWindow(t *testing.T) {
	body := `
		<html>
			<title>DVRF-6L-WH product</title>
			<body>
				<div>Unrelated accessory $23.95</div>
				<section>
					<h1>Lutron DVRF-6L-WH Diva Smart Dimmer</h1>
					<span>SKU: DVRF-6L-WH</span>
					<strong>$69.95</strong>
				</section>
			</body>
		</html>`

	got := extractRetailPrice(body, []string{"DVRF-6L-WH"})
	if got != 69.95 {
		t.Fatalf("extractRetailPrice = %.2f, want 69.95", got)
	}
}

func TestExtractRetailPricePrefersProductMetaPrice(t *testing.T) {
	body := `
		<html>
			<head>
				<meta property="product:price:amount" content="69.95">
			</head>
			<body>
				<h1>Lutron DVRF-6L-WH Diva Smart Dimmer</h1>
				<div class="product_price">
					<h5>$92.40</h5>
					<h4>$69.95</h4>
				</div>
				<div>Frequently bought together: Pico remote $24.95</div>
			</body>
		</html>`

	got := extractRetailPrice(body, []string{"DVRF-6L-WH"})
	if got != 69.95 {
		t.Fatalf("extractRetailPrice = %.2f, want 69.95", got)
	}
}

func TestFetchBuyOfferDoesNotUseSearchPageFallback(t *testing.T) {
	searchHit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "search") {
			searchHit = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`Search results for DVRF-6L-WH <span>$49.99</span>`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<h1>Different Product</h1><span>$23.95</span>`))
	}))
	defer server.Close()

	retailer := buyRetailer{
		Name: "test",
		ProductURL: func(string) string {
			return server.URL + "/product"
		},
		SearchURL: func(string) string {
			return server.URL + "/search?q=DVRF-6L-WH"
		},
	}

	_, err := fetchBuyOffer(context.Background(), server.Client(), retailer, "DVRF-6L-WH", 6, time.Now().UTC().Format(time.RFC3339))
	if err == nil {
		t.Fatal("fetchBuyOffer succeeded on non-matching product page")
	}
	if searchHit {
		t.Fatal("fetchBuyOffer used search fallback; exact-SKU buy should not accept generic search pages")
	}
}

func TestProductMatchesBuySKUUsesNormalizedSourceFields(t *testing.T) {
	product := NormalizedProduct{
		Source: "prolighting",
		ID:     "DVRF-6L-WH",
		Title:  "Lutron Diva Smart Dimmer",
		URL:    "https://www.prolighting.com/dvrf-6l-wh.html",
	}
	if !productMatchesBuySKU(product, "DVRF-6L-WH") {
		t.Fatal("productMatchesBuySKU did not match exact normalized product ID")
	}

	adjacent := NormalizedProduct{
		Source: "prolighting",
		ID:     "PJ2-3BRL-GWH-L01",
		Title:  "Lutron Pico Remote",
		URL:    "https://www.prolighting.com/pj2-3brl-gwh-l01.html",
	}
	if productMatchesBuySKU(adjacent, "DVRF-6L-WH") {
		t.Fatal("productMatchesBuySKU matched an adjacent Lutron product")
	}
}

func TestMergeBuyOffersKeepsBestEvidencePerSource(t *testing.T) {
	offers := mergeBuyOffers(nil, []buyOffer{{
		Source:    "prolighting",
		SKU:       "DVRF-6L-WH",
		Price:     69.95,
		MatchedBy: "active_source_exact_result",
	}})
	offers = mergeBuyOffers(offers, []buyOffer{{
		Source:    "prolighting",
		SKU:       "DVRF-6L-WH",
		Price:     69.95,
		MatchedBy: "exact_sku_page",
	}})
	if len(offers) != 1 {
		t.Fatalf("mergeBuyOffers returned %d offers, want 1", len(offers))
	}
	if offers[0].MatchedBy != "exact_sku_page" {
		t.Fatalf("mergeBuyOffers kept %q, want exact_sku_page", offers[0].MatchedBy)
	}

	failures := filterBuyFailuresWithOffers([]buyFailure{
		{Source: "prolighting", Reason: "no exact SKU matches"},
		{Source: "lowes", Reason: "HTTP 403"},
	}, offers)
	if len(failures) != 1 || failures[0].Source != "lowes" {
		t.Fatalf("filterBuyFailuresWithOffers = %#v, want only lowes failure", failures)
	}
}
