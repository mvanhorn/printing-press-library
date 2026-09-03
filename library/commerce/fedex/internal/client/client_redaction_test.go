// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
)

func TestDryRunOutputContainsNoRequestPIIOrCredentials(t *testing.T) {
	var output bytes.Buffer
	client := New(&config.Config{
		BaseURL:        "https://apis-sandbox.fedex.com",
		FedexApiKey:    "sentinel-client-id",
		FedexSecretKey: "sentinel-client-secret",
	}, time.Second, 0)
	client.DryRun = true
	client.DryRunWriter = &output
	client.cacheDir = ""
	request := map[string]any{
		"accountNumber": map[string]any{"value": "sentinel-account-number"},
		"requestedShipment": map[string]any{
			"recipient": map[string]any{
				"contact": map[string]any{"personName": "sentinel-recipient", "phoneNumber": "sentinel-phone"},
				"address": map[string]any{"streetLines": []any{"sentinel-street"}, "postalCode": "sentinel-postal"},
			},
		},
	}
	if _, _, err := client.Post("/ship/v1/shipments", request); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	for _, forbidden := range []string{"sentinel-client-id", "sentinel-client-secret", "sentinel-account-number", "sentinel-recipient", "sentinel-phone", "sentinel-street", "sentinel-postal"} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("dry-run output leaked %q: %s", forbidden, output.String())
		}
	}
	if !strings.Contains(output.String(), `"dry_run": true`) {
		t.Fatalf("dry-run output missing structured marker: %s", output.String())
	}
}

func TestAPIErrorDoesNotExposeResponseBodySecretsOrPII(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":[{"code":"INVALID.INPUT","message":"sentinel-recipient sentinel-client-secret"}],"echo":"sentinel-street"}`))
	}))
	t.Cleanup(server.Close)

	client := newRetryTestClient(server.URL)
	client.HTTPClient = server.Client()
	_, _, err := client.Post("/rate/v1/rates/quotes", map[string]any{"test": true})
	if err == nil {
		t.Fatal("expected API error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type=%T, want *APIError", err)
	}
	for _, text := range []string{err.Error(), apiErr.Body} {
		for _, forbidden := range []string{"sentinel-recipient", "sentinel-client-secret", "sentinel-street"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("API error leaked %q: %s", forbidden, text)
			}
		}
	}
	if !strings.Contains(err.Error(), "INVALID.INPUT") {
		t.Fatalf("API error omitted allowlisted code: %s", err)
	}
}
