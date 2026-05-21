package jsonld

import (
	"strings"
	"testing"
)

func TestExtractFromHTML_FoodstuffDataset(t *testing.T) {
	html := []byte(`<html><head>
<script type="application/ld+json">
{"@context":"https://schema.org/","@type":"Dataset","name":"jablko","description":"Czech apple nutrition","url":"https://kaloricketabulky.cz/potraviny/jablko","keywords":["Energetická hodnota : 62,7 kJ","Bílkoviny : 0,37 g","Tuky : 0,4 g","Sacharidy : 12,95 g","Vláknina : 3,14 g","Vápník : 8,27 mg","Nasycené mastné kyseliny : 0,23 g","Cukry : 10,98 g"]}
</script></head><body></body></html>`)
	d, _, err := ExtractFromHTML(html)
	if err != nil {
		t.Fatalf("ExtractFromHTML: %v", err)
	}
	if d.Title != "jablko" {
		t.Errorf("Title=%q, want jablko", d.Title)
	}
	if d.Nutrition.EnergyKJ != 62.7 {
		t.Errorf("EnergyKJ=%v, want 62.7", d.Nutrition.EnergyKJ)
	}
	if d.Nutrition.ProteinG != 0.37 {
		t.Errorf("ProteinG=%v, want 0.37", d.Nutrition.ProteinG)
	}
	if d.Nutrition.FatG != 0.4 {
		t.Errorf("FatG=%v, want 0.4", d.Nutrition.FatG)
	}
	if d.Nutrition.CarbG != 12.95 {
		t.Errorf("CarbG=%v, want 12.95", d.Nutrition.CarbG)
	}
	if d.Nutrition.FiberG != 3.14 {
		t.Errorf("FiberG=%v, want 3.14", d.Nutrition.FiberG)
	}
	if d.Nutrition.SaturatedFatG != 0.23 {
		t.Errorf("SaturatedFatG=%v, want 0.23", d.Nutrition.SaturatedFatG)
	}
	if d.Nutrition.SugarsG != 10.98 {
		t.Errorf("SugarsG=%v, want 10.98", d.Nutrition.SugarsG)
	}
}

func TestExtractAllergens(t *testing.T) {
	n := Nutrition{Raw: []string{"lepek", "obsahuje sóju", "vejce z volného chovu", "Vápník : 100 mg"}}
	got := ExtractAllergens(n)
	want := map[string]bool{"gluten": true, "soy": true, "egg": true}
	if len(got) != len(want) {
		t.Fatalf("got %v allergens, want %v", got, want)
	}
	for _, a := range got {
		if !want[a] {
			t.Errorf("unexpected allergen %q", a)
		}
	}
}

func TestParseCzechNutritionKeywords_HandlesCzechDecimals(t *testing.T) {
	kws := []string{"Energetická hodnota : 350,5 kJ", "Bílkoviny : 4,2 g"}
	n := parseCzechNutritionKeywords(kws)
	if n.EnergyKJ != 350.5 {
		t.Errorf("EnergyKJ=%v, want 350.5", n.EnergyKJ)
	}
	if n.ProteinG != 4.2 {
		t.Errorf("ProteinG=%v, want 4.2", n.ProteinG)
	}
}

func TestExtractFromHTML_NoJSONLD(t *testing.T) {
	html := []byte(`<html><head></head><body>nothing here</body></html>`)
	_, _, err := ExtractFromHTML(html)
	if err == nil {
		t.Fatalf("expected error for HTML with no JSON-LD")
	}
	if !strings.Contains(err.Error(), "no application/ld+json") {
		t.Errorf("unexpected error: %v", err)
	}
}
