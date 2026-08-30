package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
)

func TestOpenUsesExplicitDatabasePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	databasePath := filepath.Join(root, "isolated", "cache.db")
	st, err := Open(&config.Config{
		Path:         filepath.Join(root, "config.toml"),
		DatabasePath: databasePath,
	})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if _, err := os.Stat(databasePath); err != nil {
		t.Fatalf("explicit database path was not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "anylist.db")); !os.IsNotExist(err) {
		t.Fatalf("default database path exists unexpectedly: %v", err)
	}
}

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

func TestDeleteListRemovesListScopedCacheRows(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	if _, err := st.db.Exec(`INSERT INTO lists (id, name) VALUES ('list-1', 'Temporary')`); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO items (id, list_id, name) VALUES ('item-1', 'list-1', 'Milk')`); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO stores (id, list_id, name) VALUES ('store-1', 'list-1', 'Temporary Store')`); err != nil {
		t.Fatalf("insert store: %v", err)
	}

	if err := st.DeleteList(" list-1 "); err != nil {
		t.Fatalf("DeleteList returned error: %v", err)
	}
	if _, err := st.FindListByName("Temporary"); err == nil {
		t.Fatal("deleted list is still in cache")
	}
	var count int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM items WHERE list_id = 'list-1'`).Scan(&count); err != nil {
		t.Fatalf("count deleted items: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted item count = %d, want 0", count)
	}
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM stores WHERE list_id = 'list-1'`).Scan(&count); err != nil {
		t.Fatalf("count deleted stores: %v", err)
	}
	if count != 0 {
		t.Fatalf("deleted store count = %d, want 0", count)
	}
}

func TestSyncFromUserDataPersistsCreatedList(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	if err := st.SyncFromUserData(&pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{
				Identifier: "created-list",
				Name:       "Created List",
				Creator:    "user-1",
			}},
		},
	}); err != nil {
		t.Fatalf("SyncFromUserData returned error: %v", err)
	}

	list, err := st.FindListByName("Created List")
	if err != nil {
		t.Fatalf("FindListByName returned error: %v", err)
	}
	if list.ID != "created-list" || list.Creator != "user-1" {
		t.Fatalf("cached list = %#v, want created-list/user-1", list)
	}
}

func TestSyncFromUserDataPersistsProductUpc(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	const barcode = "049000028904"
	if err := st.SyncFromUserData(&pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{
				{
					Identifier: "list-1",
					Name:       "Groceries",
					Items: []*pb.ListItem{
						{Identifier: "item-1", ListId: "list-1", Name: "Cola", ProductUpc: barcode},
					},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SyncFromUserData returned error: %v", err)
	}

	item, err := st.FindItemByID("list-1", "item-1")
	if err != nil {
		t.Fatalf("FindItemByID returned error: %v", err)
	}
	if item.ProductUpc != barcode {
		t.Fatalf("ProductUpc = %q, want %q", item.ProductUpc, barcode)
	}
}

func TestSyncFromUserDataPersistsPackageSize(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	const raw = "12 oz carton"
	if err := st.SyncFromUserData(&pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{
				Identifier: "list-1",
				Name:       "Groceries",
				Items: []*pb.ListItem{{
					Identifier: "item-1",
					ListId:     "list-1",
					Name:       "Milk",
					PackageSizePb: &pb.PBItemPackageSize{
						Size: "12", Unit: "oz", PackageType: "carton", RawPackageSize: raw,
					},
				}},
			}},
		},
	}); err != nil {
		t.Fatalf("SyncFromUserData returned error: %v", err)
	}

	item, err := st.FindItemByID("list-1", "item-1")
	if err != nil {
		t.Fatalf("FindItemByID returned error: %v", err)
	}
	if item.PackageSize == nil {
		t.Fatal("PackageSize is nil after sync")
	}
	if got := item.PackageSize.GetRawPackageSize(); got != raw {
		t.Fatalf("PackageSize raw value = %q, want %q", got, raw)
	}
	if got := item.PackageSize.GetPackageType(); got != "carton" {
		t.Fatalf("PackageSize package type = %q, want carton", got)
	}
}

func TestSyncFromUserDataPersistsPhotoIDs(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()
	if err := st.SyncFromUserData(&pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{NewLists: []*pb.ShoppingList{{
			Identifier: "list-1", Name: "Groceries", Items: []*pb.ListItem{{
				Identifier: "item-1", ListId: "list-1", Name: "Milk", PhotoIds: []string{"item-photo-1", "item-photo-2"},
			}},
		}}},
		RecipeDataResponse: &pb.PBRecipeDataResponse{Recipes: []*pb.PBRecipe{{
			Identifier: "recipe-1", Name: "Milkshake", PhotoIds: []string{"recipe-photo-1"},
		}}},
	}); err != nil {
		t.Fatalf("SyncFromUserData returned error: %v", err)
	}
	item, err := st.FindItemByID("list-1", "item-1")
	if err != nil {
		t.Fatalf("FindItemByID returned error: %v", err)
	}
	if len(item.PhotoIDs) != 2 || item.PhotoIDs[1] != "item-photo-2" {
		t.Fatalf("cached item photo IDs = %#v, want two IDs", item.PhotoIDs)
	}
	recipe, err := st.FindRecipeByID("recipe-1")
	if err != nil {
		t.Fatalf("FindRecipeByID returned error: %v", err)
	}
	if len(recipe.PhotoIDs) != 1 || recipe.PhotoIDs[0] != "recipe-photo-1" {
		t.Fatalf("cached recipe photo IDs = %#v, want recipe-photo-1", recipe.PhotoIDs)
	}
}

