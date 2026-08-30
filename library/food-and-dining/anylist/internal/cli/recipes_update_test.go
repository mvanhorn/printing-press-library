package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestRecipeUpdateApplyPreservesOmittedFields(t *testing.T) {
	t.Parallel()

	current := &pb.PBRecipe{
		Identifier:        "recipe-1",
		Timestamp:         11,
		CreationTimestamp: 7,
		Name:              "Original",
		Icon:              "icon",
		SourceName:        "Paprika",
		SourceUrl:         "https://example.test/original",
		Note:              "old note",
		NutritionalInfo:   "old nutrition",
		Servings:          "4",
		PrepTime:          600,
		CookTime:          1200,
		Rating:            4,
		PhotoIds:          []string{"photo-1"},
		PaprikaIdentifier: "paprika-1",
		Ingredients:       []*pb.PBIngredient{{RawIngredient: "1 cup flour"}},
		PreparationSteps:  []string{"Mix", "Bake"},
	}
	newNote := "cleaned note"
	newNutrition := "30g protein"
	newPrep := 15
	updated, err := (recipeUpdateInput{
		name:            nil,
		note:            &newNote,
		nutritionalInfo: &newNutrition,
		prepTimeMinutes: &newPrep,
	}).apply(current)
	if err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	if updated.GetNote() != newNote || updated.GetNutritionalInfo() != newNutrition || updated.GetPrepTime() != 900 {
		t.Fatalf("updated fields = note %q, nutrition %q, prep %d; want cleaned note/30g protein/900", updated.GetNote(), updated.GetNutritionalInfo(), updated.GetPrepTime())
	}
	if updated.GetName() != current.GetName() || updated.GetSourceUrl() != current.GetSourceUrl() ||
		updated.GetServings() != current.GetServings() || updated.GetCookTime() != current.GetCookTime() ||
		updated.GetRating() != current.GetRating() || updated.GetIcon() != current.GetIcon() ||
		updated.GetPaprikaIdentifier() != current.GetPaprikaIdentifier() ||
		updated.GetTimestamp() != current.GetTimestamp() || updated.GetCreationTimestamp() != current.GetCreationTimestamp() {
		t.Fatalf("omitted fields were changed: %#v", updated)
	}
	if !recipeIngredientsEqual(updated.GetIngredients(), current.GetIngredients()) {
		t.Fatal("omitted ingredients were changed")
	}
	if len(updated.GetPreparationSteps()) != len(current.GetPreparationSteps()) {
		t.Fatal("omitted preparation steps were changed")
	}
}

func TestRecipeUpdateInputParsesPaprikaCleanupArrays(t *testing.T) {
	t.Parallel()

	input, err := recipeUpdateInputFromBody(map[string]any{
		"name":             "Imported Recipe",
		"new_name":         "Clean Recipe",
		"nutritional_info": "30g protein",
		"ingredients": []any{
			map[string]any{"raw_ingredient": "1 cup flour", "name": "flour", "quantity": "1 cup"},
			"2 eggs",
		},
		"preparation_steps": []any{"Mix\nwell", "Bake"},
	})
	if err != nil {
		t.Fatalf("recipeUpdateInputFromBody returned error: %v", err)
	}
	if input.selector != "Imported Recipe" || input.name == nil || *input.name != "Clean Recipe" {
		t.Fatalf("parsed selector/name = %q/%v, want Imported Recipe/Clean Recipe", input.selector, input.name)
	}
	if input.nutritionalInfo == nil || *input.nutritionalInfo != "30g protein" {
		t.Fatalf("parsed nutritional info = %v, want 30g protein", input.nutritionalInfo)
	}
	if input.ingredients == nil || len(*input.ingredients) != 2 || (*input.ingredients)[0].GetRawIngredient() != "1 cup flour" || (*input.ingredients)[1].GetRawIngredient() != "2 eggs" {
		t.Fatalf("parsed ingredients = %#v", input.ingredients)
	}
	if input.preparationSteps == nil || len(*input.preparationSteps) != 2 || (*input.preparationSteps)[0] != "Mix\nwell" {
		t.Fatalf("parsed preparation steps = %#v", input.preparationSteps)
	}

	rename, err := recipeUpdateInputFromBody(map[string]any{
		"recipe": "Imported Recipe",
		"name":   "Clean Recipe",
	})
	if err != nil {
		t.Fatalf("recipeUpdateInputFromBody rename returned error: %v", err)
	}
	if rename.selector != "Imported Recipe" || rename.name == nil || *rename.name != "Clean Recipe" {
		t.Fatalf("parsed rename selector/name = %q/%v", rename.selector, rename.name)
	}
}

