package client

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExtractSamplesPreservesMetadata(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	payload, err := json.Marshal([]any{
		[]any{"request echo"},
		[]any{
			[]any{[]any{"https://example.com/page", fetchedAt.Format(time.RFC3339), 200, 12345, 67}},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	samples := extractSamples(payload)
	if len(samples) != 1 {
		t.Fatalf("expected one sample, got %d", len(samples))
	}
	got := samples[0]
	if got.URL != "https://example.com/page" {
		t.Fatalf("unexpected URL: %q", got.URL)
	}
	if !got.FetchedAt.Equal(fetchedAt) {
		t.Fatalf("expected fetched_at %s, got %s", fetchedAt, got.FetchedAt)
	}
	if got.ResponseCode != 200 || got.SizeBytes != 12345 || got.ResponseMs != 67 {
		t.Fatalf("metadata was not preserved: %+v", got)
	}
}

func TestExtractSamplesPreservesEpochMillisTimestamp(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	payload, err := json.Marshal([]any{
		[]any{"request echo"},
		[]any{
			[]any{[]any{"https://example.com/epoch", fetchedAt.UnixMilli(), 304, 9000, 12}},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	samples := extractSamples(payload)
	if len(samples) != 1 {
		t.Fatalf("expected one sample, got %d", len(samples))
	}
	got := samples[0]
	if !got.FetchedAt.Equal(fetchedAt) || got.ResponseCode != 304 || got.SizeBytes != 9000 || got.ResponseMs != 12 {
		t.Fatalf("metadata was not preserved: %+v", got)
	}
}
