// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/mutationguard"
)

func duplicateTestServer(t *testing.T, status int) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"output":{"ok":true}}`))
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func TestIdenticalWriteAfterSuccessIsRefused(t *testing.T) {
	server, calls := duplicateTestServer(t, http.StatusOK)
	client := newRetryTestClient(server.URL)
	client.HTTPClient = server.Client()
	approveTestWrites(t, client)

	body := map[string]any{"requestedShipment": map[string]any{"serviceType": "FEDEX_GROUND", "weight": 5}}
	if _, _, err := client.Post("/ship/v1/shipments", body); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// Same content, different key order and number spelling: still a duplicate.
	reordered := map[string]any{"requestedShipment": map[string]any{"weight": 5.0, "serviceType": "FEDEX_GROUND"}}
	_, _, err := client.Post("/ship/v1/shipments", reordered)
	var duplicate *mutationguard.DuplicateError
	if !errors.As(err, &duplicate) {
		t.Fatalf("identical write error=%v, want DuplicateError", err)
	}
	if *calls != 1 {
		t.Fatalf("duplicate write reached FedEx: calls=%d, want 1", *calls)
	}

	different := map[string]any{"requestedShipment": map[string]any{"serviceType": "FEDEX_GROUND", "weight": 6}}
	if _, _, err := client.Post("/ship/v1/shipments", different); err != nil {
		t.Fatalf("different write: %v", err)
	}
	if *calls != 2 {
		t.Fatalf("different write calls=%d, want 2", *calls)
	}

	client.AllowDuplicate = true
	if _, _, err := client.Post("/ship/v1/shipments", body); err != nil {
		t.Fatalf("forced duplicate: %v", err)
	}
	if *calls != 3 {
		t.Fatalf("forced duplicate calls=%d, want 3", *calls)
	}
}

func TestWriteWithUnknownOutcomeBlocksRetry(t *testing.T) {
	server, calls := duplicateTestServer(t, http.StatusInternalServerError)
	client := newRetryTestClient(server.URL)
	client.HTTPClient = server.Client()
	approveTestWrites(t, client)

	body := map[string]any{"carrierCode": "FDXG", "packageCount": 1}
	_, _, err := client.Post("/pickup/v1/pickups", body)
	var unknown *OutcomeUnknownError
	if !errors.As(err, &unknown) {
		t.Fatalf("first pickup error=%v, want OutcomeUnknownError", err)
	}
	_, _, err = client.Post("/pickup/v1/pickups", body)
	var duplicate *mutationguard.DuplicateError
	if !errors.As(err, &duplicate) || duplicate.Status != mutationguard.StatusOutcomeUnknown {
		t.Fatalf("retry after unknown outcome error=%v, want outcome_unknown DuplicateError", err)
	}
	if *calls != 1 {
		t.Fatalf("retry reached FedEx: calls=%d, want 1", *calls)
	}
}

func TestRejectedWriteCanBeResent(t *testing.T) {
	server, calls := duplicateTestServer(t, http.StatusBadRequest)
	client := newRetryTestClient(server.URL)
	client.HTTPClient = server.Client()
	approveTestWrites(t, client)

	body := map[string]any{"trackingNumber": "000000000000"}
	for attempt := 1; attempt <= 2; attempt++ {
		_, _, err := client.Put("/ship/v1/shipments/cancel", body)
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("attempt %d error=%v, want APIError", attempt, err)
		}
	}
	if *calls != 2 {
		t.Fatalf("rejected write resend calls=%d, want 2", *calls)
	}
}

func TestReadOnlyPostIsNeverDuplicateGated(t *testing.T) {
	server, calls := duplicateTestServer(t, http.StatusOK)
	client := newRetryTestClient(server.URL)
	client.HTTPClient = server.Client()
	client.MutationLedgerDir = t.TempDir()

	body := map[string]any{"trackingInfo": []any{map[string]any{"trackingNumberInfo": map[string]any{"trackingNumber": "1"}}}}
	for i := 0; i < 2; i++ {
		if _, _, err := client.Post("/track/v1/trackingnumbers", body); err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
	}
	if *calls != 2 {
		t.Fatalf("read-only calls=%d, want 2", *calls)
	}
}

func TestMCPChildProcessCannotWrite(t *testing.T) {
	server, calls := duplicateTestServer(t, http.StatusOK)
	client := newRetryTestClient(server.URL)
	client.HTTPClient = server.Client()
	approveTestWrites(t, client)
	t.Setenv(MCPChildEnv, "1")

	if _, _, err := client.Post("/ship/v1/shipments", map[string]any{"x": 1}); !errors.Is(err, ErrCompanionWriteRefused) {
		t.Fatalf("child write error=%v, want ErrCompanionWriteRefused", err)
	}
	if *calls != 0 {
		t.Fatalf("child write reached FedEx: calls=%d", *calls)
	}
}
