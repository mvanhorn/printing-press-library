package ocr

import (
	"strings"
	"testing"
	"time"
)

func TestNewObservationHashesImageAndCanonicalEvidence(t *testing.T) {
	capturedAt := time.Date(2026, 9, 2, 12, 0, 0, 123, time.FixedZone("offset", -7*60*60))
	observation, err := NewObservation([]byte("snapshot"), capturedAt, "tesseract", 100, 50, []Region{{Text: "Advanced", Confidence: 95.2, Box: [4]int{10, 5, 40, 10}, Pixel: [2]int{30, 10}}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(observation.ID, "sha256:") {
		t.Fatalf("ID = %q, want sha256 prefix", observation.ID)
	}
	if observation.ImageSHA256 != "16a0eeb0791b6c92451fd284dd9f599e0a7dbe7f6ebea6e2d2d06c7f74aec112" {
		t.Fatalf("image hash = %q", observation.ImageSHA256)
	}
	if observation.CapturedAt.Location() != time.UTC || observation.CapturedAt.Nanosecond() != 123 {
		t.Fatalf("captured at = %s, want UTC with nanoseconds", observation.CapturedAt)
	}
}

func TestObservationStoreExpiresEntriesAndDefensivelyCopies(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	store := NewObservationStore(60*time.Second, 2, func() time.Time { return now })
	observation, err := NewObservation([]byte("snapshot"), now, "test", 100, 50, []Region{{Text: "Original", Confidence: 90, Box: [4]int{1, 2, 3, 4}, Pixel: [2]int{2, 4}}})
	if err != nil {
		t.Fatal(err)
	}
	store.Put(observation)

	observation.OCR.Regions[0].Text = "mutated after put"
	got, ok := store.Get(observation.ID)
	if !ok || got.OCR.Regions[0].Text != "Original" {
		t.Fatalf("stored observation = %#v, ok=%v", got, ok)
	}
	got.OCR.Regions[0].Text = "mutated after get"
	got, ok = store.Get(observation.ID)
	if !ok || got.OCR.Regions[0].Text != "Original" {
		t.Fatalf("returned observation was not copied: %#v", got)
	}

	now = now.Add(61 * time.Second)
	if _, ok := store.Get(observation.ID); ok {
		t.Fatal("expired observation returned")
	}
}

func TestObservationStoreEvictsOldestAtCapacity(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	store := NewObservationStore(time.Minute, 2, func() time.Time { return now })
	first, _ := NewObservation([]byte("one"), now, "test", 1, 1, nil)
	store.Put(first)
	now = now.Add(time.Nanosecond)
	second, _ := NewObservation([]byte("two"), now, "test", 1, 1, nil)
	store.Put(second)
	now = now.Add(time.Nanosecond)
	third, _ := NewObservation([]byte("three"), now, "test", 1, 1, nil)
	store.Put(third)
	if _, ok := store.Get(first.ID); ok {
		t.Fatal("oldest observation was not evicted")
	}
	if _, ok := store.Get(second.ID); !ok {
		t.Fatal("second observation was evicted")
	}
	if _, ok := store.Get(third.ID); !ok {
		t.Fatal("third observation was not stored")
	}
}
