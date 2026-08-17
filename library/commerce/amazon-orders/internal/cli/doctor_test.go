package cli

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/config"
)

func testDoctorClient(t *testing.T, html string, status int) *client.Client {
	t.Helper()
	c := client.New(&config.Config{BaseURL: "https://example.invalid"}, 0, 0)
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != browserSessionValidationPath {
			t.Fatalf("probe path = %q, want %q", req.URL.Path, browserSessionValidationPath)
		}
		if got := req.URL.Query().Get("timeFilter"); got != "months-3" {
			t.Fatalf("timeFilter = %q, want months-3", got)
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(html)),
			Header:     make(http.Header),
		}, nil
	})}
	return c
}

func TestDoctorLiveCredentialStatusRejectsInterstitial(t *testing.T) {
	ok, detail := doctorLiveCredentialStatus(testDoctorClient(t, signInInterstitialHTML, http.StatusOK))
	if ok {
		t.Fatalf("doctorLiveCredentialStatus ok=true, want false (%s)", detail)
	}
	if !strings.Contains(detail, "sign-in/interstitial") {
		t.Fatalf("detail = %q, want sign-in/interstitial", detail)
	}
}

func TestDoctorLiveCredentialStatusAcceptsAuthenticatedOrdersPage(t *testing.T) {
	ok, detail := doctorLiveCredentialStatus(testDoctorClient(t, realOrderPageHTML, http.StatusOK))
	if !ok {
		t.Fatalf("doctorLiveCredentialStatus ok=false, want true (%s)", detail)
	}
	if !strings.Contains(detail, "/your-orders/orders") {
		t.Fatalf("detail = %q, want validation path", detail)
	}
}

func TestDoctorLiveCredentialStatusRejectsGenericHTML(t *testing.T) {
	ok, detail := doctorLiveCredentialStatus(testDoctorClient(t, `<html><head><title>Amazon.com</title></head><body>hello</body></html>`, http.StatusOK))
	if ok {
		t.Fatalf("doctorLiveCredentialStatus ok=true, want false (%s)", detail)
	}
	if !strings.Contains(detail, "order-history") {
		t.Fatalf("detail = %q, want order-history failure", detail)
	}
}

func TestCollectCacheReportFlagsAuthInterstitialRows(t *testing.T) {
	db := testStore(t)
	if err := db.Upsert("orders", "orders", []byte(signInInterstitialHTML)); err != nil {
		t.Fatalf("upsert interstitial: %v", err)
	}
	if err := db.SaveSyncState("orders", "", 1); err != nil {
		t.Fatalf("SaveSyncState: %v", err)
	}

	report := collectCacheReport(context.Background(), "")
	if got := report["status"]; got != "error" {
		t.Fatalf("cache status = %v, want error", got)
	}
	if hint := report["hint"].(string); !strings.Contains(hint, "sign-in/interstitial") {
		t.Fatalf("hint = %q, want sign-in/interstitial", hint)
	}
}
