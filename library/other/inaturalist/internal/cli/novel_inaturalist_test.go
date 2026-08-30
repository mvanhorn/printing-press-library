package cli

import (
	"testing"
	"time"
)

func TestSpeciesCountsUsesTaxonAndCount(t *testing.T) {
	counts := speciesCounts(map[string]any{"results": []any{map[string]any{"count": float64(4), "taxon": map[string]any{"id": float64(3), "name": "Aves", "preferred_common_name": "Birds", "iconic_taxon_name": "Aves"}}}})
	if len(counts) != 1 || counts[0].ID != "3" || counts[0].Name != "Birds" || counts[0].Count != 4 {
		t.Fatalf("speciesCounts() = %#v", counts)
	}
}

func TestDateSinceRejectsUnboundedWindow(t *testing.T) {
	if _, err := dateSince("0d", nowForTest()); err == nil {
		t.Fatal("dateSince accepted zero days")
	}
}

func nowForTest() time.Time { return time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC) }
