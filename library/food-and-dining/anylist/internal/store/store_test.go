package store

import (
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
)

func TestGetListsByStoreMatchesStoreIDsExactly(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	if _, err := st.db.Exec(`INSERT INTO lists (id, name) VALUES ('list-1', 'Groceries')`); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO stores (id, list_id, name, sort_index) VALUES
		('abc', 'list-1', 'Short ID Store', 1),
		('xyzabc123', 'list-1', 'Exact Store', 2)`); err != nil {
		t.Fatalf("insert stores: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO items
		(id, list_id, name, checked, manual_sort_index, store_ids)
		VALUES ('item-1', 'list-1', 'Milk', 0, 1, '["xyzabc123"]')`); err != nil {
		t.Fatalf("insert item: %v", err)
	}

	groups, err := st.GetListsByStore("list-1")
	if err != nil {
		t.Fatalf("GetListsByStore returned error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1: %#v", len(groups), groups)
	}
	if groups[0].StoreName != "Exact Store" {
		t.Fatalf("StoreName = %q, want %q", groups[0].StoreName, "Exact Store")
	}
	if len(groups[0].Items) != 1 || groups[0].Items[0].ID != "item-1" {
		t.Fatalf("items = %#v, want only item-1", groups[0].Items)
	}
}

func TestGetMissingIngredientsEscapesLikeWildcards(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	if _, err := st.db.Exec(`INSERT INTO lists (id, name) VALUES ('list-1', 'Groceries')`); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO recipes (id, name) VALUES ('recipe-1', 'Pancakes')`); err != nil {
		t.Fatalf("insert recipe: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO ingredients
		(id, recipe_id, raw_ingredient, name, sort_index)
		VALUES
		('ingredient-1', 'recipe-1', '1% milk', '1% milk', 1),
		('ingredient-2', 'recipe-1', 'a_b spice', 'a_b spice', 2)`); err != nil {
		t.Fatalf("insert ingredients: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO items
		(id, list_id, name, checked, manual_sort_index, store_ids)
		VALUES
		('item-1', 'list-1', '1 gallon milk', 0, 1, '[]'),
		('item-2', 'list-1', 'acb spice', 0, 2, '[]')`); err != nil {
		t.Fatalf("insert items: %v", err)
	}

	missing, err := st.GetMissingIngredients("recipe-1", "list-1")
	if err != nil {
		t.Fatalf("GetMissingIngredients returned error: %v", err)
	}
	if len(missing) != 2 {
		t.Fatalf("len(missing) = %d, want 2: %#v", len(missing), missing)
	}
	if missing[0].ID != "ingredient-1" || missing[1].ID != "ingredient-2" {
		t.Fatalf("missing = %#v, want both wildcard ingredients", missing)
	}
}

func TestFindRecipeByID(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	if _, err := st.db.Exec(`INSERT INTO recipes (id, name) VALUES ('recipe-1', 'Pancakes')`); err != nil {
		t.Fatalf("insert recipe: %v", err)
	}

	recipe, err := st.FindRecipeByID("recipe-1")
	if err != nil {
		t.Fatalf("FindRecipeByID returned error: %v", err)
	}
	if recipe.Name != "Pancakes" {
		t.Fatalf("Name = %q, want Pancakes", recipe.Name)
	}
	if _, err := st.FindRecipeByID("missing"); err == nil {
		t.Fatal("FindRecipeByID missing id returned nil error")
	}
}

func TestSyncFromUserDataClearsStaleMealCalendarRows(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	firstSync := &pb.PBUserDataResponse{
		MealPlanningCalendarResponse: &pb.PBCalendarResponse{
			Events: []*pb.PBCalendarEvent{
				{
					Identifier:          "event-1",
					CalendarId:          "calendar-1",
					Date:                "2026-05-18",
					Title:               "Dinner",
					LabelId:             "label-1",
					OrderAddedSortIndex: 7,
				},
			},
			Labels: []*pb.PBCalendarLabel{
				{
					Identifier: "label-1",
					CalendarId: "calendar-1",
					Name:       "Dinner",
					HexColor:   "#ff0000",
					SortIndex:  3,
				},
			},
		},
	}
	if err := st.SyncFromUserData(firstSync); err != nil {
		t.Fatalf("first SyncFromUserData returned error: %v", err)
	}

	events, err := st.GetMealEvents("2026-05-18", "2026-05-18")
	if err != nil {
		t.Fatalf("GetMealEvents after first sync returned error: %v", err)
	}
	if len(events) != 1 || events[0].ID != "event-1" {
		t.Fatalf("events after first sync = %#v, want event-1", events)
	}
	labels, err := st.GetCalendarLabels()
	if err != nil {
		t.Fatalf("GetCalendarLabels after first sync returned error: %v", err)
	}
	if len(labels) != 1 || labels[0].ID != "label-1" {
		t.Fatalf("labels after first sync = %#v, want label-1", labels)
	}

	// PATCH: A later full calendar payload with no events/labels must remove stale cache rows.
	if err := st.SyncFromUserData(&pb.PBUserDataResponse{
		MealPlanningCalendarResponse: &pb.PBCalendarResponse{},
	}); err != nil {
		t.Fatalf("second SyncFromUserData returned error: %v", err)
	}

	events, err = st.GetMealEvents("2026-05-18", "2026-05-18")
	if err != nil {
		t.Fatalf("GetMealEvents after second sync returned error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events after second sync = %#v, want none", events)
	}
	labels, err = st.GetCalendarLabels()
	if err != nil {
		t.Fatalf("GetCalendarLabels after second sync returned error: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("labels after second sync = %#v, want none", labels)
	}
}
