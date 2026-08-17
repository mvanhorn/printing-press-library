// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizedReorderRemovesPaymentAndHistoricalIdentifiers(t *testing.T) {
	input := map[string]any{
		"StoreID":       "54321",
		"ServiceMethod": "Delivery",
		"OrderID":       "historical-order-id",
		"Payments": []any{
			map[string]any{"CardID": "opaque-token", "MaskedNumber": "************1234"},
		},
		"Customer": map[string]any{
			"FirstName":   "Test",
			"CreditCards": []any{map[string]any{"LastFour": "1234"}},
			"Cards":       []any{map[string]any{"OpaqueReference": "must-not-survive"}},
			"Address":     map[string]any{"PostalCode": "V4K1V8"},
		},
		"Products": []any{map[string]any{"Code": "12GARFIN", "Qty": float64(1)}},
	}

	got := sanitizedReorder(input)
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"historical-order-id", "opaque-token", "must-not-survive", "1234", "Payments", "CreditCards", "Cards", "CardID", "MaskedNumber"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("sanitized order leaked %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{"54321", "Delivery", "12GARFIN", "PostalCode"} {
		if !strings.Contains(text, required) {
			t.Fatalf("sanitized order dropped required value %q: %s", required, text)
		}
	}
}

func TestLatestHistoricalOrderSupportsCanadianEnvelope(t *testing.T) {
	data := json.RawMessage(`{"customerOrders":[{"order":{"StoreID":"54321","ServiceMethod":"Delivery"}}]}`)
	order, err := latestHistoricalOrder(data)
	if err != nil {
		t.Fatal(err)
	}
	if order["StoreID"] != "54321" {
		t.Fatalf("StoreID = %v, want 54321", order["StoreID"])
	}
}

