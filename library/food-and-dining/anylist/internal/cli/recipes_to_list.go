package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"
)

func bulkRecipeToListDisabledError() error {
	return fmt.Errorf("bulk recipe-to-list writes are disabled; use 'recipes ingredients' for raw facts, then apply explicit item operations selected by the AI")
}

func countRecipeIngredients(st *store.Store, recipe *store.RecipeRow) (int, error) {
	if recipe == nil {
		return 0, fmt.Errorf("recipe not found")
	}
	ingredients, err := st.GetIngredients(recipe.ID)
	if err != nil {
		return 0, fmt.Errorf("reading ingredients for %q: %w", recipe.Name, err)
	}
	count := 0
	for _, ing := range ingredients {
		name := ing.Name
		if name == "" {
			name = ing.RawIngredient
		}
		if name != "" {
			count++
		}
	}
	return count, nil
}
