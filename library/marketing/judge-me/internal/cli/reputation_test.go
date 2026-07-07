package cli

import (
	"encoding/json"
	"testing"
)

func rawReview(t *testing.T, obj map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestProductQualityStatsFromReviews(t *testing.T) {
	items := []json.RawMessage{
		rawReview(t, map[string]any{"id": 1, "rating": 1, "product_external_id": "sku-1", "product_title": "Widget", "product_handle": "widget", "verified": "verified-purchase", "created_at": "2026-06-10T00:00:00Z"}),
		rawReview(t, map[string]any{"id": 2, "rating": 5, "product_external_id": "sku-1", "product_title": "Widget", "product_handle": "widget", "verified": "", "created_at": "2026-06-11T00:00:00Z"}),
		rawReview(t, map[string]any{"id": 3, "rating": 2, "product_external_id": "sku-2", "product_title": "Gadget", "product_handle": "gadget", "verified": "buyer", "created_at": "2026-06-09T00:00:00Z"}),
	}
	stats := productQualityStatsFromReviews(items, 1)
	if len(stats) != 2 {
		t.Fatalf("expected 2 product groups, got %d", len(stats))
	}
	if stats[0].ProductKey != "sku-2" || stats[0].LowRatingRate != 100 {
		t.Fatalf("expected sku-2 first with 100%% low-rating rate, got %+v", stats[0])
	}
	if stats[1].AverageRating != 3 || stats[1].LowRatingRate != 50 || stats[1].VerifiedRate != 50 {
		t.Fatalf("unexpected sku-1 stats: %+v", stats[1])
	}
}

func TestModerationCandidatesFromReviews(t *testing.T) {
	items := []json.RawMessage{
		rawReview(t, map[string]any{"id": "low", "rating": 2, "curated": "ok", "title": "Bad", "hidden": false}),
		rawReview(t, map[string]any{"id": "new", "rating": 5, "curated": "not-yet", "title": "Pending"}),
		rawReview(t, map[string]any{"id": "clean", "rating": 5, "curated": "ok", "title": "Good"}),
	}
	candidates := moderationCandidatesFromReviews(items, 2)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %+v", len(candidates), candidates)
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		seen[c.ID] = true
	}
	if !seen["low"] || !seen["new"] || seen["clean"] {
		t.Fatalf("unexpected candidate set: %+v", candidates)
	}
}

func TestReviewStatsFromNestedReviewsEnvelope(t *testing.T) {
	envelope := rawReview(t, map[string]any{"reviews": []map[string]any{
		{"id": 1, "rating": 4, "verified": "buyer"},
		{"id": 2, "rating": 2, "verified": ""},
	}})
	stats := reviewStatsFromRaw([]json.RawMessage{envelope})
	if stats.Count != 2 || stats.AverageRating != 3 || stats.LowRatingCount != 1 || stats.VerifiedCount != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestIsVerifiedUsesExactJudgeMeStatuses(t *testing.T) {
	for _, status := range []string{"confirmed-buyer", "buyer", "verified-purchase", "semi-verified-purchase", "admin"} {
		if !isVerified(status) {
			t.Fatalf("expected %q to be verified", status)
		}
	}
	for _, status := range []string{"", "not-verified", "unverified", "non-buyer", "guest"} {
		if isVerified(status) {
			t.Fatalf("expected %q not to be verified", status)
		}
	}
}
