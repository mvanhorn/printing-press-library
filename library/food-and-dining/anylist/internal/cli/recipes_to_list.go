package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"
)

func addRecipeIngredientsToList(ctx context.Context, cfg *config.Config, st *store.Store, recipeName, listName string, scale int, dedup bool) (int, error) {
	recipe, err := st.FindRecipeByName(recipeName)
	if err != nil {
		return 0, err
	}
	factor := 1.0
	if scale > 0 {
		if original := parseLeadingInt(recipe.Servings); original > 0 {
			factor = float64(scale) / float64(original)
		}
	}
	return addRecipeRowIngredientsToList(ctx, cfg, st, recipe, listName, factor, dedup)
}

func addRecipeRowIngredientsToList(ctx context.Context, cfg *config.Config, st *store.Store, recipe *store.RecipeRow, listName string, factor float64, dedup bool) (int, error) {
	if recipe == nil {
		return 0, fmt.Errorf("recipe not found")
	}
	if factor <= 0 {
		factor = 1.0
	}
	list, err := st.FindListByName(listName)
	if err != nil {
		return 0, err
	}
	ingredients, err := st.GetIngredients(recipe.ID)
	if err != nil {
		return 0, fmt.Errorf("reading ingredients for %q: %w", recipe.Name, err)
	}
	existing, err := st.GetItems(list.ID, nil)
	if err != nil {
		return 0, fmt.Errorf("reading existing list items: %w", err)
	}
	existingNames := map[string]bool{}
	for _, item := range existing {
		if !item.Checked {
			existingNames[strings.ToLower(item.Name)] = true
		}
	}

	alClient := anylist.New(cfg)
	added := 0
	for _, ing := range ingredients {
		name := ing.Name
		if name == "" {
			name = ing.RawIngredient
		}
		if name == "" {
			continue
		}
		if dedup && existingNames[strings.ToLower(name)] {
			continue
		}
		quantity := ing.Quantity
		if factor != 1.0 {
			if scaled := scaleIngredient(ing, factor); scaled["scaled"] == true {
				if q, ok := scaled["scaled_quantity"].(string); ok {
					quantity = q
				}
			}
		}
		if err := alClient.AddItem(ctx, list.ID, name, quantity, ing.Note, ""); err != nil {
			return added, fmt.Errorf("adding ingredient %q: %w", name, err)
		}
		existingNames[strings.ToLower(name)] = true
		added++
	}
	if err := syncStoreFromLive(ctx, cfg, st); err != nil {
		return added, fmt.Errorf("refreshing data after adding ingredients: %w", err)
	}
	return added, nil
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
