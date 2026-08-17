// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProductLookupUsesV3ProductEndpointAndIdentifiedUserAgent(t *testing.T) {
	var sawProduct bool
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/product/737628064502.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawProduct = true
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, "open-food-facts-tests") || !strings.Contains(ua, "food@example.test") {
			t.Fatalf("User-Agent = %q", ua)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "737628064502",
			"status": "success",
			"product": {
				"code": "737628064502",
				"product_name": "Thai Kitchen Red Curry Paste",
				"brands": "Thai Kitchen",
				"quantity": "4 oz",
				"categories_tags": ["en:groceries", "en:curries"],
				"labels_tags": ["en:gluten-free"],
				"countries_tags": ["en:united-states"],
				"nutriscore_grade": "c",
				"nova_group": 3,
				"ecoscore_grade": "d",
				"ingredients_text": "red chili pepper, garlic, lemongrass",
				"allergens_tags": ["en:soybeans"],
				"traces_tags": ["en:peanuts"],
				"additives_tags": ["en:e330"],
				"nutriments": {"energy-kcal_100g": 120, "sugars_100g": 6, "salt_100g": 2.4},
				"data_quality_tags": ["en:nutrition-value-over-105-for-salt"]
			}
		}`))
	}))

	t.Setenv(baseURLEnv, "https://openfoodfacts.test")
	t.Setenv(userAgentEnv, "open-food-facts-tests")
	t.Setenv(contactEmailEnv, "food@example.test")

	output := runCLI(t, "product", "737628064502", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal product output: %v\n%s", err, output)
	}
	if !sawProduct {
		t.Fatalf("product endpoint was not called")
	}
	if result["source"] != "Open Food Facts Product API v3" {
		t.Fatalf("source = %#v", result["source"])
	}
	product := result["product"].(map[string]any)
	if product["name"] != "Thai Kitchen Red Curry Paste" {
		t.Fatalf("product = %#v", product)
	}
	if product["nutriscore_grade"] != "c" {
		t.Fatalf("nutriscore_grade = %#v", product["nutriscore_grade"])
	}
}

func TestSearchUsesBoundedV2StructuredSearch(t *testing.T) {
	var sawSearch bool
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/search" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawSearch = true
		query := r.URL.Query()
		if got := query.Get("categories_tags_en"); got != "breakfast cereals" {
			t.Fatalf("categories_tags_en = %q", got)
		}
		if got := query.Get("countries_tags_en"); got != "united-states" {
			t.Fatalf("countries_tags_en = %q", got)
		}
		if got := query.Get("page_size"); got != "2" {
			t.Fatalf("page_size = %q", got)
		}
		if fields := query.Get("fields"); !strings.Contains(fields, "product_name") || !strings.Contains(fields, "nutriments") {
			t.Fatalf("fields = %q", fields)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"count": 1,
			"page": 1,
			"page_size": 2,
			"products": [{
				"code": "000000000001",
				"product_name": "Example Cereal",
				"brands": "Example Brand",
				"categories_tags": ["en:breakfast-cereals"],
				"nutriscore_grade": "b",
				"nutriments": {"sugars_100g": 8}
			}]
		}`))
	}))

	t.Setenv(baseURLEnv, "https://openfoodfacts.test")

	output := runCLI(t, "search", "--category", "breakfast cereals", "--country", "united-states", "--page-size", "2", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal search output: %v\n%s", err, output)
	}
	if !sawSearch {
		t.Fatalf("search endpoint was not called")
	}
	results := result["results"].([]any)
	if results[0].(map[string]any)["name"] != "Example Cereal" {
		t.Fatalf("results = %#v", results)
	}
}

func TestCompareReturnsSuccessfulProductsAndPerBarcodeErrors(t *testing.T) {
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/product/111.json":
			_, _ = w.Write([]byte(`{"code":"111","status":"success","product":{"code":"111","product_name":"Alpha","brands":"A","nutriscore_grade":"a","allergens_tags":[],"nutriments":{"sugars_100g":1}}}`))
		case "/api/v3/product/222.json":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"222","status":"failure","errors":[{"message":"not found"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))

	t.Setenv(baseURLEnv, "https://openfoodfacts.test")

	output := runCLI(t, "compare", "111", "222", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal compare output: %v\n%s", err, output)
	}
	products := result["products"].([]any)
	if len(products) != 1 || products[0].(map[string]any)["barcode"] != "111" {
		t.Fatalf("products = %#v", products)
	}
	errors := result["errors"].([]any)
	if len(errors) != 1 || errors[0].(map[string]any)["barcode"] != "222" {
		t.Fatalf("errors = %#v", errors)
	}
}

