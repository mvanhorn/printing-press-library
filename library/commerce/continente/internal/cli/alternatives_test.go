package cli

import "testing"

func TestBuildAlternatives_RanksComparableCandidates(t *testing.T) {
	t.Parallel()

	seed := productResponse{
		ID:         "seed",
		Name:       "Leite UHT Meio Gordo Mimosa",
		Brand:      "Mimosa",
		Category:   "Leite/Meio Gordo",
		Categories: []string{"Leite", "Meio Gordo"},
		UnitLabel:  "lt",
		UnitPrice:  1.3,
		PackLabel:  "1 x 1 lt",
	}
	items := []storefrontItem{
		{ID: "seed", Name: "Leite UHT Meio Gordo Mimosa", Category: "Leite/Meio Gordo", Categories: []string{"Leite", "Meio Gordo"}, UnitLabel: "lt", Price: 1, PackLabel: "1 x 1 lt"},
		{ID: "best", Name: "Leite UHT Meio Gordo Marca X", Category: "Leite/Meio Gordo", Categories: []string{"Leite", "Meio Gordo"}, UnitLabel: "lt", Price: 1.1, UnitPrice: 1.1, PackLabel: "1 x 1 lt", SavingsPercent: 20, HasDiscount: true},
		{ID: "other", Name: "Iogurte Natural", Category: "Iogurtes", Categories: []string{"Iogurtes"}, UnitLabel: "kg", Price: 0.9},
	}

	got := buildAlternatives(seed, items, "savings-percent", 5)

	if len(got) != 1 {
		t.Fatalf("alternatives len = %d, want 1 (%+v)", len(got), got)
	}
	if got[0].ID != "best" {
		t.Fatalf("top alternative = %q, want best", got[0].ID)
	}
	if got[0].SimilarityScore <= 0 {
		t.Fatalf("similarity score = %v, want > 0", got[0].SimilarityScore)
	}
	if !got[0].BetterDeal {
		t.Fatalf("expected better deal candidate: %+v", got[0])
	}
	if got[0].UnitPriceDelta >= 0 {
		t.Fatalf("unit price delta = %v, want negative", got[0].UnitPriceDelta)
	}
	if !got[0].SamePack {
		t.Fatalf("expected same pack comparison: %+v", got[0])
	}
	if len(got[0].ComparisonSummary) == 0 {
		t.Fatalf("expected comparison summary, got none")
	}
}

func TestAlternativeTokens_RemovesStopwordsAndShortTokens(t *testing.T) {
	t.Parallel()

	got := alternativeTokens("Leite UHT Meio Gordo de Mimosa", "Mimosa")
	for _, token := range got {
		if token == "de" {
			t.Fatalf("unexpected token set: %v", got)
		}
	}
}

func TestIsCommercialPromotionLabel_FiltersNonPromoBadges(t *testing.T) {
	t.Parallel()

	if isCommercialPromotionLabel("PVP Recomendado: 1,00€/un") {
		t.Fatal("expected PVPR label to be filtered")
	}
	if isCommercialPromotionLabel("Produzido em Portugal") {
		t.Fatal("expected origin badge to be filtered")
	}
	if !isCommercialPromotionLabel("Exclusivo Online") {
		t.Fatal("expected Exclusivo Online to remain")
	}
}