func TestSavedCardCountNeverRequiresCardDetails(t *testing.T) {
	tests := []struct {
		name string
		data json.RawMessage
		want int
	}{
		{"empty object", json.RawMessage(`{}`), 0},
		{"wrapped cards", json.RawMessage(`{"CreditCards":[{"CardID":"secret"},{"CardID":"secret2"}]}`), 2},
		{"array", json.RawMessage(`[{"CardID":"secret"}]`), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := savedCardCount(tt.data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("savedCardCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSavedCardCountRejectsUnknownNonEmptyShape(t *testing.T) {
	if _, err := savedCardCount(json.RawMessage(`{"dry_run":true}`)); err == nil {
		t.Fatal("savedCardCount accepted an unknown non-empty response")
	}
	if _, err := savedCardCount(json.RawMessage(`{"CreditCards":{}}`)); err == nil {
		t.Fatal("savedCardCount accepted a non-array CreditCards field")
	}
}

func TestValidationSucceededDistinguishesUnknownResponse(t *testing.T) {
	if ok, known := validationSucceeded(json.RawMessage(`{"Status":1}`)); !known || !ok {
		t.Fatalf("Status=1 produced ok=%v known=%v", ok, known)
	}
	if ok, known := validationSucceeded(json.RawMessage(`{"Status":0}`)); !known || !ok {
		t.Fatalf("Status=0 produced ok=%v known=%v", ok, known)
	}
	if ok, known := validationSucceeded(json.RawMessage(`{"Status":-1}`)); !known || ok {
		t.Fatalf("Status=-1 produced ok=%v known=%v", ok, known)
	}
	if ok, known := validationSucceeded(json.RawMessage(`{"Status":2}`)); known || ok {
		t.Fatalf("unrecognized numeric status produced ok=%v known=%v", ok, known)
	}
	if ok, known := validationSucceeded(json.RawMessage(`{"Status":"2"}`)); known || ok {
		t.Fatalf("unrecognized string status produced ok=%v known=%v", ok, known)
	}
	if ok, known := validationSucceeded(json.RawMessage(`{"message":"accepted"}`)); known || ok {
		t.Fatalf("unknown response produced ok=%v known=%v", ok, known)
	}
}

func TestExtractCustomerTotalUsesKnownPath(t *testing.T) {
	data := json.RawMessage(`{"unrelated":{"Total":1.25},"Order":{"Amounts":{"Tax":4.50,"Customer":42.17}}}`)
	got, err := extractCustomerTotal(data)
	if err != nil {
		t.Fatal(err)
	}
	if got != float64(42.17) {
		t.Fatalf("total = %v, want 42.17", got)
	}
	if _, err := extractCustomerTotal(json.RawMessage(`{"unrelated":{"Total":1.25}}`)); err == nil {
		t.Fatal("accepted a nested unrelated total")
	}
}

func TestPaymentCapabilitiesUsesAllowlist(t *testing.T) {
	data := json.RawMessage(`{
		"Store":{"AcceptCash":true,"AcceptSavedCreditCard":true,"AllowCardSaving":true,"SecretMerchantKey":"do-not-print"}
	}`)
	got, err := paymentCapabilities(data)
	if err != nil {
		t.Fatal(err)
	}
	if got["AcceptCash"] != true || got["AcceptSavedCreditCard"] != true || got["AllowCardSaving"] != true {
		t.Fatalf("expected allowed capabilities, got %#v", got)
	}
	if _, exists := got["SecretMerchantKey"]; exists {
		t.Fatalf("unexpected secret field in capability output: %#v", got)
	}
	if _, err := paymentCapabilities(json.RawMessage(`not-json`)); err == nil {
		t.Fatal("accepted malformed store profile response")
	}
}

func TestCheckoutPreviewNeverCallsPlaceOrderAndRedactsHistoricalPayment(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.Method+" "+r.URL.Path)
		if strings.Contains(strings.ToLower(r.URL.Path), "place-order") {
			t.Fatalf("checkout preview called placement endpoint: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/order"):
			fmt.Fprint(w, `{"customerOrders":[{"order":{"StoreID":"54321","ServiceMethod":"Delivery","OrderID":"old-order","Payments":[{"CardID":"opaque-card","MaskedNumber":"************1234"}],"Customer":{"FirstName":"Private"},"Products":[{"Code":"12GARFIN","Qty":1}]}}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/power/validate-order":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"old-order", "opaque-card", "1234", "Payments", "MaskedNumber"} {
				if bytes.Contains(body, []byte(forbidden)) {
					t.Fatalf("validate body leaked %q: %s", forbidden, body)
				}
			}
			fmt.Fprint(w, `{"Status":0,"Order":{"StoreID":"54321","ServiceMethod":"Delivery","Products":[{"Code":"NORMALIZED","Qty":1}]}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/power/price-order":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(body, []byte("NORMALIZED")) || bytes.Contains(body, []byte("12GARFIN")) {
				t.Fatalf("price body did not use validated order: %s", body)
			}
			fmt.Fprint(w, `{"Status":0,"Order":{"Amounts":{"Customer":42.17}}}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/card"):
			fmt.Fprint(w, `{"CreditCards":[{"CardID":"must-not-appear","MaskedNumber":"************5678"}]}`)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/profile"):
			fmt.Fprint(w, `{"Store":{"AcceptSavedCreditCard":true,"AllowCardSaving":true,"SecretMerchantKey":"must-not-appear"}}`)
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	configBody := fmt.Sprintf("base_url = %q\nmarket = 'ca'\naccess_token = 'test-token-more-than-twenty-characters'\ncustomer_id = 'customer-id'\n", server.URL)
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config", configPath, "--json", "--no-cache", "checkout", "preview", "--last"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout preview failed: %v\nstderr: %s", err, stderr.String())
	}
	output := stdout.String()
	for _, forbidden := range []string{"must-not-appear", "opaque-card", "Private", "MaskedNumber", "CardID"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("checkout output leaked %q: %s", forbidden, output)
		}
	}
	for _, required := range []string{`"priced_total": 42.17`, `"pricing_ok": true`, `"saved_card_count": 1`, `"place_order_available": false`, `"placed": false`} {
		if !strings.Contains(output, required) {
			t.Fatalf("checkout output missing %s: %s", required, output)
		}
	}
	wantRequests := []string{
		"GET /power/customer/customer-id/order",
		"POST /power/validate-order",
		"POST /power/price-order",
		"GET /power/customer/customer-id/card",
		"GET /power/store/54321/profile",
	}
	if strings.Join(requested, "\n") != strings.Join(wantRequests, "\n") {
		t.Fatalf("requests = %#v, want %#v", requested, wantRequests)
	}
}

func TestCheckoutPreviewRejectsUnknownApplicationStatus(t *testing.T) {
	tests := []struct {
		name               string
		validationResponse string
		pricingResponse    string
		wantRequests       int
		wantError          string
	}{
		{
			name:               "validation",
			validationResponse: `{"message":"accepted"}`,
			pricingResponse:    `{"Status":0,"Order":{"Amounts":{"Customer":42.17}}}`,
			wantRequests:       2,
			wantError:          "validation response did not include a recognized success status",
		},
		{
			name:               "pricing",
			validationResponse: `{"Status":0,"Order":{"StoreID":"54321","ServiceMethod":"Delivery","Products":[]}}`,
			pricingResponse:    `{"Order":{"Amounts":{"Customer":42.17}}}`,
			wantRequests:       3,
			wantError:          "pricing response did not include a recognized success status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requested []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requested = append(requested, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/order"):
					fmt.Fprint(w, `{"customerOrders":[{"order":{"StoreID":"54321","ServiceMethod":"Delivery","Products":[]}}]}`)
				case r.Method == http.MethodPost && r.URL.Path == "/power/validate-order":
					fmt.Fprint(w, tt.validationResponse)
				case r.Method == http.MethodPost && r.URL.Path == "/power/price-order":
					fmt.Fprint(w, tt.pricingResponse)
				default:
					t.Fatalf("unexpected request after unknown %s status: %s %s", tt.name, r.Method, r.URL.Path)
				}
			}))
			defer server.Close()

			configPath := filepath.Join(t.TempDir(), "config.toml")
			configBody := fmt.Sprintf("base_url = %q\nmarket = 'ca'\naccess_token = 'test-token-more-than-twenty-characters'\ncustomer_id = 'customer-id'\n", server.URL)
			if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
				t.Fatal(err)
			}

			flags := &rootFlags{}
			cmd := newRootCmd(flags)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--config", configPath, "--json", "--no-cache", "checkout", "preview", "--last"})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("checkout preview error = %v, want substring %q", err, tt.wantError)
			}
			if len(requested) != tt.wantRequests {
				t.Fatalf("requests = %#v, want %d requests", requested, tt.wantRequests)
			}
		})
	}
}

func TestCheckoutPreviewDryRunStopsBeforeParsingHistory(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("customer_id = 'synthetic-customer'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config", configPath, "--json", "--dry-run", "checkout", "preview", "--last"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run preview failed: %v\nstderr: %s", err, stderr.String())
	}
	output := stdout.String()
	for _, required := range []string{`"dry_run": true`, `"lookup_performed": false`, `"dependent_requests_previewed": false`, `"placed": false`} {
		if !strings.Contains(output, required) {
			t.Fatalf("dry-run output missing %s: %s", required, output)
		}
	}
	if strings.Contains(output, "saved_card_count") {
		t.Fatalf("dry-run output claimed account state: %s", output)
	}
}

func TestCustomerCardsDryRunDoesNotClaimCardState(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--json", "--dry-run", "customer", "cards", "synthetic-customer"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run cards failed: %v\nstderr: %s", err, stderr.String())
	}
	output := stdout.String()
	for _, required := range []string{`"dry_run": true`, `"lookup_performed": false`} {
		if !strings.Contains(output, required) {
			t.Fatalf("dry-run output missing %s: %s", required, output)
		}
	}
	for _, forbidden := range []string{"saved_card_count", "has_saved_cards"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("dry-run output claimed %s: %s", forbidden, output)
		}
	}
}

func TestCanadianPlacementIsRejectedBeforeRequest(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("market = 'ca'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--config", configPath, "orders", "place", "--dry-run"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "Canadian order placement is not supported") {
		t.Fatalf("got error %v", err)
	}
}

func TestCustomerCommandRejectsCredentialMarketMismatch(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.toml")
	configBody := "market = 'us'\ncredential_market = 'us'\naccess_token = 'stored-token'\ncustomer_id = 'us-customer'\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatal(err)
	}

	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--config", configPath, "--market", "ca", "customer", "orders", "--dry-run"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "stored credentials belong to market us") {
		t.Fatalf("got error %v", err)
	}
}

func TestEnvironmentTokenRequiresExplicitCustomerID(t *testing.T) {
	t.Setenv("DOMINOS_TOKEN", "invocation-token")
	t.Setenv("DOMINOS_MARKET", "ca")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("market = 'ca'\ncredential_market = 'ca'\ncustomer_id = 'stale-customer'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--config", configPath, "checkout", "preview", "--last", "--dry-run"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "pass <customerID> explicitly") {
		t.Fatalf("got error %v", err)
	}
}
