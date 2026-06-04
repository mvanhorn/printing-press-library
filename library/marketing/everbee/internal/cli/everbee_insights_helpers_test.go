package cli

import (
	"strings"
	"testing"
)

func TestNormalizeTokens(t *testing.T) {
	got := normalizeTokens("Teacher Gift, teacher-gift! A opportunity shortlist")
	want := []string{"teacher", "gift"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestMatchesTokensRequiresAllMeaningfulTerms(t *testing.T) {
	tokens := normalizeTokens("dark humor dad mug opportunity shortlist")
	if matchesTokens("funny dad coffee mug", tokens) {
		t.Fatal("partial token match returned true; want exact niche miss")
	}
	if !matchesTokens("dark humor dad coffee mug", tokens) {
		t.Fatal("all query tokens present returned false")
	}
}

func TestSearchableRecordTextUsesValuesNotJSONKeys(t *testing.T) {
	text := searchableRecordText(map[string]any{
		"category": "coffee mugs",
		"title":    "funny dad mug",
		"tags":     []any{"father gift", "ceramic cup"},
	})
	if strings.Contains(text, "category") || strings.Contains(text, "title") {
		t.Fatalf("searchable text includes JSON keys: %q", text)
	}
	if !strings.Contains(text, "funny dad mug") || !strings.Contains(text, "father gift") {
		t.Fatalf("searchable text missing values: %q", text)
	}
}

func TestScoreRecordUsesTextAndNumericSignals(t *testing.T) {
	score, reasons := scoreRecord(
		map[string]any{
			"estimated_sales":   float64(100),
			"monthly_revenue":   "$1,200",
			"competition_score": float64(42),
		},
		"teacher gift printable",
		[]string{"teacher", "gift"},
	)
	if score <= 20 {
		t.Fatalf("score = %v, want text and numeric boost", score)
	}
	if len(reasons) < 3 {
		t.Fatalf("reasons = %v, want text and numeric reasons", reasons)
	}
}

func TestClusterRecordsGroupsTerms(t *testing.T) {
	records := []everbeeRecord{
		{ID: "1", Text: "wedding sign svg printable"},
		{ID: "2", Text: "wedding welcome sign template"},
	}
	clusters := clusterRecords(records)
	if len(clusters) == 0 {
		t.Fatal("expected clusters")
	}
	if clusters[0]["term"] != "wedding" {
		t.Fatalf("top cluster = %v, want wedding", clusters[0])
	}
}
