package airbnb

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestBuildCookieHeader_Multiple(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "session", Value: "abc"},
		{Name: "csrf", Value: "xyz"},
	}
	got := buildCookieHeader(cookies)
	want := "session=abc; csrf=xyz"
	if got != want {
		t.Errorf("buildCookieHeader = %q, want %q", got, want)
	}
}

func TestBuildCookieHeader_Empty(t *testing.T) {
	if got := buildCookieHeader(nil); got != "" {
		t.Errorf("buildCookieHeader(nil) = %q, want empty", got)
	}
	if got := buildCookieHeader([]*http.Cookie{}); got != "" {
		t.Errorf("buildCookieHeader([]) = %q, want empty", got)
	}
}

func TestBuildCookieHeader_SkipsNilAndEmptyName(t *testing.T) {
	cookies := []*http.Cookie{
		nil,
		{Name: "", Value: "ignored"},
		{Name: "ok", Value: "v"},
	}
	got := buildCookieHeader(cookies)
	if !strings.Contains(got, "ok=v") || strings.Contains(got, "ignored") {
		t.Errorf("buildCookieHeader = %q, want only ok=v", got)
	}
}

func TestPriceBreakdownFromAnySkipsLegacyFeesWhenStructuredFeesExist(t *testing.T) {
	root := map[string]any{
		"pdpPresentation": map[string]any{
			"bookIt": map[string]any{
				"structuredDisplayPrice": map[string]any{
					"primaryLine": map[string]any{"price": "$200"},
					"explanationData": map[string]any{
						"priceDetails": []any{
							map[string]any{
								"items": []any{
									map[string]any{"description": "Cleaning fee", "priceString": "$25"},
									map[string]any{"description": "Service fee", "priceString": "$30"},
								},
							},
						},
					},
				},
			},
		},
		"legacyFees": []any{
			map[string]any{"label": "Cleaning fee", "amount": float64(25)},
			map[string]any{"label": "Service fee", "amount": float64(30)},
		},
	}

	got := priceBreakdownFromAny(root)
	if got.Fees["cleaning"] != 25 {
		t.Fatalf("cleaning fee = %v, want structured fee only", got.Fees["cleaning"])
	}
	if got.Fees["service"] != 30 {
		t.Fatalf("service fee = %v, want structured fee only", got.Fees["service"])
	}
}

func TestPriceBreakdownFromAnyLegacySubtotalDoesNotSetTotal(t *testing.T) {
	root := map[string]any{
		"legacyFees": []any{
			map[string]any{"label": "Subtotal", "amount": float64(120)},
			map[string]any{"label": "Total", "amount": float64(150)},
		},
	}

	got := priceBreakdownFromAny(root)
	if got.Subtotal != 120 {
		t.Fatalf("Subtotal = %v, want 120", got.Subtotal)
	}
	if got.Total != 150 {
		t.Fatalf("Total = %v, want 150", got.Total)
	}
}

// TestParseAPIKeyFromSSR confirms the regex extracts the api_config key
// embedded in airbnb.com SSR HTML.
func TestParseAPIKeyFromSSR(t *testing.T) {
	html := []byte(`...,"locale":"en","api_config":{"key":"d306zoyjsyarp7ifhu67rjxn52tv0t20","baseUrl":"/api"},...`)
	got := parseAPIKey(html)
	if got != "d306zoyjsyarp7ifhu67rjxn52tv0t20" {
		t.Fatalf("parseAPIKey = %q, want d306zoyjsyarp7ifhu67rjxn52tv0t20", got)
	}
}

// TestParseAPIKeyAcceptsFutureKeys ensures the regex matches any 20+ char
// lowercase alphanumeric value, not the literal constant.
func TestParseAPIKeyAcceptsFutureKeys(t *testing.T) {
	html := []byte(`"api_config":{"key":"abc123def456ghi789jklmno","baseUrl":"/api"}`)
	got := parseAPIKey(html)
	if got != "abc123def456ghi789jklmno" {
		t.Fatalf("parseAPIKey = %q, want abc123def456ghi789jklmno", got)
	}
}

