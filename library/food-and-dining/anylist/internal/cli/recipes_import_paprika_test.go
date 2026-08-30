package cli

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestPaprikaRecipeFromMapPreservesRecipeContent(t *testing.T) {
	recipe, err := paprikaRecipeFromMap(map[string]any{
		"uid":              "paprika-123",
		"name":             "Test Chicken",
		"description":      "A useful description",
		"notes":            "Keep the sauce warm",
		"ingredients":      "2 chicken thighs\n1 tbsp olive oil",
		"directions":       "Sear the chicken.\n\nFinish in the oven.",
		"nutritional_info": "Protein 30g",
		"prep_time":        "15",
		"cook_time":        "1 hr 20 min",
		"servings":         "4",
		"rating":           json.Number("5"),
		"source":           "Test Source",
		"source_url":       "https://example.test/recipe",
		"image_url":        "https://example.test/image.jpg",
	})
	if err != nil {
		t.Fatalf("paprikaRecipeFromMap returned error: %v", err)
	}
	if recipe.GetName() != "Test Chicken" || recipe.GetPaprikaIdentifier() != "paprika-123" {
		t.Fatalf("recipe identity = %#v", recipe)
	}
	if recipe.GetPrepTime() != 15*60 || recipe.GetCookTime() != 80*60 {
		t.Fatalf("times = %d/%d, want 900/4800", recipe.GetPrepTime(), recipe.GetCookTime())
	}
	if len(recipe.GetIngredients()) != 2 || recipe.GetIngredients()[0].GetRawIngredient() != "2 chicken thighs" {
		t.Fatalf("ingredients = %#v", recipe.GetIngredients())
	}
	if len(recipe.GetPreparationSteps()) != 2 || recipe.GetPreparationSteps()[1] != "Finish in the oven." {
		t.Fatalf("steps = %#v", recipe.GetPreparationSteps())
	}
	if recipe.GetNutritionalInfo() != "Protein 30g" || len(recipe.GetPhotoUrls()) != 0 {
		t.Fatalf("extended fields = %#v; Paprika image URLs must not be sent as unverified photo writes", recipe)
	}
}

