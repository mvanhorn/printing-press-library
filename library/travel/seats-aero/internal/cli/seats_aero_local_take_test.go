package cli

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/store"
)

func TestSeatsAeroLocalAvailabilityHonorsTake(t *testing.T) {
	isolateNovelTest(t)
	path := defaultDBPath("seats-aero-pp-cli")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "three"} {
		raw, _ := json.Marshal(map[string]any{"ID": id, "Source": "aeroplan", "JAvailable": true})
		if err := db.UpsertAvailability(raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	out, _, err := executeRoot("availability", "--source", "aeroplan", "--cabin", "business", "--take", "1", "--data-source", "local", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Results []json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatalf("decode %q: %v", out.String(), err)
	}
	if len(envelope.Results) != 1 {
		t.Fatalf("results=%d, want 1: %s", len(envelope.Results), out.String())
	}
}
