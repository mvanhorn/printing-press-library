// Copyright 2026 Brian Wishan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/config"
)

func testClientFor(url string) *client.Client {
	cfg := &config.Config{
		BaseURL:     url,
		AccessToken: "session-id=test-cookie",
		AuthSource:  "browser",
	}
	c := client.New(cfg, 5_000_000_000, 0)
	c.BaseURL = strings.TrimRight(url, "/")
	c.NoCache = true
	return c
}

const signInInterstitialHTML = `<html><head><title>Amazon Sign-In</title></head>
<body><form action="/ax/claim?arb=193cca18-8b42-4d51-9a67-6e97f1363c27">
<input name="email"><input name="password" id="ap_password"></form></body></html>`

const realOrderPageHTML = `<html><head><title>Amazon.in - Your Orders</title></head><body>
<div class="order-card js-order-card">ORDER PLACED May 5, 2026 TOTAL ₹1,234.56 SHIP TO Jane ORDER # 408-1234567-1234567</div>
</body></html>`

// A sign-in/claim interstitial served with HTTP 200 must surface as an auth
// error from the order-list walk (the path spend and the novel commands use),
// not roll up an empty result.
func TestFetchOrderListPages_FailsOnInterstitial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(signInInterstitialHTML))
	}))
	defer srv.Close()

	c := testClientFor(srv.URL)
	orders, err := fetchOrderListPages(context.Background(), c, "year-2026", 3)
	if err == nil {
		t.Fatalf("expected auth/interstitial error, got nil (orders=%d)", len(orders))
	}
	if !strings.Contains(err.Error(), "sign-in/interstitial") {
		t.Errorf("error = %v, want it to mention sign-in/interstitial", err)
	}
}

// A genuine order page must still parse to orders — the interstitial guard must
// not break the authenticated path.
func TestFetchOrderListPages_ParsesRealOrderPage(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// First page returns one order; subsequent pages are empty so the walk
		// terminates without a "HasNext" link.
		if calls == 1 {
			_, _ = w.Write([]byte(realOrderPageHTML))
			return
		}
		_, _ = w.Write([]byte(`<html><head><title>Your Orders</title></head><body></body></html>`))
	}))
	defer srv.Close()

	c := testClientFor(srv.URL)
	orders, err := fetchOrderListPages(context.Background(), c, "year-2026", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
	if orders[0].OrderID != "408-1234567-1234567" {
		t.Errorf("orderID = %q, want 408-1234567-1234567", orders[0].OrderID)
	}
	if orders[0].Currency != "INR" {
		t.Errorf("currency = %q, want INR", orders[0].Currency)
	}
}

func TestFetchOrderListPages_FailsOnLaterInterstitial(t *testing.T) {
	calls := 0
	firstPage := `<html><head><title>Your Orders</title></head><body>
<div class="order-card js-order-card">ORDER PLACED May 5, 2026 TOTAL $51.46 SHIP TO Jane ORDER # 111-1111111-1111111</div>
<li class="a-last"><a href="#">Next</a></li>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(firstPage))
			return
		}
		_, _ = w.Write([]byte(signInInterstitialHTML))
	}))
	defer srv.Close()

	c := testClientFor(srv.URL)
	orders, err := fetchOrderListPages(context.Background(), c, "year-2026", 3)
	if err == nil {
		t.Fatalf("expected auth/interstitial error, got nil (orders=%d)", len(orders))
	}
	if !strings.Contains(err.Error(), "sign-in/interstitial") {
		t.Errorf("error = %v, want it to mention sign-in/interstitial", err)
	}
}
