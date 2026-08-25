package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/parser"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/store"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fakeSyncClient struct {
	body string
	err  error
}

func (f fakeSyncClient) Get(string, map[string]string) (json.RawMessage, error) {
	return json.RawMessage(f.body), f.err
}

func (f fakeSyncClient) RateLimit() float64 {
	return 0
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, _ := testStoreAtHome(t)
	return db
}

func testStoreAtHome(t *testing.T) (*store.Store, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	db, err := store.OpenWithContext(context.Background(), defaultDBPath("amazon-orders-pp-cli"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, home
}

func TestResolveReadLocalGetPrefersOrderIDParam(t *testing.T) {
	db := testStore(t)
	const orderID = "702-5010515-8774615"
	html := `<html><body>ORDER # 702-5010515-8774615</body></html>`
	if err := db.Upsert("orders", orderID, []byte(html)); err != nil {
		t.Fatalf("upsert order: %v", err)
	}

	flags := &rootFlags{dataSource: "local"}
	data, _, err := resolveRead(context.Background(), nil, flags, "orders", false, "/your-orders/order-details", map[string]string{"orderID": orderID}, nil, orderID)
	if err != nil {
		t.Fatalf("resolveRead local: %v", err)
	}
	if !strings.Contains(string(data), orderID) {
		t.Fatalf("local data = %q, want order %s", string(data), orderID)
	}
}

func TestResolveReadLocalRejectsAuthInterstitialRow(t *testing.T) {
	db := testStore(t)
	if err := db.Upsert("orders", "orders", []byte(signInInterstitialHTML)); err != nil {
		t.Fatalf("upsert interstitial: %v", err)
	}

	flags := &rootFlags{dataSource: "local"}
	_, _, err := resolveRead(context.Background(), nil, flags, "orders", false, "/your-orders/orders", nil, nil, "")
	if err == nil {
		t.Fatal("resolveRead local returned nil error for stored sign-in HTML")
	}
	if !parser.IsAuthInterstitialError(err) {
		t.Fatalf("error = %v, want auth interstitial error", err)
	}
}

func TestResolveReadLiveRejectsAuthInterstitial(t *testing.T) {
	c := client.New(&config.Config{BaseURL: "https://example.invalid"}, 0, 0)
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(signInInterstitialHTML)),
			Header:     make(http.Header),
		}, nil
	})}
	flags := &rootFlags{dataSource: "live"}
	_, _, err := resolveRead(context.Background(), c, flags, "orders", false, "/your-orders/orders", nil, nil, "")
	if err == nil {
		t.Fatal("resolveRead live returned nil error for sign-in HTML")
	}
	if !parser.IsAuthInterstitialError(err) {
		t.Fatalf("error = %v, want auth interstitial error", err)
	}
}

func TestResolveReadAutoRejectsAuthInterstitialWithoutCaching(t *testing.T) {
	db, home := testStoreAtHome(t)
	c := client.New(&config.Config{BaseURL: "https://example.invalid"}, 0, 0)
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(signInInterstitialHTML)),
			Header:     make(http.Header),
		}, nil
	})}

	flags := &rootFlags{dataSource: "auto"}
	_, _, err := resolveRead(context.Background(), c, flags, "orders", false, "/your-orders/orders", nil, nil, "")
	if err == nil {
		t.Fatal("resolveRead auto returned nil error for sign-in HTML")
	}
	if !parser.IsAuthInterstitialError(err) {
		t.Fatalf("error = %v, want auth interstitial error", err)
	}
	ids, err := db.ListIDs("orders")
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("stored IDs = %v, want none", ids)
	}
	cacheDir := filepath.Join(home, ".cache", "amazon-orders-pp-cli")
	if entries, readErr := os.ReadDir(cacheDir); readErr == nil && len(entries) > 0 {
		t.Fatalf("HTTP cache entries = %d, want none", len(entries))
	}
}

func TestResolveReadLocalListSkipsAuthInterstitialRows(t *testing.T) {
	db := testStore(t)
	if err := db.Upsert("orders", "auth", []byte(signInInterstitialHTML)); err != nil {
		t.Fatalf("upsert interstitial: %v", err)
	}
	if err := db.Upsert("orders", "real", []byte(`{"id":"real","orderId":"111-1111111-1111111"}`)); err != nil {
		t.Fatalf("upsert real row: %v", err)
	}

	flags := &rootFlags{dataSource: "local"}
	data, _, err := resolveRead(context.Background(), nil, flags, "orders", true, "/your-orders/orders", nil, nil, "")
	if err != nil {
		t.Fatalf("resolveRead local list: %v", err)
	}
	if strings.Contains(string(data), "Sign-In") {
		t.Fatalf("local list data includes sign-in HTML: %s", data)
	}
	if !strings.Contains(string(data), "111-1111111-1111111") {
		t.Fatalf("local list data = %s, want real order row", data)
	}
}

func TestResolveReadLocalListAllAuthInterstitialRowsErrors(t *testing.T) {
	db := testStore(t)
	if err := db.Upsert("orders", "auth", []byte(signInInterstitialHTML)); err != nil {
		t.Fatalf("upsert interstitial: %v", err)
	}

	flags := &rootFlags{dataSource: "local"}
	_, _, err := resolveRead(context.Background(), nil, flags, "orders", true, "/your-orders/orders", nil, nil, "")
	if err == nil {
		t.Fatal("resolveRead local list returned nil error for all-tainted rows")
	}
	if !parser.IsAuthInterstitialError(err) {
		t.Fatalf("error = %v, want auth interstitial error", err)
	}
}

func TestSyncResourceRejectsAuthInterstitial(t *testing.T) {
	db := testStore(t)
	res := syncResource(fakeSyncClient{body: signInInterstitialHTML}, db, "orders", "", true, 1)
	if res.Err == nil {
		t.Fatalf("syncResource err = nil, count=%d; want auth interstitial error", res.Count)
	}
	if !parser.IsAuthInterstitialError(res.Err) {
		t.Fatalf("error = %v, want auth interstitial error", res.Err)
	}
	ids, err := db.ListIDs("orders")
	if err != nil {
		t.Fatalf("ListIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("stored IDs = %v, want none", ids)
	}
}
