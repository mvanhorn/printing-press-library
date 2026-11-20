package cli

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEtsyDefaultSyncIncludesIncrementalShopReviews(t *testing.T) {
	if !slices.Contains(defaultSyncResources(), "reviews") {
		t.Fatal("default sync must include reviews")
	}
	if got := syncResourceSinceParam("reviews"); got != "min_created" {
		t.Fatalf("reviews since parameter = %q", got)
	}
	path, err := syncResourcePath("reviews")
	s2Must(t, err)
	if !strings.Contains(path, "{shop_id}") {
		t.Fatalf("reviews must be shop scoped: %s", path)
	}
}

type defaultReviewsSyncFake struct {
	sinceSyncFake
	calls []map[string]string
}

func (f *defaultReviewsSyncFake) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	copyParams := make(map[string]string, len(params))
	for key, value := range params {
		copyParams[key] = value
	}
	f.calls = append(f.calls, copyParams)
	if len(f.calls) == 1 {
		return json.RawMessage(`{"results":[{"shop_id":9000001,"listing_id":1000000001,"transaction_id":3000000001,"rating":5,"review":"Lovely linen apron","create_timestamp":1700000000}],"count":1}`), nil
	}
	return json.RawMessage(`{"results":[],"count":0}`), nil
}

func TestEtsyDefaultReviewsSyncStoresThenRequestsIncrementally(t *testing.T) {
	_, db := listingStatesFixture(t)
	f := &defaultReviewsSyncFake{}
	for _, since := range []string{"", time.Unix(1700000000, 0).UTC().Format(time.RFC3339)} {
		result := syncResource(context.Background(), f, db, "reviews", since, false, 0, false, false, nil, nil)
		s2Must(t, result.Err)
		if result.Warn != nil {
			t.Fatal(result.Warn)
		}
		count, err := db.Count("reviews")
		s2Must(t, err)
		if count != 1 {
			t.Fatalf("stored reviews = %d, want 1", count)
		}
	}
	if len(f.calls) != 2 {
		t.Fatalf("requests = %d, want 2", len(f.calls))
	}
	if got := f.calls[0]["min_created"]; got != "" {
		t.Fatalf("initial sync unexpectedly incremental: %q", got)
	}
	if got := f.calls[1]["min_created"]; got != "1700000000" {
		t.Fatalf("second sync min_created = %q, want 1700000000", got)
	}
}

func TestEtsyDefaultReviewsEmptyStoreCostsOneHeadPage(t *testing.T) {
	_, db := listingStatesFixture(t)
	defaults := defaultSyncResources()
	withoutReviews := make([]string, 0, len(defaults))
	for _, resource := range defaults {
		if resource != "reviews" {
			withoutReviews = append(withoutReviews, resource)
		}
	}
	with := estimateSyncCalls(db, defaults, nil, 0, false)
	without := estimateSyncCalls(db, withoutReviews, nil, 0, false)
	if with-without != 1 {
		t.Fatalf("reviews estimate delta = %d, want 1 (with=%d without=%d)", with-without, with, without)
	}
}