func TestCategoryUsesBoundedStructuredSearch(t *testing.T) {
	var sawCategory bool
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/search" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		sawCategory = true
		query := r.URL.Query()
		if got := query.Get("categories_tags_en"); got != "breakfast cereals" {
			t.Fatalf("categories_tags_en = %q", got)
		}
		if got := query.Get("page_size"); got != "2" {
			t.Fatalf("page_size = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"count": 1,
			"page": 1,
			"page_size": 2,
			"products": [{
				"code": "000000000002",
				"product_name": "Example Oats",
				"brands": "Example Brand",
				"categories_tags": ["en:breakfast-cereals"],
				"nutriments": {"fiber_100g": 9}
			}]
		}`))
	}))

	t.Setenv(baseURLEnv, "https://openfoodfacts.test")

	output := runCLI(t, "category", "breakfast cereals", "--page-size", "2", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal category output: %v\n%s", err, output)
	}
	if !sawCategory {
		t.Fatalf("category endpoint was not called")
	}
	results := result["results"].([]any)
	if results[0].(map[string]any)["name"] != "Example Oats" {
		t.Fatalf("results = %#v", results)
	}
}

func TestSourcesReportsNoAuthAndNoBulkPosture(t *testing.T) {
	output := runCLI(t, "sources", "--agent")

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("unmarshal sources output: %v\n%s", err, output)
	}
	if result["auth"] != "none for read operations" {
		t.Fatalf("auth = %#v", result["auth"])
	}
	caveats := result["caveats"].([]any)
	if !containsString(caveats, "bulk") {
		t.Fatalf("caveats did not mention bulk guidance: %#v", caveats)
	}
}

func TestTrimTagsStripsLocalePrefixes(t *testing.T) {
	got := trimTags([]string{
		"en:sweet-spreads",
		"fr:pates-a-tartiner",
		"de:schokolade",
		"es:azucar",
		"plain-tag",
		"too-long:prefix",
		"fr:",
	})
	want := []string{
		"sweet spreads",
		"pates a tartiner",
		"schokolade",
		"azucar",
		"plain tag",
		"too long:prefix",
	}
	if len(got) != len(want) {
		t.Fatalf("trimTags length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("trimTags[%d] = %q, want %q; all = %#v", i, got[i], want[i], got)
		}
	}
}

func TestHTTPErrorBodyIsSummarized(t *testing.T) {
	withTestTransport(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(strings.Repeat("x", 2000)))
	}))

	t.Setenv(baseURLEnv, "https://openfoodfacts.test")

	var target map[string]any
	err := newClient(time.Second).getJSON(context.Background(), "/api/v2/search", nil, &target)
	if err == nil {
		t.Fatalf("expected HTTP error")
	}
	message := err.Error()
	if !strings.Contains(message, "HTTP 503") {
		t.Fatalf("error did not include status: %s", message)
	}
	if strings.Contains(message, strings.Repeat("x", 1000)) || len(message) > 700 {
		t.Fatalf("error body was not summarized: length=%d", len(message))
	}
}

func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	flags := rootFlags{timeout: time.Second}
	cmd := newRootCmd(&flags)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v\nstderr: %s", args, err, stderr.String())
	}
	return stdout.String()
}

func containsString(items []any, needle string) bool {
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.(string)), strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func withTestTransport(t *testing.T, handler http.Handler) {
	t.Helper()
	previous := newHTTPClient
	newHTTPClient = func(timeout time.Duration) *http.Client {
		return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			resp := recorder.Result()
			if resp.Body == nil {
				resp.Body = io.NopCloser(strings.NewReader(""))
			}
			return resp, nil
		})}
	}
	t.Cleanup(func() {
		newHTTPClient = previous
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
