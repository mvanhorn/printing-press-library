// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestOverlapExplainsSharedTokens(t *testing.T) {
	shared, score := overlap("Acme model 123 toaster", "Acme recalls model 123 toaster for fire hazard")
	if len(shared) < 4 || score <= 0 {
		t.Fatalf("shared=%v score=%v", shared, score)
	}
}

func TestExactInventoryEvidence(t *testing.T) {
	recall := map[string]any{"Products": []any{map[string]any{"Name": "Toaster", "Model": "T-1", "UPC": "123"}}, "Manufacturers": []any{map[string]any{"Name": "Acme"}}}
	evidence := exactInventoryEvidence(map[string]string{"name": "toaster", "brand": "ACME", "model": "T-1", "upc": "123"}, recall)
	if !hasExactInventoryEvidence(evidence) {
		t.Fatalf("expected exact evidence: %#v", evidence)
	}
	for _, field := range []string{"name", "brand", "model", "upc"} {
		if !evidence[field].(map[string]any)["exact"].(bool) {
			t.Fatalf("%s did not match", field)
		}
	}
}

func TestCanonicalMaterialRecallIgnoresArrayOrder(t *testing.T) {
	a := map[string]any{"RecallID": "1", "Hazards": []any{map[string]any{"Name": "Fire"}, map[string]any{"Name": "Burn"}}}
	b := map[string]any{"RecallID": "1", "Hazards": []any{map[string]any{"Name": "Burn"}, map[string]any{"Name": "Fire"}}}
	left, _ := canonicalMaterialRecall(a)
	right, _ := canonicalMaterialRecall(b)
	if string(left) != string(right) {
		t.Fatalf("canonical values differ:\n%s\n%s", left, right)
	}
}

func TestCPSCFetcherRetriesProviderError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			_, _ = w.Write([]byte(`[{"Title":"Error retrieving Recalls: temporary"}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"RecallID":"10000"}]`))
	}))
	defer server.Close()
	fetcher := &cpscFetcher{baseURL: server.URL, httpClient: server.Client(), maxAttempts: 2, waitRetry: func(context.Context, time.Duration) error { return nil }}
	rows, err := fetcher.fetch(context.Background(), url.Values{"RecallID": {"10000"}})
	if err != nil || attempts != 2 || len(rows) != 1 {
		t.Fatalf("attempts=%d rows=%v err=%v", attempts, rows, err)
	}
}

func TestCPSCFetcherRejectsProviderErrorMixedWithRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"RecallID":"10000"},{"Title":"Error retrieving Recalls: partial failure"}]`))
	}))
	defer server.Close()
	fetcher := &cpscFetcher{baseURL: server.URL, httpClient: server.Client(), maxAttempts: 1, waitRetry: func(context.Context, time.Duration) error { return nil }}
	if _, err := fetcher.fetch(context.Background(), nil); err == nil {
		t.Fatal("mixed provider error row was accepted")
	}
}