func TestPaprikaNormalizeText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain text preserved", input: "2 cups flour\n1 tsp salt", want: "2 cups flour\n1 tsp salt"},
		{name: "empty string", input: "", want: ""},
		{name: "named entities", input: "Tomato &amp; Basil &quot;classic&quot; &copy;", want: "Tomato & Basil \"classic\" ©"},
		{name: "numeric entities", input: "Grate &#65; &#x42;", want: "Grate A B"},
		{name: "single pass does not double-decode", input: "&amp;amp;", want: "&amp;"},
		{name: "br markup to newlines", input: "Stir<br>simmer<br />pour<br/> bake", want: "Stir\nsimmer\npour\nbake"},
		{name: "li block markup", input: "<li>Mix</li><li>Rest</li>", want: "Mix\nRest"},
		{name: "p block markup", input: "<p>Intro</p><p>More</p>", want: "Intro\nMore"},
		{name: "simple tags stripped", input: "<b>Boil</b> the <i>pasta</i>", want: "Boil the pasta"},
		{name: "non-breaking spaces normalized", input: "2\u00a0cups&nbsp;flour", want: "2 cups flour"},
		{name: "excessive blank lines collapsed", input: "line1\n\n\n\n\nline2", want: "line1\n\nline2"},
		{name: "ordinary angle brackets and bare ampersand preserved", input: "3 < 4 & 5", want: "3 < 4 & 5"},
		{name: "surrounding whitespace trimmed", input: "  \tHearty dish\n  ", want: "Hearty dish"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := paprikaNormalizeText(test.input); got != test.want {
				t.Fatalf("paprikaNormalizeText(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestPaprikaRecipeFromMapNormalizesHTMLFields(t *testing.T) {
	recipe, err := paprikaRecipeFromMap(map[string]any{
		"uid":         "paprika-html-1",
		"name":        "HTML Recipe",
		"description": "Rich <b>tangy</b> sauce &amp; all",
		"notes":       "Simmer&nbsp;10&nbsp;min",
		"ingredients": "2&nbsp;cups flour<br>1 cup <i>sugar</i>\n1 tsp <b>salt</b>",
		"directions":  "<ol><li>Preheat <i>oven</i> &amp; stove</li><li>Roast<br>until done</li></ol>",
	})
	if err != nil {
		t.Fatalf("paprikaRecipeFromMap returned error: %v", err)
	}
	if recipe.GetNote() != "Rich tangy sauce & all\n\nSimmer 10 min" {
		t.Fatalf("note = %q, want normalized description and notes", recipe.GetNote())
	}
	wantIngredients := []string{"2 cups flour", "1 cup sugar", "1 tsp salt"}
	if len(recipe.GetIngredients()) != len(wantIngredients) {
		t.Fatalf("ingredients = %#v, want %d lines", recipe.GetIngredients(), len(wantIngredients))
	}
	for index, want := range wantIngredients {
		got := recipe.GetIngredients()[index]
		if got.GetRawIngredient() != want || got.GetName() != want {
			t.Fatalf("ingredient %d = %#v, want %q", index, got, want)
		}
	}
	wantSteps := []string{"Preheat oven & stove", "Roast", "until done"}
	if len(recipe.GetPreparationSteps()) != len(wantSteps) {
		t.Fatalf("steps = %#v, want %d steps", recipe.GetPreparationSteps(), len(wantSteps))
	}
	for index, want := range wantSteps {
		if recipe.GetPreparationSteps()[index] != want {
			t.Fatalf("step %d = %q, want %q", index, recipe.GetPreparationSteps()[index], want)
		}
	}
}

func TestPaprikaRecipeFromMapPreservesPlainMultilineFields(t *testing.T) {
	recipe, err := paprikaRecipeFromMap(map[string]any{
		"name":        "Plain Recipe",
		"description": "Line one\nLine two",
		"directions":  "Step one.\n\nStep two.\n\nStep three.",
	})
	if err != nil {
		t.Fatalf("paprikaRecipeFromMap returned error: %v", err)
	}
	if recipe.GetNote() != "Line one\nLine two" {
		t.Fatalf("note = %q, want plain multiline text preserved", recipe.GetNote())
	}
	wantSteps := []string{"Step one.", "Step two.", "Step three."}
	if len(recipe.GetPreparationSteps()) != len(wantSteps) {
		t.Fatalf("steps = %#v, want %#v", recipe.GetPreparationSteps(), wantSteps)
	}
	for index := range wantSteps {
		if recipe.GetPreparationSteps()[index] != wantSteps[index] {
			t.Fatalf("step %d = %q, want %q", index, recipe.GetPreparationSteps()[index], wantSteps[index])
		}
	}
}

func TestReadPaprikaRecipeAcceptsGzipJSON(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := io.WriteString(writer, `{"uid":"p-1","name":"Compressed Recipe","ingredients":"salt","directions":"Stir."}`); err != nil {
		t.Fatalf("write gzip: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	recipes, err := readPaprikaRecipe("test.paprikarecipe", bytes.NewReader(compressed.Bytes()))
	if err != nil {
		t.Fatalf("readPaprikaRecipe returned error: %v", err)
	}
	if len(recipes) != 1 || recipes[0].Recipe.GetName() != "Compressed Recipe" {
		t.Fatalf("recipes = %#v", recipes)
	}
}

func TestLoadPaprikaInputAcceptsArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recipes.paprika")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	archive := zip.NewWriter(file)
	entry, err := archive.Create("Recipes/Test.paprikarecipe")
	if err != nil {
		file.Close()
		t.Fatalf("Create archive entry returned error: %v", err)
	}
	if _, err := io.WriteString(entry, `{"uid":"archive-1","name":"Archive Recipe","directions":"Bake."}`); err != nil {
		file.Close()
		t.Fatalf("Write archive entry returned error: %v", err)
	}
	if err := archive.Close(); err != nil {
		file.Close()
		t.Fatalf("Close archive returned error: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close file returned error: %v", err)
	}
	recipes, err := loadPaprikaInput(path)
	if err != nil {
		t.Fatalf("loadPaprikaInput returned error: %v", err)
	}
	if len(recipes) != 1 || recipes[0].Recipe.GetName() != "Archive Recipe" {
		t.Fatalf("recipes = %#v", recipes)
	}
}

func TestMergePaprikaRecipePreservesIdentifierAndUnknownFields(t *testing.T) {
	existing := &pb.PBRecipe{
		Identifier:      "anylist-id",
		Name:            "Old Name",
		Icon:            "custom-icon",
		PhotoIds:        []string{"photo-1"},
		NutritionalInfo: "old nutrition",
	}
	imported := &pb.PBRecipe{Name: "New Name", NutritionalInfo: "new nutrition"}
	merged := mergePaprikaRecipe(existing, imported)
	if merged.GetIdentifier() != "anylist-id" || merged.GetIcon() != "custom-icon" || len(merged.GetPhotoIds()) != 1 {
		t.Fatalf("preserved fields lost: %#v", merged)
	}
	if merged.GetName() != "New Name" || merged.GetNutritionalInfo() != "new nutrition" {
		t.Fatalf("imported fields not applied: %#v", merged)
	}
}

func TestPlanPaprikaImportsDeduplicatesInput(t *testing.T) {
	imports := []paprikaImportRecipe{
		{SourcePath: "one.paprikarecipe", Recipe: &pb.PBRecipe{Name: "Same Recipe", PaprikaIdentifier: "uid-1"}},
		{SourcePath: "two.paprikarecipe", Recipe: &pb.PBRecipe{Name: "Same Recipe", PaprikaIdentifier: "uid-1"}},
	}
	plans, err := planPaprikaImports(imports, nil, false)
	if err != nil {
		t.Fatalf("planPaprikaImports returned error: %v", err)
	}
	if len(plans) != 2 || plans[0].Action != "create" || plans[1].Action != "skip" || plans[1].SkipReason != "duplicate-input" {
		t.Fatalf("plans = %#v, want create then duplicate-input skip", plans)
	}
}

func TestPlanPaprikaImportsRejectsAmbiguousUpdate(t *testing.T) {
	imports := []paprikaImportRecipe{{
		SourcePath: "one.paprikarecipe",
		Recipe:     &pb.PBRecipe{Name: "Same Recipe", PaprikaIdentifier: "new-uid"},
	}}
	existing := []*pb.PBRecipe{
		{Identifier: "any-1", Name: "Same Recipe"},
		{Identifier: "any-2", Name: "Same Recipe"},
	}
	if _, err := planPaprikaImports(imports, existing, true); err == nil {
		t.Fatal("planPaprikaImports returned nil for ambiguous update")
	}
}

func TestSamePaprikaRecipeContentChecksIngredientAndStepText(t *testing.T) {
	expected := &pb.PBRecipe{
		Name:              "Recipe",
		Ingredients:       []*pb.PBIngredient{{RawIngredient: "1 cup flour", Name: "1 cup flour"}},
		PreparationSteps:  []string{"Mix."},
		PaprikaIdentifier: "uid-1",
	}
	actual := &pb.PBRecipe{
		Name:              "Recipe",
		Ingredients:       []*pb.PBIngredient{{RawIngredient: "2 cups flour", Name: "2 cups flour"}},
		PreparationSteps:  []string{"Mix."},
		PaprikaIdentifier: "uid-1",
	}
	if samePaprikaRecipeContent(actual, expected) {
		t.Fatal("samePaprikaRecipeContent accepted mismatched ingredient text")
	}
	actual.Ingredients[0] = expected.Ingredients[0]
	actual.PreparationSteps[0] = "Bake."
	if samePaprikaRecipeContent(actual, expected) {
		t.Fatal("samePaprikaRecipeContent accepted mismatched step text")
	}
}