// TestParseAPIKeyEmptyOnNoMatch returns "" when no api_config block is
// present (e.g., bot-block page). resolveAPIKey then falls back to the
// constant.
func TestParseAPIKeyEmptyOnNoMatch(t *testing.T) {
	html := []byte(`<html><head><title>Access denied</title></head></html>`)
	got := parseAPIKey(html)
	if got != "" {
		t.Fatalf("parseAPIKey = %q, want empty on no match", got)
	}
}

// TestAirbnbDefaultAPIKey guards against accidentally clearing the
// fallback constant — the GraphQL gateway returns invalid_key 400 if it
// ever ships empty.
func TestAirbnbDefaultAPIKey(t *testing.T) {
	if airbnbDefaultAPIKey == "" {
		t.Fatal("airbnbDefaultAPIKey must not be empty")
	}
	if len(airbnbDefaultAPIKey) < 20 {
		t.Fatalf("airbnbDefaultAPIKey too short: %q", airbnbDefaultAPIKey)
	}
}

// TestResolveAPIKeyIgnoresCallerCancellation ensures the process-wide scrape
// is not poisoned by whichever request happens to trigger it first.
func TestResolveAPIKeyIgnoresCallerCancellation(t *testing.T) {
	apiKeyOnce = sync.Once{}
	apiKeyVal = ""
	t.Cleanup(func() {
		apiKeyOnce = sync.Once{}
		apiKeyVal = ""
	})

	called := false
	c := &Client{
		http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			called = true
			if err := req.Context().Err(); err != nil {
				t.Fatalf("homepage scrape inherited canceled caller context: %v", err)
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`"api_config":{"key":"freshkey1234567890abcdef","baseUrl":"/api"}`)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		})},
		limiter: defaultClient.limiter,
	}

	got := c.resolveAPIKey()
	if got != "freshkey1234567890abcdef" {
		t.Fatalf("resolveAPIKey = %q, want fresh key from scrape", got)
	}
	if !called {
		t.Fatal("resolveAPIKey did not try the homepage scrape")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestParseWishlistIndexReadsEachWishlist(t *testing.T) {
	// Shape of a real WishlistIndexPageQuery response (Nov 2026), trimmed.
	raw := `{"data":{"presentation":{"__typename":"RootPresentationContainer","wishlistIndexPage":{
		"__typename":"WishlistIndexPagePresentationContainer","wishlists":[
		{"__typename":"Wishlist","id":"V2lzaGxpc3Q6MTExMQ==","name":"Saved","productIds":{
			"stayIds":["U3RheUxpc3Rpbmc6MTIzNDU=","U3RheUxpc3Rpbmc6Njc4OTA="],"experienceIds":[],"placeIds":[]}},
		{"__typename":"Wishlist","id":"V2lzaGxpc3Q6MjIyMg==","name":"Empty","productIds":{
			"stayIds":[],"experienceIds":[],"placeIds":[]}}]}}}}`
	var root any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		t.Fatal(err)
	}
	got := parseWishlistIndex(root)
	if len(got) != 2 {
		t.Fatalf("want 2 wishlists, got %d: %+v", len(got), got)
	}
	if got[0].ID != "1111" || got[0].Name != "Saved" || got[0].Count != 2 {
		t.Errorf("first wishlist = %+v", got[0])
	}
	if strings.Join(got[0].ListingIDs, ",") != "12345,67890" {
		t.Errorf("listing ids = %v", got[0].ListingIDs)
	}
	if got[1].ID != "2222" || got[1].Name != "Empty" || got[1].Count != 0 {
		t.Errorf("second wishlist = %+v", got[1])
	}
}

func TestParseWishlistIndexNullWishlists(t *testing.T) {
	var root any
	_ = json.Unmarshal([]byte(`{"data":{"presentation":{"wishlistIndexPage":{"wishlists":null}}}}`), &root)
	if got := parseWishlistIndex(root); len(got) != 0 {
		t.Fatalf("want none, got %+v", got)
	}
}
