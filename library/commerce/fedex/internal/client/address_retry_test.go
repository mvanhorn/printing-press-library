// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBillableAddressValidationDoesNotRetry(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	client := newRetryTestClient(server.URL)
	client.HTTPClient = server.Client()
	client.retrySleep = func(time.Duration) {}
	_, _, err := client.Post("/address/v1/addresses/resolve", map[string]any{"addressesToValidate": []any{}})
	if err == nil {
		t.Fatal("address validation HTTP 500 returned nil error")
	}
	if calls != 1 {
		t.Fatalf("address validation calls=%d, want 1", calls)
	}
}