func TestOpenMigratesPackageSizeColumnOnExistingCache(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")
	dbPath := filepath.Join(tempDir, "anylist.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE items (
		id TEXT PRIMARY KEY,
		list_id TEXT NOT NULL,
		name TEXT NOT NULL,
		product_upc TEXT DEFAULT '',
		quantity TEXT DEFAULT '',
		details TEXT DEFAULT '',
		category TEXT DEFAULT '',
		category_match_id TEXT DEFAULT '',
		checked INTEGER NOT NULL DEFAULT 0,
		manual_sort_index INTEGER DEFAULT 0,
		store_ids TEXT DEFAULT '[]'
	)`)
	if err != nil {
		db.Close()
		t.Fatalf("create legacy items table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	st, err := Open(&config.Config{Path: configPath})
	if err != nil {
		t.Fatalf("Open returned error for legacy cache: %v", err)
	}
	defer st.Close()

	var found int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('items') WHERE name = 'package_size'`).Scan(&found); err != nil {
		t.Fatalf("check package_size column: %v", err)
	}
	if found != 1 {
		t.Fatalf("package_size column count = %d, want 1", found)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('items') WHERE name = 'prices'`).Scan(&found); err != nil {
		t.Fatalf("check prices column: %v", err)
	}
	if found != 1 {
		t.Fatalf("prices column count = %d, want 1", found)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('items') WHERE name = 'photo_ids'`).Scan(&found); err != nil {
		t.Fatalf("check item photo_ids column: %v", err)
	}
	if found != 1 {
		t.Fatalf("item photo_ids column count = %d, want 1", found)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('recipes') WHERE name = 'photo_ids'`).Scan(&found); err != nil {
		t.Fatalf("check recipe photo_ids column: %v", err)
	}
	if found != 1 {
		t.Fatalf("recipe photo_ids column count = %d, want 1", found)
	}
}

func TestSyncPersistsItemPrices(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	price := &pb.PBItemPrice{Amount: 3.49, Details: "probe", StoreId: "store-1", Date: "2026-08-16"}
	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{
		NewLists: []*pb.ShoppingList{{Identifier: "list-1", Name: "Groceries", Items: []*pb.ListItem{{Identifier: "item-1", ListId: "list-1", Name: "Milk", Prices: []*pb.PBItemPrice{price}}}}},
	}}
	if err := st.SyncFromUserData(data); err != nil {
		t.Fatalf("SyncFromUserData returned error: %v", err)
	}
	item, err := st.FindItemByID("list-1", "item-1")
	if err != nil {
		t.Fatalf("FindItemByID returned error: %v", err)
	}
	if len(item.Prices) != 1 || item.Prices[0].GetAmount() != 3.49 || item.Prices[0].GetStoreId() != "store-1" {
		t.Fatalf("cached prices = %#v, want the synced price", item.Prices)
	}
}

func TestSearchFTSPrefixQueriesQuotePunctuation(t *testing.T) {
	t.Parallel()

	st, err := Open(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	if _, err := st.db.Exec(`INSERT INTO lists (id, name) VALUES ('list-1', 'Groceries')`); err != nil {
		t.Fatalf("insert list: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO items (id, list_id, name) VALUES ('item-1', 'list-1', 'Example Value')`); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	if _, err := st.db.Exec(`INSERT INTO recipes (id, name) VALUES ('recipe-1', 'Example Value Recipe')`); err != nil {
		t.Fatalf("insert recipe: %v", err)
	}

	items, err := st.SearchItems("example-value")
	if err != nil {
		t.Fatalf("SearchItems returned error for punctuation query: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("SearchItems punctuation result = %#v, want one tokenized match", items)
	}
	recipes, err := st.SearchRecipesByName("example-value")
	if err != nil {
		t.Fatalf("SearchRecipesByName returned error for punctuation query: %v", err)
	}
	if len(recipes) != 1 {
		t.Fatalf("SearchRecipesByName punctuation result = %#v, want one tokenized match", recipes)
	}

	items, err = st.SearchItems("example")
	if err != nil || len(items) != 1 {
		t.Fatalf("SearchItems prefix result = %#v, err %v, want one match", items, err)
	}
	recipes, err = st.SearchRecipesByName("example")
	if err != nil || len(recipes) != 1 {
		t.Fatalf("SearchRecipesByName prefix result = %#v, err %v, want one match", recipes, err)
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