func TestVerifyRecipeUpdateChecksExplicitFieldsOnly(t *testing.T) {
	t.Parallel()

	expected := &pb.PBRecipe{
		Identifier:       "recipe-1",
		Name:             "Clean Recipe",
		Note:             "updated",
		NutritionalInfo:  "updated nutrition",
		Ingredients:      []*pb.PBIngredient{{RawIngredient: "1 cup flour"}},
		PreparationSteps: []string{"Mix", "Bake"},
	}
	name := "Clean Recipe"
	input := recipeUpdateInput{name: &name}
	readBack := &pb.PBRecipe{
		Identifier: "recipe-1",
		Name:       "Clean Recipe",
		Note:       "server-normalized note",
	}
	if err := verifyRecipeUpdate(readBack, expected, input); err != nil {
		t.Fatalf("verifyRecipeUpdate returned error for omitted note: %v", err)
	}

	input.note = stringPtrForRecipeTest("updated")
	if err := verifyRecipeUpdate(readBack, expected, input); err == nil {
		t.Fatal("verifyRecipeUpdate accepted a mismatched explicit note")
	}

	input.note = nil
	input.nutritionalInfo = stringPtrForRecipeTest("updated nutrition")
	if err := verifyRecipeUpdate(readBack, expected, input); err == nil {
		t.Fatal("verifyRecipeUpdate accepted mismatched explicit nutritional info")
	}
}

func stringPtrForRecipeTest(value string) *string {
	return &value
}

func TestRecipesUpdateDryRunRemainsStructuredAndOffline(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"recipes", "update", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	var payload struct {
		DryRun bool   `json:"dry_run"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, output.String())
	}
	if !payload.DryRun || payload.Status != "preview" {
		t.Fatalf("payload = %#v, want dry-run preview", payload)
	}
}

func TestRecipesUpdateDefaultsToPreviewWithoutAuth(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{}
	cmd := newRootCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"recipes", "update", "--name", "Existing Recipe", "--note", "preview only", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("default preview returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, output.String())
	}
	if payload["dry_run"] != true || payload["apply"] != false || payload["recipe"] != "Existing Recipe" {
		t.Fatalf("payload = %#v, want offline default preview", payload)
	}
}

func TestRecipesUpdateApplyRequiresSelectorBeforeLiveWork(t *testing.T) {
	t.Parallel()

	cmd := newRootCmd(&rootFlags{})
	cmd.SetArgs([]string{"recipes", "update", "--apply", "--note", "live write"})
	if err := cmd.Execute(); err == nil || err.Error() != `required flag "name" not set` {
		t.Fatalf("error = %v, want required selector error before auth/network", err)
	}
}

func TestRecipeUpdateStdinApplyGate(t *testing.T) {
	t.Parallel()

	input, err := recipeUpdateInputFromBody(map[string]any{
		"recipe": "Existing Recipe",
		"note":   "preview",
		"apply":  false,
	})
	if err != nil {
		t.Fatalf("preview stdin body returned error: %v", err)
	}
	if input.applyRequested {
		t.Fatal("stdin apply=false enabled live write")
	}

	input, err = recipeUpdateInputFromBody(map[string]any{
		"recipe": "Existing Recipe",
		"note":   "live",
		"apply":  true,
	})
	if err != nil {
		t.Fatalf("apply stdin body returned error: %v", err)
	}
	if !input.applyRequested || !input.hasChanges() {
		t.Fatalf("stdin apply gate = %#v, want apply=true with changes", input)
	}

	if _, err := recipeUpdateInputFromBody(map[string]any{"apply": "true"}); err == nil {
		t.Fatal("non-boolean stdin apply was accepted")
	}
}
