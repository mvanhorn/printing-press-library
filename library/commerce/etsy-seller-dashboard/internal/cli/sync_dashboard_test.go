// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseLookbackSupportsPrintingPressDurations(t *testing.T) {
	tests := map[string]time.Duration{
		"7d":  7 * 24 * time.Hour,
		"1w":  7 * 24 * time.Hour,
		"24h": 24 * time.Hour,
		"30m": 30 * time.Minute,
	}
	for input, expected := range tests {
		actual, err := parseLookback(input)
		if err != nil {
			t.Fatalf("%s: %v", input, err)
		}
		if actual != expected {
			t.Fatalf("%s: got %v, want %v", input, actual, expected)
		}
	}
}

func TestDashboardSyncTasksRespectResourceSelection(t *testing.T) {
	tasks, err := dashboardSyncTasks("ads,promotions", "123", "2026-01-01", "2026-01-31", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].resource != "ads" ||
		tasks[1].label != "combined" || tasks[1].responsePath != "" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
	if tasks[0].params["limit"] != "50" {
		t.Fatalf("ads limit missing: %#v", tasks[0].params)
	}
}

func TestSyncItemsExtractsResponsePath(t *testing.T) {
	items, err := syncItems([]byte(`{"listings":[{"listingId":1},{"listingId":2}]}`), "listings")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
}

func TestSyncItemsExtractPromotionCollectionsIndependently(t *testing.T) {
	fixture := []byte(`{"promotions":[{"promotion_id":42}],"revenue_stats":{"revenue":0}}`)
	promotions, err := syncItems(fixture, "promotions")
	if err != nil {
		t.Fatal(err)
	}
	revenueStats, err := syncItems(fixture, "revenue_stats")
	if err != nil {
		t.Fatal(err)
	}
	if len(promotions) != 1 || len(revenueStats) != 1 {
		t.Fatalf("unexpected extracted items: promotions=%d revenue_stats=%d", len(promotions), len(revenueStats))
	}
}

func TestEnrichObservationPersistsRequestWindow(t *testing.T) {
	observedAt := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	raw, err := enrichObservation(
		[]byte(`{"listingId":1}`),
		"listings",
		observedAt,
		"listings:1",
		map[string]string{"start_date": "2026-07-01", "end_date": "2026-07-25"},
	)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value["_request_start_date"] != "2026-07-01" || value["_request_end_date"] != "2026-07-25" {
		t.Fatalf("request window missing: %#v", value)
	}
}

func TestDashboardSyncDryRunShowsBoundedAdsPagination(t *testing.T) {
	tasks, err := dashboardSyncTasks("ads", "123", "2026-01-01", "2026-01-31", 50)
	if err != nil {
		t.Fatal(err)
	}
	result := dashboardSyncDryRun("/tmp/dashboard.db", tasks, true, 3)
	requests, ok := result["requests"].([]map[string]any)
	if !ok || len(requests) != 3 {
		t.Fatalf("unexpected requests: %#v", result["requests"])
	}
	wantOffsets := []string{"0", "50", "100"}
	for index, request := range requests {
		params := request["params"].(map[string]string)
		if params["offset"] != wantOffsets[index] {
			t.Fatalf("request %d offset = %q, want %q", index, params["offset"], wantOffsets[index])
		}
	}
}

func TestDashboardSyncDryRunFetchesCombinedPromotionsOnce(t *testing.T) {
	tasks, err := dashboardSyncTasks("promotions", "123", "2026-01-01", "2026-01-31", 50)
	if err != nil {
		t.Fatal(err)
	}
	result := dashboardSyncDryRun("/tmp/dashboard.db", tasks, false, 1)
	requests := result["requests"].([]map[string]any)
	if len(requests) != 1 || requests[0]["label"] != "combined" {
		t.Fatalf("combined endpoint must be fetched once: %#v", requests)
	}
}
