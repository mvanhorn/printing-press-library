// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/approval"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

func TestPickupPreflightRequiredAndExecutedDuringPreview(t *testing.T) {
	schedule, availability := pickupPreflightFixture()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != workflow.PickupAvailabilityPath {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"options":[{"carrier":"FDXG","available":true,"pickupDate":"2026-09-04","scheduleDay":"FRI","cutOffTime":"17:00"}]}}`))
	}))
	defer server.Close()
	fedexClient := client.New(&config.Config{
		BaseURL:      server.URL,
		AccessToken:  "test-token",
		TokenBaseURL: server.URL,
		TokenExpiry:  time.Now().Add(time.Hour),
	}, time.Second, 0)
	fedexClient.HTTPClient = server.Client()
	fedexClient.NoCache = true

	if _, err := preparePickupPreflight(&rootFlags{}, fedexClient, schedule, "", ""); err == nil {
		t.Fatal("missing preflight was accepted")
	}
	options, err := preparePickupPreflight(&rootFlags{}, fedexClient, schedule, availability, "")
	if err != nil {
		t.Fatalf("preparePickupPreflight: %v", err)
	}
	if calls != 1 {
		t.Fatalf("availability calls=%d, want 1", calls)
	}
	var summary approval.ReviewSummary
	options.ReviewUpdate(&summary)
	if summary.PickupPreflight != "verified" || !strings.Contains(summary.PickupWindow, "17:00") {
		t.Fatalf("review summary=%#v", summary)
	}
}

func TestPickupPreflightConfirmationDoesNotRepeatCall(t *testing.T) {
	schedule, availability := pickupPreflightFixture()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("confirmation repeated pickup availability call")
	}))
	defer server.Close()
	fedexClient := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	fedexClient.HTTPClient = server.Client()
	options, err := preparePickupPreflight(&rootFlags{yes: true}, fedexClient, schedule, availability, "")
	if err != nil {
		t.Fatalf("preparePickupPreflight: %v", err)
	}
	if options.Context == nil {
		t.Fatal("preflight context was not bound")
	}
}

func TestPickupPreflightRejectsUnavailableAndShortOverride(t *testing.T) {
	schedule, availability := pickupPreflightFixture()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"options":[{"carrier":"FDXG","available":false,"pickupDate":"2026-09-04","scheduleDay":"FRI"}]}}`))
	}))
	defer server.Close()
	fedexClient := client.New(&config.Config{
		BaseURL:      server.URL,
		AccessToken:  "test-token",
		TokenBaseURL: server.URL,
		TokenExpiry:  time.Now().Add(time.Hour),
	}, time.Second, 0)
	fedexClient.HTTPClient = server.Client()
	if _, err := preparePickupPreflight(&rootFlags{}, fedexClient, schedule, availability, ""); err == nil {
		t.Fatal("unavailable pickup was accepted")
	}
	if _, err := preparePickupPreflight(&rootFlags{}, fedexClient, schedule, "", "short"); err == nil {
		t.Fatal("short override reason was accepted")
	}
}

func pickupPreflightFixture() (map[string]any, string) {
	address := map[string]any{"streetLines": []any{"1 Test Way"}, "city": "Austin", "stateOrProvinceCode": "TX", "postalCode": "78701", "countryCode": "US"}
	schedule := map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"carrierCode":             "FDXG",
		"packageCount":            1,
		"totalWeight":             map[string]any{"units": "LB", "value": 2.0},
		"originDetail": map[string]any{
			"readyDateTimestamp": "2026-09-04T09:00:00-05:00",
			"customerCloseTime":  "17:00:00",
			"pickupLocation": map[string]any{
				"contact": map[string]any{"personName": "Test User", "phoneNumber": "5555550100"},
				"address": address,
			},
		},
	}
	availability := `{"associatedAccountNumber":"123456789","carriers":["FDXG"],"pickupRequestType":["SAME_DAY"],"countryRelationship":"DOMESTIC","dispatchDate":"2026-09-04","packageReadyTime":"09:00:00","customerCloseTime":"17:00:00","pickupAddress":{"streetLines":["1 Test Way"],"city":"Austin","stateOrProvinceCode":"TX","postalCode":"78701","countryCode":"US"}}`
	return schedule, availability
}
