package cli

import (
	"encoding/json"
	"testing"
)

func TestDealRatingAtLeastUsesMinimumThreshold(t *testing.T) {
	tests := []struct {
		rating  string
		minimum string
		want    bool
	}{
		{rating: "high", minimum: "high", want: true},
		{rating: "low", minimum: "high", want: false},
		{rating: "high", minimum: "low", want: true},
		{rating: "low", minimum: "low", want: true},
		{rating: "custom", minimum: "custom", want: true},
	}

	for _, tt := range tests {
		if got := dealRatingAtLeast(tt.rating, tt.minimum); got != tt.want {
			t.Errorf("dealRatingAtLeast(%q, %q): want %v, got %v", tt.rating, tt.minimum, tt.want, got)
		}
	}
}

func TestWatchRecordPersistsPreviousSnapshotForDrift(t *testing.T) {
	rec := watchRecord{
		ID: "SFO|HND|2026-12-22|business",
		PreviousSnapshot: &snapshotData{
			CapturedAt: "2026-05-13T00:00:00Z",
			Offers: []offerSummary{
				{ID: "same", PricePoints: 90000},
				{ID: "cheaper", PricePoints: 110000},
				{ID: "gone", PricePoints: 100000},
			},
		},
		LastSnapshot: snapshotData{
			CapturedAt: "2026-05-13T01:00:00Z",
			Offers: []offerSummary{
				{ID: "same", PricePoints: 90000},
				{ID: "cheaper", PricePoints: 95000},
				{ID: "new", PricePoints: 85000},
			},
		},
	}

	payload, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal watch record: %v", err)
	}
	var got watchRecord
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal watch record: %v", err)
	}
	if got.PreviousSnapshot == nil {
		t.Fatalf("previous snapshot was not persisted")
	}

	diff := computeDiff(got.PreviousSnapshot.Offers, got.LastSnapshot.Offers)
	if len(diff.New) != 1 || diff.New[0].ID != "new" {
		t.Fatalf("new offers: got %+v", diff.New)
	}
	if len(diff.Cheaper) != 1 || diff.Cheaper[0].ID != "cheaper" {
		t.Fatalf("cheaper offers: got %+v", diff.Cheaper)
	}
	if len(diff.Disappeared) != 1 || diff.Disappeared[0].ID != "gone" {
		t.Fatalf("disappeared offers: got %+v", diff.Disappeared)
	}
	if diff.Unchanged != 1 {
		t.Fatalf("unchanged: want 1, got %d", diff.Unchanged)
	}
}
