package cli

import "testing"

func TestScoreKeywordRatesUsefulKeyword(t *testing.T) {
	tests := []struct {
		name    string
		signals keywordSignals
		want    string
	}{
		{
			name: "strong keyword with tag evidence",
			signals: keywordSignals{
				Keyword: "dad mug",
				Stats: map[string]any{
					"avg_searches": 2500.0,
					"competition":  1000.0,
					"difficulty":   10.0,
				},
				TopListings: []map[string]any{
					{"title": "funny dad mug", "tags": []any{"dad gift", "father mug"}},
					{"title": "best dad cup", "tags": []any{"dad gift", "coffee mug"}},
				},
			},
			want: "strong",
		},
		{
			name: "crowded keyword",
			signals: keywordSignals{
				Keyword: "dad mug",
				Stats: map[string]any{
					"avg_searches": 100.0,
					"competition":  500000.0,
					"difficulty":   90.0,
				},
			},
			want: "crowded",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoreKeyword(tt.signals)
			if got.Rating != tt.want {
				t.Fatalf("rating = %q, want %q (score %.1f)", got.Rating, tt.want, got.Score)
			}
		})
	}
}

func TestScoreKeywordDifficultyIgnoresUnrelatedScoreFields(t *testing.T) {
	got := scoreKeyword(keywordSignals{
		Keyword: "dad mug",
		Stats: map[string]any{
			"avg_searches": 100.0,
			"competition":  100.0,
			"difficulty":   12.0,
			"search_score": 900.0,
		},
	})

	if got.DifficultySignal != 12 {
		t.Fatalf("DifficultySignal = %.1f, want 12.0", got.DifficultySignal)
	}
}

func TestRankConsensusTagsMergesSources(t *testing.T) {
	signals := keywordSignals{
		TopListings: []map[string]any{{"title": "funny dad mug", "tags": []any{"dad gift", "coffee mug"}}},
		EtsyTags:    []map[string]any{{"tag": "dad gift"}, {"tag": "father mug"}},
		Related:     []map[string]any{{"keyword": "dad gift"}},
		NearMatches: []map[string]any{{"phrase": "coffee mug"}},
	}
	got := rankConsensusTags(signals, 2)
	if len(got) == 0 {
		t.Fatal("expected consensus tags")
	}
	if got[0].Tag != "dad gift" {
		t.Fatalf("top tag = %q, want dad gift", got[0].Tag)
	}
	if got[0].Count < 3 {
		t.Fatalf("dad gift count = %d, want at least 3", got[0].Count)
	}
}

func TestRankConsensusTagsCountsEachRowOnce(t *testing.T) {
	signals := keywordSignals{
		TopListings: []map[string]any{
			{"title": "dad gift", "tags": []any{"dad gift", "dad gift"}},
		},
	}

	got := rankConsensusTags(signals, 1)
	if len(got) != 1 {
		t.Fatalf("rankConsensusTags() = %#v, want one tag", got)
	}
	if got[0].Tag != "dad gift" || got[0].Count != 1 {
		t.Fatalf("top tag = %#v, want dad gift counted once", got[0])
	}
}

func TestDriftSummaryFlagsThreshold(t *testing.T) {
	current := scoredKeyword{Keyword: "dad mug", Score: 80}
	history := []scoredKeywordSnapshot{
		{scoredKeyword: scoredKeyword{Keyword: "dad mug", Score: 55}},
		{scoredKeyword: current},
	}
	got := driftSummary(history, current, 15)
	if got["status"] != "drift" {
		t.Fatalf("status = %v, want drift", got["status"])
	}
}

func TestDriftSummaryScopesHistoryBySourceAndCountry(t *testing.T) {
	current := scoredKeyword{Keyword: "dad mug", Source: "etsy", Country: "GBR", Score: 80}
	history := []scoredKeywordSnapshot{
		{scoredKeyword: scoredKeyword{Keyword: "dad mug", Source: "etsy", Country: "USA", Score: 20}},
		{scoredKeyword: scoredKeyword{Keyword: "dad mug", Source: "etsy", Country: "GBR", Score: 70}},
		{scoredKeyword: current},
	}

	got := driftSummary(history, current, 15)
	if got["score_change"] != 10.0 {
		t.Fatalf("score_change = %v, want 10.0 from matching country history", got["score_change"])
	}
}

func TestBuildAnglesUsesRelatedSearchesAndTags(t *testing.T) {
	signals := keywordSignals{
		Keyword: "ceramic mug",
		Related: []map[string]any{
			{"keyword": "handmade ceramic mug"},
			{"keyword": "pottery gift"},
		},
		EtsyTags: []map[string]any{
			{"tag": "handmade ceramic mug"},
			{"tag": "pottery gift"},
		},
	}
	got := buildAngles(signals, 2)
	if len(got) != 2 {
		t.Fatalf("buildAngles() = %#v", got)
	}
	angle, ok := got[0]["angle"].(string)
	if !ok || angle != "Handmade Ceramic Mug" {
		t.Fatalf("first angle = %#v, want Handmade Ceramic Mug", got[0]["angle"])
	}
}

func TestNormalizeTokenSetDeduplicatesTerms(t *testing.T) {
	got := normalizeTokenSet([]string{"Dad-Mug", "dad mug", " dad_mug ", "", "Coffee/Cup"})
	if len(got) != 2 {
		t.Fatalf("normalizeTokenSet() = %#v, want 2 terms", got)
	}
	if !got["dad mug"] || !got["coffee cup"] {
		t.Fatalf("normalizeTokenSet() = %#v, want normalized terms", got)
	}
}
