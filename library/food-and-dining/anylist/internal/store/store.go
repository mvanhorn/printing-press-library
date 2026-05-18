// Package store provides a SQLite-backed local cache for AnyList data.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"

	_ "modernc.org/sqlite"
)

// Row types

type ListRow struct {
	ID              string
	Name            string
	Creator         string
	Timestamp       float64
	SortOrder       int
	NewItemPosition int
}

type ItemRow struct {
	ID              string
	ListID          string
	Name            string
	Quantity        string
	Details         string
	Category        string
	CategoryMatchID string
	Checked         bool
	SortIndex       int
	StoreIDs        []string
}

type RecipeRow struct {
	ID                string
	Name              string
	Note              string
	SourceName        string
	SourceURL         string
	Servings          string
	Rating            int
	PrepTime          int
	CookTime          int
	Timestamp         float64
	CreationTimestamp float64
}

type IngredientRow struct {
	ID            string
	RecipeID      string
	RawIngredient string
	Name          string
	Quantity      string
	Note          string
	SortIndex     int
}

type RecipeStepRow struct {
	ID        string
	RecipeID  string
	Text      string
	SortIndex int
}

type ItemSearchResult struct {
	ItemRow
	ListName string
}

type StoreGroup struct {
	StoreName string
	Items     []ItemRow
}

type RecipeIngredientResult struct {
	RecipeRow
	IngredientName string
}

type MealEventRow struct {
	ID          string
	CalendarID  string
	Date        string
	Title       string
	Details     string
	RecipeID    string
	LabelID     string
	SortIndex   int
	ScaleFactor float64
}

type CalendarLabelRow struct {
	ID         string
	CalendarID string
	Name       string
	HexColor   string
	SortIndex  int
}

type RecipeCollectionRow struct {
	ID          string
	Name        string
	Timestamp   float64
	RecipeCount int
}

type ListFolderItemRow struct {
	Identifier string
	ItemType   int
}

type ListFolderRow struct {
	ID                 string
	Name               string
	Timestamp          float64
	ListsSortOrder     int
	FolderSortPosition int
	FolderHexColor     string
	Items              []ListFolderItemRow
}

// StoreSchemaVersion is incremented whenever the schema changes in a breaking
// way. The database's user_version pragma is compared against this constant on
// every open; a mismatch means the cache was written by a different code version
// and must be rebuilt (delete the .db file and re-run sync).
const StoreSchemaVersion = 1

// Store wraps a SQLite database.
type Store struct {
	db *sql.DB
}

// DB exposes the raw database connection.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Open opens (or creates) the SQLite database adjacent to the config file.
func Open(cfg *config.Config) (*Store, error) {
	dir := filepath.Dir(cfg.Path)
	dbPath := filepath.Join(dir, "anylist.db")
	dsn := dbPath + "?_journal=WAL&_foreign_keys=on"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating store: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	var dbVersion int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&dbVersion); err != nil {
		return fmt.Errorf("reading user_version: %w", err)
	}
	if dbVersion != 0 && dbVersion != StoreSchemaVersion {
		return fmt.Errorf("schema version mismatch: db=%d code=%d — delete the cache file and re-run sync", dbVersion, StoreSchemaVersion)
	}

	schema := `
CREATE TABLE IF NOT EXISTS lists (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    creator TEXT DEFAULT '',
    timestamp REAL DEFAULT 0,
    list_item_sort_order INTEGER DEFAULT 0,
    new_list_item_position INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS items (
    id TEXT PRIMARY KEY,
    list_id TEXT NOT NULL,
    name TEXT NOT NULL,
    quantity TEXT DEFAULT '',
    details TEXT DEFAULT '',
    category TEXT DEFAULT '',
    category_match_id TEXT DEFAULT '',
    checked INTEGER NOT NULL DEFAULT 0,
    manual_sort_index INTEGER DEFAULT 0,
    store_ids TEXT DEFAULT '[]'
);

CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(name, content='items', content_rowid='rowid');

CREATE TABLE IF NOT EXISTS recipes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    note TEXT DEFAULT '',
    source_name TEXT DEFAULT '',
    source_url TEXT DEFAULT '',
    rating INTEGER DEFAULT 0,
    prep_time INTEGER DEFAULT 0,
    cook_time INTEGER DEFAULT 0,
    servings TEXT DEFAULT '',
    timestamp REAL DEFAULT 0,
    creation_timestamp REAL DEFAULT 0
);

CREATE VIRTUAL TABLE IF NOT EXISTS recipes_fts USING fts5(name, note, content='recipes', content_rowid='rowid');

CREATE TABLE IF NOT EXISTS ingredients (
    id TEXT PRIMARY KEY,
    recipe_id TEXT NOT NULL,
    raw_ingredient TEXT DEFAULT '',
    name TEXT NOT NULL,
    quantity TEXT DEFAULT '',
    note TEXT DEFAULT '',
    sort_index INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS recipe_steps (
    id TEXT PRIMARY KEY,
    recipe_id TEXT NOT NULL,
    text TEXT NOT NULL,
    sort_index INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS recipe_collections (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    timestamp REAL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS recipe_collection_members (
    collection_id TEXT NOT NULL,
    recipe_id TEXT NOT NULL,
    PRIMARY KEY (collection_id, recipe_id)
);

CREATE TABLE IF NOT EXISTS list_folders (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    timestamp REAL DEFAULT 0,
    lists_sort_order INTEGER DEFAULT 0,
    folder_sort_position INTEGER DEFAULT 0,
    folder_hex_color TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS list_folder_items (
    folder_id TEXT NOT NULL,
    identifier TEXT NOT NULL,
    item_type INTEGER DEFAULT 0,
    sort_index INTEGER DEFAULT 0,
    PRIMARY KEY (folder_id, identifier)
);

CREATE TABLE IF NOT EXISTS meal_events (
    id TEXT PRIMARY KEY,
    calendar_id TEXT DEFAULT '',
    date TEXT NOT NULL,
    title TEXT DEFAULT '',
    details TEXT DEFAULT '',
    recipe_id TEXT DEFAULT '',
    label_id TEXT DEFAULT '',
    sort_index INTEGER DEFAULT 0,
    scale_factor REAL DEFAULT 1.0
);

CREATE TABLE IF NOT EXISTS calendar_labels (
    id TEXT PRIMARY KEY,
    calendar_id TEXT DEFAULT '',
    name TEXT NOT NULL,
    hex_color TEXT DEFAULT '',
    sort_index INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS stores (
    id TEXT PRIMARY KEY,
    list_id TEXT DEFAULT '',
    name TEXT NOT NULL,
    sort_index INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS starter_items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    quantity TEXT DEFAULT '',
    category TEXT DEFAULT '',
    list_type TEXT NOT NULL DEFAULT 'starter'
);

CREATE TABLE IF NOT EXISTS _sync_meta (
    entity TEXT PRIMARY KEY,
    last_sync INTEGER NOT NULL
);

CREATE TRIGGER IF NOT EXISTS items_fts_insert AFTER INSERT ON items BEGIN
  INSERT INTO items_fts(rowid, name) VALUES (new.rowid, new.name);
END;
CREATE TRIGGER IF NOT EXISTS items_fts_delete AFTER DELETE ON items BEGIN
  INSERT INTO items_fts(items_fts, rowid, name) VALUES('delete', old.rowid, old.name);
END;
CREATE TRIGGER IF NOT EXISTS items_fts_update AFTER UPDATE ON items BEGIN
  INSERT INTO items_fts(items_fts, rowid, name) VALUES('delete', old.rowid, old.name);
  INSERT INTO items_fts(rowid, name) VALUES (new.rowid, new.name);
END;
CREATE TRIGGER IF NOT EXISTS recipes_fts_insert AFTER INSERT ON recipes BEGIN
  INSERT INTO recipes_fts(rowid, name, note) VALUES (new.rowid, new.name, new.note);
END;
CREATE TRIGGER IF NOT EXISTS recipes_fts_delete AFTER DELETE ON recipes BEGIN
  INSERT INTO recipes_fts(recipes_fts, rowid, name, note) VALUES('delete', old.rowid, old.name, old.note);
END;
CREATE TRIGGER IF NOT EXISTS recipes_fts_update AFTER UPDATE ON recipes BEGIN
  INSERT INTO recipes_fts(recipes_fts, rowid, name, note) VALUES('delete', old.rowid, old.name, old.note);
  INSERT INTO recipes_fts(rowid, name, note) VALUES (new.rowid, new.name, new.note);
END;
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	if err := s.ensureColumn("lists", "new_list_item_position", "INTEGER DEFAULT 0"); err != nil {
		return err
	}
	_, err := s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", StoreSchemaVersion))
	return err
}

func (s *Store) ensureColumn(table, column, ddl string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return fmt.Errorf("checking table %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + ddl)
	return err
}

// SyncFromUserData populates all tables from the protobuf user data response.
func (s *Store) SyncFromUserData(userData *pb.PBUserDataResponse) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Sync lists and items
	if slr := userData.GetShoppingListsResponse(); slr != nil {
		allLists := append(slr.GetNewLists(), slr.GetModifiedLists()...)
		for _, list := range allLists {
			if err := upsertList(tx, list); err != nil {
				return fmt.Errorf("upserting list %s: %w", list.GetIdentifier(), err)
			}
			if _, err := tx.Exec(`DELETE FROM items WHERE list_id = ?`, list.GetIdentifier()); err != nil {
				return fmt.Errorf("clearing items for list %s: %w", list.GetIdentifier(), err)
			}
			for _, item := range list.GetItems() {
				if err := upsertItem(tx, item); err != nil {
					return fmt.Errorf("upserting item %s: %w", item.GetIdentifier(), err)
				}
			}
		}

		// Sync stores from list responses
		for _, lr := range slr.GetListResponses() {
			for _, store := range lr.GetStores() {
				if err := upsertStore(tx, store); err != nil {
					return fmt.Errorf("upserting store %s: %w", store.GetIdentifier(), err)
				}
			}
		}
	}

	if lfr := userData.GetListFoldersResponse(); lfr != nil {
		if _, err := tx.Exec(`DELETE FROM list_folder_items`); err != nil {
			return fmt.Errorf("clearing list folder items: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM list_folders`); err != nil {
			return fmt.Errorf("clearing list folders: %w", err)
		}
		for _, folder := range lfr.GetListFolders() {
			if err := upsertListFolder(tx, folder); err != nil {
				return fmt.Errorf("upserting list folder %s: %w", folder.GetIdentifier(), err)
			}
		}
	}

	// Sync recipes
	if rdr := userData.GetRecipeDataResponse(); rdr != nil {
		if _, err := tx.Exec(`DELETE FROM recipe_steps`); err != nil {
			return fmt.Errorf("clearing recipe steps: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM ingredients`); err != nil {
			return fmt.Errorf("clearing ingredients: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM recipes`); err != nil {
			return fmt.Errorf("clearing recipes: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM recipe_collection_members`); err != nil {
			return fmt.Errorf("clearing recipe collection members: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM recipe_collections`); err != nil {
			return fmt.Errorf("clearing recipe collections: %w", err)
		}
		for _, recipe := range rdr.GetRecipes() {
			if err := upsertRecipe(tx, recipe); err != nil {
				return fmt.Errorf("upserting recipe %s: %w", recipe.GetIdentifier(), err)
			}
			// Upsert ingredients
			for i, ing := range recipe.GetIngredients() {
				ingID := recipe.GetIdentifier() + ":" + strconv.Itoa(i)
				if err := upsertIngredient(tx, ingID, recipe.GetIdentifier(), i, ing); err != nil {
					return fmt.Errorf("upserting ingredient: %w", err)
				}
			}
			// Upsert preparation steps
			for i, step := range recipe.GetPreparationSteps() {
				stepID := recipe.GetIdentifier() + ":" + strconv.Itoa(i)
				if _, err := tx.Exec(
					`INSERT OR REPLACE INTO recipe_steps (id, recipe_id, text, sort_index) VALUES (?, ?, ?, ?)`,
					stepID, recipe.GetIdentifier(), step, i,
				); err != nil {
					return fmt.Errorf("upserting recipe step: %w", err)
				}
			}
		}

		// Sync recipe collections
		for _, col := range rdr.GetRecipeCollections() {
			if _, err := tx.Exec(
				`INSERT OR REPLACE INTO recipe_collections (id, name, timestamp) VALUES (?, ?, ?)`,
				col.GetIdentifier(), col.GetName(), col.GetTimestamp(),
			); err != nil {
				return fmt.Errorf("upserting recipe collection: %w", err)
			}
			for _, recipeID := range col.GetRecipeIds() {
				if _, err := tx.Exec(
					`INSERT OR IGNORE INTO recipe_collection_members (collection_id, recipe_id) VALUES (?, ?)`,
					col.GetIdentifier(), recipeID,
				); err != nil {
					return fmt.Errorf("upserting recipe collection member: %w", err)
				}
			}
		}
	}

	// Sync meal events and calendar labels
	if cr := userData.GetMealPlanningCalendarResponse(); cr != nil {
		// PATCH: User-data sync returns the current meal calendar, so replace
		// cached rows before re-inserting to drop events and labels deleted upstream.
		if _, err := tx.Exec(`DELETE FROM meal_events`); err != nil {
			return fmt.Errorf("clearing meal events: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM calendar_labels`); err != nil {
			return fmt.Errorf("clearing calendar labels: %w", err)
		}
		for _, event := range cr.GetEvents() {
			if err := upsertMealEvent(tx, event); err != nil {
				return fmt.Errorf("upserting meal event %s: %w", event.GetIdentifier(), err)
			}
		}
		for _, label := range cr.GetLabels() {
			if err := upsertCalendarLabel(tx, label); err != nil {
				return fmt.Errorf("upserting calendar label %s: %w", label.GetIdentifier(), err)
			}
		}
	}

	// Sync starter lists (user starters and favorites)
	if slr := userData.GetStarterListsResponse(); slr != nil {
		syncStarterBatch := func(batch interface {
			GetListResponses() []*pb.StarterListResponse
		}, listType string) error {
			for _, resp := range batch.GetListResponses() {
				sl := resp.GetStarterList()
				if sl == nil {
					continue
				}
				for _, item := range sl.GetItems() {
					if item.GetIdentifier() == "" || item.GetName() == "" {
						continue
					}
					if _, err := tx.Exec(
						`INSERT OR REPLACE INTO starter_items (id, name, quantity, category, list_type) VALUES (?, ?, ?, ?, ?)`,
						item.GetIdentifier(), item.GetName(), item.GetQuantity(), item.GetCategory(), listType,
					); err != nil {
						return fmt.Errorf("upserting starter item: %w", err)
					}
				}
			}
			return nil
		}
		if err := syncStarterBatch(slr.GetUserListsResponse(), "starter"); err != nil {
			return err
		}
		if err := syncStarterBatch(slr.GetFavoriteItemListsResponse(), "favorite"); err != nil {
			return err
		}
		if err := syncStarterBatch(slr.GetRecentItemListsResponse(), "recent"); err != nil {
			return err
		}
	}

	// Update _sync_meta
	now := time.Now().Unix()
	for _, entity := range []string{"lists", "recipes", "meal", "stores"} {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO _sync_meta (entity, last_sync) VALUES (?, ?)`,
			entity, now,
		); err != nil {
			return fmt.Errorf("updating sync meta for %s: %w", entity, err)
		}
	}

	return tx.Commit()
}

func upsertList(tx *sql.Tx, list *pb.ShoppingList) error {
	_, err := tx.Exec(
		`INSERT OR REPLACE INTO lists (id, name, creator, timestamp, list_item_sort_order, new_list_item_position)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		list.GetIdentifier(),
		list.GetName(),
		list.GetCreator(),
		list.GetTimestamp(),
		int(list.GetListItemSortOrder()),
		int(list.GetNewListItemPosition()),
	)
	return err
}

func upsertListFolder(tx *sql.Tx, folder *pb.PBListFolder) error {
	settings := folder.GetFolderSettings()
	listsSortOrder := 0
	folderSortPosition := 0
	folderHexColor := ""
	if settings != nil {
		listsSortOrder = int(settings.GetListsSortOrder())
		folderSortPosition = int(settings.GetFolderSortPosition())
		folderHexColor = settings.GetFolderHexColor()
	}
	if _, err := tx.Exec(
		`INSERT OR REPLACE INTO list_folders
		 (id, name, timestamp, lists_sort_order, folder_sort_position, folder_hex_color)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		folder.GetIdentifier(),
		folder.GetName(),
		folder.GetTimestamp(),
		listsSortOrder,
		folderSortPosition,
		folderHexColor,
	); err != nil {
		return err
	}
	for i, item := range folder.GetItems() {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO list_folder_items (folder_id, identifier, item_type, sort_index)
			 VALUES (?, ?, ?, ?)`,
			folder.GetIdentifier(),
			item.GetIdentifier(),
			int(item.GetItemType()),
			i,
		); err != nil {
			return err
		}
	}
	return nil
}

func upsertItem(tx *sql.Tx, item *pb.ListItem) error {
	storeIDsJSON, err := json.Marshal(item.GetStoreIds())
	if err != nil {
		storeIDsJSON = []byte("[]")
	}
	checked := 0
	if item.GetChecked() {
		checked = 1
	}
	_, err = tx.Exec(
		`INSERT OR REPLACE INTO items
		 (id, list_id, name, quantity, details, category, category_match_id, checked, manual_sort_index, store_ids)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.GetIdentifier(),
		item.GetListId(),
		item.GetName(),
		item.GetQuantity(),
		item.GetDetails(),
		item.GetCategory(),
		item.GetCategoryMatchId(),
		checked,
		int(item.GetManualSortIndex()),
		string(storeIDsJSON),
	)
	return err
}

func upsertRecipe(tx *sql.Tx, recipe *pb.PBRecipe) error {
	_, err := tx.Exec(
		`INSERT OR REPLACE INTO recipes
		 (id, name, note, source_name, source_url, rating, prep_time, cook_time, servings, timestamp, creation_timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		recipe.GetIdentifier(),
		recipe.GetName(),
		recipe.GetNote(),
		recipe.GetSourceName(),
		recipe.GetSourceUrl(),
		int(recipe.GetRating()),
		int(recipe.GetPrepTime()),
		int(recipe.GetCookTime()),
		recipe.GetServings(),
		recipe.GetTimestamp(),
		recipe.GetCreationTimestamp(),
	)
	return err
}

func upsertIngredient(tx *sql.Tx, id, recipeID string, sortIndex int, ing *pb.PBIngredient) error {
	_, err := tx.Exec(
		`INSERT OR REPLACE INTO ingredients (id, recipe_id, raw_ingredient, name, quantity, note, sort_index)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id,
		recipeID,
		ing.GetRawIngredient(),
		ing.GetName(),
		ing.GetQuantity(),
		ing.GetNote(),
		sortIndex,
	)
	return err
}

func upsertMealEvent(tx *sql.Tx, event *pb.PBCalendarEvent) error {
	scaleFactor := event.GetRecipeScaleFactor()
	if scaleFactor == 0 {
		scaleFactor = 1.0
	}
	_, err := tx.Exec(
		`INSERT OR REPLACE INTO meal_events
		 (id, calendar_id, date, title, details, recipe_id, label_id, sort_index, scale_factor)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.GetIdentifier(),
		event.GetCalendarId(),
		event.GetDate(),
		event.GetTitle(),
		event.GetDetails(),
		event.GetRecipeId(),
		event.GetLabelId(),
		int(event.GetOrderAddedSortIndex()),
		scaleFactor,
	)
	return err
}

func upsertCalendarLabel(tx *sql.Tx, label *pb.PBCalendarLabel) error {
	_, err := tx.Exec(
		`INSERT OR REPLACE INTO calendar_labels (id, calendar_id, name, hex_color, sort_index)
		 VALUES (?, ?, ?, ?, ?)`,
		label.GetIdentifier(),
		label.GetCalendarId(),
		label.GetName(),
		label.GetHexColor(),
		int(label.GetSortIndex()),
	)
	return err
}

func upsertStore(tx *sql.Tx, store *pb.PBStore) error {
	_, err := tx.Exec(
		`INSERT OR REPLACE INTO stores (id, list_id, name, sort_index)
		 VALUES (?, ?, ?, ?)`,
		store.GetIdentifier(),
		store.GetListId(),
		store.GetName(),
		int(store.GetSortIndex()),
	)
	return err
}

// GetLists returns all lists.
func (s *Store) GetLists() ([]ListRow, error) {
	rows, err := s.db.Query(`SELECT id, name, creator, timestamp, list_item_sort_order, new_list_item_position FROM lists ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lists []ListRow
	for rows.Next() {
		var l ListRow
		if err := rows.Scan(&l.ID, &l.Name, &l.Creator, &l.Timestamp, &l.SortOrder, &l.NewItemPosition); err != nil {
			return nil, err
		}
		lists = append(lists, l)
	}
	return lists, rows.Err()
}

// FindListByName finds a list by name with case-insensitive fuzzy matching.
func (s *Store) FindListByName(name string) (*ListRow, error) {
	lower := strings.ToLower(name)
	rows, err := s.db.Query(`SELECT id, name, creator, timestamp, list_item_sort_order, new_list_item_position FROM lists`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []ListRow
	for rows.Next() {
		var l ListRow
		if err := rows.Scan(&l.ID, &l.Name, &l.Creator, &l.Timestamp, &l.SortOrder, &l.NewItemPosition); err != nil {
			return nil, err
		}
		all = append(all, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Exact match
	for i, l := range all {
		if strings.EqualFold(l.Name, name) {
			return &all[i], nil
		}
	}
	// Prefix match
	for i, l := range all {
		if strings.HasPrefix(strings.ToLower(l.Name), lower) {
			return &all[i], nil
		}
	}
	// Contains match
	for i, l := range all {
		if strings.Contains(strings.ToLower(l.Name), lower) {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("list %q not found — run 'anylist-pp-cli sync' first", name)
}

// GetItems returns items for a list, optionally filtered by checked state.
func (s *Store) GetItems(listID string, checked *bool) ([]ItemRow, error) {
	query := `SELECT id, list_id, name, quantity, details, category, category_match_id, checked, manual_sort_index, store_ids
	          FROM items WHERE list_id = ?`
	args := []any{listID}
	if checked != nil {
		if *checked {
			query += " AND checked = 1"
		} else {
			query += " AND checked = 0"
		}
	}
	query += " ORDER BY manual_sort_index, name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanItems(rows)
}

// FindItemByName finds an item in a list by name (case-insensitive).
func (s *Store) FindItemByName(listID, name string) (*ItemRow, error) {
	lower := strings.ToLower(name)
	rows, err := s.db.Query(
		`SELECT id, list_id, name, quantity, details, category, category_match_id, checked, manual_sort_index, store_ids
		 FROM items WHERE list_id = ?`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}

	// Exact match
	for i, it := range items {
		if strings.EqualFold(it.Name, name) {
			return &items[i], nil
		}
	}
	// Prefix match
	for i, it := range items {
		if strings.HasPrefix(strings.ToLower(it.Name), lower) {
			return &items[i], nil
		}
	}
	// Contains match
	for i, it := range items {
		if strings.Contains(strings.ToLower(it.Name), lower) {
			return &items[i], nil
		}
	}
	return nil, fmt.Errorf("item %q not found in list", name)
}

// FindItemByID finds an item in a list by exact item identifier.
func (s *Store) FindItemByID(listID, itemID string) (*ItemRow, error) {
	rows, err := s.db.Query(
		`SELECT id, list_id, name, quantity, details, category, category_match_id, checked, manual_sort_index, store_ids
		 FROM items WHERE list_id = ? AND id = ?`, listID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("item id %q not found in list", itemID)
	}
	return &items[0], nil
}

// SearchItems searches items across all lists using FTS5.
func (s *Store) SearchItems(query string) ([]ItemSearchResult, error) {
	ftsQuery := query + "*"
	sqlQuery := `
		SELECT i.id, i.list_id, i.name, i.quantity, i.details, i.category, i.category_match_id,
		       i.checked, i.manual_sort_index, i.store_ids, l.name as list_name
		FROM items i
		JOIN lists l ON i.list_id = l.id
		WHERE i.rowid IN (SELECT rowid FROM items_fts WHERE items_fts MATCH ?)
		ORDER BY l.name, i.name`

	rows, err := s.db.Query(sqlQuery, ftsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ItemSearchResult
	for rows.Next() {
		var r ItemSearchResult
		var checkedInt int
		var storeIDsStr string
		if err := rows.Scan(
			&r.ID, &r.ListID, &r.Name, &r.Quantity, &r.Details, &r.Category,
			&r.CategoryMatchID, &checkedInt, &r.SortIndex, &storeIDsStr, &r.ListName,
		); err != nil {
			return nil, err
		}
		r.Checked = checkedInt != 0
		r.StoreIDs = parseStoreIDs(storeIDsStr)
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetListsByStore returns unchecked items grouped by store for a list.
func (s *Store) GetListsByStore(listID string) ([]StoreGroup, error) {
	sqlQuery := `
		SELECT i.id, i.list_id, i.name, i.quantity, i.details, i.category, i.category_match_id,
		       i.checked, i.manual_sort_index, i.store_ids,
		       COALESCE(s.name, 'Unassigned') as store_name, COALESCE(s.sort_index, 9999) as ssi
		FROM items i
		LEFT JOIN (
			  SELECT s.id, s.name, s.sort_index
			  FROM stores s
			  WHERE s.list_id = ?
			) s ON EXISTS (
			  SELECT 1
			  FROM json_each(i.store_ids) store_id
			  WHERE store_id.value = s.id
			)
			WHERE i.list_id = ? AND i.checked = 0
			ORDER BY ssi, i.manual_sort_index`

	rows, err := s.db.Query(sqlQuery, listID, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groupMap := map[string][]ItemRow{}
	var groupOrder []string
	seen := map[string]bool{}

	for rows.Next() {
		var item ItemRow
		var checkedInt int
		var storeIDsStr string
		var storeName string
		var ssi int
		if err := rows.Scan(
			&item.ID, &item.ListID, &item.Name, &item.Quantity, &item.Details,
			&item.Category, &item.CategoryMatchID, &checkedInt, &item.SortIndex,
			&storeIDsStr, &storeName, &ssi,
		); err != nil {
			return nil, err
		}
		item.Checked = checkedInt != 0
		item.StoreIDs = parseStoreIDs(storeIDsStr)
		groupMap[storeName] = append(groupMap[storeName], item)
		if !seen[storeName] {
			seen[storeName] = true
			groupOrder = append(groupOrder, storeName)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var groups []StoreGroup
	for _, name := range groupOrder {
		groups = append(groups, StoreGroup{StoreName: name, Items: groupMap[name]})
	}
	return groups, nil
}

// GetCheckedItems returns checked items in a list.
func (s *Store) GetCheckedItems(listID string) ([]ItemRow, error) {
	checked := true
	return s.GetItems(listID, &checked)
}

// GetRecipes returns all recipes.
func (s *Store) GetRecipes() ([]RecipeRow, error) {
	rows, err := s.db.Query(
		`SELECT id, name, note, source_name, source_url, rating, prep_time, cook_time,
		        servings, timestamp, creation_timestamp
		 FROM recipes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecipes(rows)
}

// FindRecipeByID returns a recipe by its stable AnyList identifier.
func (s *Store) FindRecipeByID(id string) (*RecipeRow, error) {
	var r RecipeRow
	err := s.db.QueryRow(
		`SELECT id, name, note, source_name, source_url, rating, prep_time, cook_time,
		        servings, timestamp, creation_timestamp FROM recipes WHERE id = ?`,
		id,
	).Scan(
		&r.ID, &r.Name, &r.Note, &r.SourceName, &r.SourceURL,
		&r.Rating, &r.PrepTime, &r.CookTime, &r.Servings,
		&r.Timestamp, &r.CreationTimestamp,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("recipe id %q not found — run 'anylist-pp-cli sync' first", id)
		}
		return nil, err
	}
	return &r, nil
}

// FindRecipeByName finds a recipe by name with case-insensitive fuzzy matching.
func (s *Store) FindRecipeByName(name string) (*RecipeRow, error) {
	lower := strings.ToLower(name)
	rows, err := s.db.Query(
		`SELECT id, name, note, source_name, source_url, rating, prep_time, cook_time,
		        servings, timestamp, creation_timestamp FROM recipes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	all, err := scanRecipes(rows)
	if err != nil {
		return nil, err
	}

	// Exact match
	for i, r := range all {
		if strings.EqualFold(r.Name, name) {
			return &all[i], nil
		}
	}
	// Prefix match
	for i, r := range all {
		if strings.HasPrefix(strings.ToLower(r.Name), lower) {
			return &all[i], nil
		}
	}
	// Contains match
	for i, r := range all {
		if strings.Contains(strings.ToLower(r.Name), lower) {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("recipe %q not found — run 'anylist-pp-cli sync' first", name)
}

// GetIngredients returns ingredients for a recipe ordered by sort_index.
func (s *Store) GetIngredients(recipeID string) ([]IngredientRow, error) {
	rows, err := s.db.Query(
		`SELECT id, recipe_id, raw_ingredient, name, quantity, note, sort_index
		 FROM ingredients WHERE recipe_id = ? ORDER BY sort_index`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIngredients(rows)
}

// GetRecipeSteps returns preparation steps for a recipe.
func (s *Store) GetRecipeSteps(recipeID string) ([]RecipeStepRow, error) {
	rows, err := s.db.Query(
		`SELECT id, recipe_id, text, sort_index FROM recipe_steps WHERE recipe_id = ? ORDER BY sort_index`,
		recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var steps []RecipeStepRow
	for rows.Next() {
		var st RecipeStepRow
		if err := rows.Scan(&st.ID, &st.RecipeID, &st.Text, &st.SortIndex); err != nil {
			return nil, err
		}
		steps = append(steps, st)
	}
	return steps, rows.Err()
}

// SearchRecipesByIngredient finds recipes that contain an ingredient matching the query.
func (s *Store) SearchRecipesByIngredient(ingredient string) ([]RecipeIngredientResult, error) {
	lower := "%" + strings.ToLower(ingredient) + "%"
	sqlQuery := `
		SELECT r.id, r.name, r.note, r.source_name, r.source_url, r.rating, r.prep_time, r.cook_time,
		       r.servings, r.timestamp, r.creation_timestamp, i.name as ingredient_name
		FROM recipes r
		JOIN ingredients i ON i.recipe_id = r.id
		WHERE LOWER(i.name) LIKE ?
		ORDER BY r.name`

	rows, err := s.db.Query(sqlQuery, lower)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RecipeIngredientResult
	seen := map[string]bool{}
	for rows.Next() {
		var r RecipeIngredientResult
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Note, &r.SourceName, &r.SourceURL,
			&r.Rating, &r.PrepTime, &r.CookTime, &r.Servings,
			&r.Timestamp, &r.CreationTimestamp, &r.IngredientName,
		); err != nil {
			return nil, err
		}
		if !seen[r.ID] {
			seen[r.ID] = true
			results = append(results, r)
		}
	}
	return results, rows.Err()
}

// SearchRecipesByName searches recipes by name using FTS5.
func (s *Store) SearchRecipesByName(query string) ([]RecipeRow, error) {
	ftsQuery := query + "*"
	sqlQuery := `
		SELECT r.id, r.name, r.note, r.source_name, r.source_url, r.rating, r.prep_time, r.cook_time,
		       r.servings, r.timestamp, r.creation_timestamp
		FROM recipes r
		WHERE r.rowid IN (SELECT rowid FROM recipes_fts WHERE recipes_fts MATCH ?)
		ORDER BY r.name`

	rows, err := s.db.Query(sqlQuery, ftsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecipes(rows)
}

// FilterRecipes filters recipes by prep time, cook time, rating, servings, and collection.
func (s *Store) FilterRecipes(maxPrepTime, maxCookTime, minRating, servings int, collection string) ([]RecipeRow, error) {
	query := `SELECT id, name, note, source_name, source_url, rating, prep_time, cook_time,
	                 servings, timestamp, creation_timestamp FROM recipes`
	var conditions []string
	var args []any

	if maxPrepTime > 0 {
		conditions = append(conditions, "prep_time <= ? AND prep_time > 0")
		args = append(args, maxPrepTime)
	}
	if maxCookTime > 0 {
		conditions = append(conditions, "cook_time <= ? AND cook_time > 0")
		args = append(args, maxCookTime)
	}
	if minRating > 0 {
		conditions = append(conditions, "rating >= ?")
		args = append(args, minRating)
	}
	if servings > 0 {
		conditions = append(conditions, "servings = ?")
		args = append(args, strconv.Itoa(servings))
	}
	if collection != "" {
		query = `SELECT r.id, r.name, r.note, r.source_name, r.source_url, r.rating, r.prep_time, r.cook_time,
		                r.servings, r.timestamp, r.creation_timestamp
		         FROM recipes r
		         JOIN recipe_collection_members rcm ON rcm.recipe_id = r.id
		         JOIN recipe_collections rc ON rc.id = rcm.collection_id`
		conditions = append(conditions, "LOWER(rc.name) LIKE ?")
		args = append(args, "%"+strings.ToLower(collection)+"%")
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY name"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecipes(rows)
}

// GetRecipeCollections returns all recipe collections with member counts.
func (s *Store) GetRecipeCollections() ([]RecipeCollectionRow, error) {
	rows, err := s.db.Query(
		`SELECT rc.id, rc.name, rc.timestamp, COUNT(rcm.recipe_id) AS recipe_count
		 FROM recipe_collections rc
		 LEFT JOIN recipe_collection_members rcm ON rcm.collection_id = rc.id
		 GROUP BY rc.id, rc.name, rc.timestamp
		 ORDER BY rc.name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var collections []RecipeCollectionRow
	for rows.Next() {
		var c RecipeCollectionRow
		if err := rows.Scan(&c.ID, &c.Name, &c.Timestamp, &c.RecipeCount); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, rows.Err()
}

// GetListFolders returns all list folders with their ordered contents.
func (s *Store) GetListFolders() ([]ListFolderRow, error) {
	rows, err := s.db.Query(
		`SELECT id, name, timestamp, lists_sort_order, folder_sort_position, folder_hex_color
		 FROM list_folders ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []ListFolderRow
	for rows.Next() {
		var f ListFolderRow
		if err := rows.Scan(&f.ID, &f.Name, &f.Timestamp, &f.ListsSortOrder, &f.FolderSortPosition, &f.FolderHexColor); err != nil {
			return nil, err
		}
		items, err := s.getListFolderItems(f.ID)
		if err != nil {
			return nil, err
		}
		f.Items = items
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

func (s *Store) getListFolderItems(folderID string) ([]ListFolderItemRow, error) {
	rows, err := s.db.Query(
		`SELECT identifier, item_type FROM list_folder_items WHERE folder_id = ? ORDER BY sort_index`,
		folderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ListFolderItemRow
	for rows.Next() {
		var item ListFolderItemRow
		if err := rows.Scan(&item.Identifier, &item.ItemType); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetMissingIngredients returns ingredients not already present (as unchecked items) in a list.
func (s *Store) GetMissingIngredients(recipeID, listID string) ([]IngredientRow, error) {
	sqlQuery := `
		SELECT ing.id, ing.recipe_id, ing.raw_ingredient, ing.name, ing.quantity, ing.note, ing.sort_index
		FROM ingredients ing
		WHERE ing.recipe_id = ?
		AND NOT EXISTS (
		  SELECT 1 FROM items it
		  WHERE it.list_id = ?
		  AND it.checked = 0
		  AND LOWER(it.name) LIKE '%' || REPLACE(REPLACE(REPLACE(LOWER(ing.name), '\', '\\'), '%', '\%'), '_', '\_') || '%' ESCAPE '\'
		)
		ORDER BY ing.sort_index`

	rows, err := s.db.Query(sqlQuery, recipeID, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIngredients(rows)
}

// GetMealEvents returns meal events within a date range.
func (s *Store) GetMealEvents(from, to string) ([]MealEventRow, error) {
	rows, err := s.db.Query(
		`SELECT id, calendar_id, date, title, details, recipe_id, label_id, sort_index, scale_factor
		 FROM meal_events WHERE date >= ? AND date <= ? ORDER BY date, sort_index`,
		from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []MealEventRow
	for rows.Next() {
		var e MealEventRow
		if err := rows.Scan(&e.ID, &e.CalendarID, &e.Date, &e.Title, &e.Details,
			&e.RecipeID, &e.LabelID, &e.SortIndex, &e.ScaleFactor); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetCalendarLabels returns all calendar labels.
func (s *Store) GetCalendarLabels() ([]CalendarLabelRow, error) {
	rows, err := s.db.Query(
		`SELECT id, calendar_id, name, hex_color, sort_index FROM calendar_labels ORDER BY sort_index, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []CalendarLabelRow
	for rows.Next() {
		var l CalendarLabelRow
		if err := rows.Scan(&l.ID, &l.CalendarID, &l.Name, &l.HexColor, &l.SortIndex); err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

// UpsertSyncTimestamp records that entity was synced at t. Use time.Now() for
// the common case; pass a specific time when replaying historical sync records.
func (s *Store) UpsertSyncTimestamp(entity string, t time.Time) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO _sync_meta (entity, last_sync) VALUES (?, ?)`,
		entity, t.Unix(),
	)
	return err
}

// SaveSyncState marks entity as synced at the current time.
func (s *Store) SaveSyncState(entity string) error {
	return s.UpsertSyncTimestamp(entity, time.Now())
}

// GetSyncMeta returns last_sync timestamps per entity.
func (s *Store) GetSyncMeta() (map[string]time.Time, error) {
	rows, err := s.db.Query(`SELECT entity, last_sync FROM _sync_meta`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string]time.Time{}
	for rows.Next() {
		var entity string
		var ts int64
		if err := rows.Scan(&entity, &ts); err != nil {
			return nil, err
		}
		result[entity] = time.Unix(ts, 0)
	}
	return result, rows.Err()
}

// LastSync returns the last sync time for a single entity.
func (s *Store) LastSync(entity string) (time.Time, bool) {
	var ts int64
	err := s.db.QueryRow(`SELECT last_sync FROM _sync_meta WHERE entity = ?`, entity).Scan(&ts)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(ts, 0), true
}

// StarterItemRow holds a starter/favorite/recent item.
type StarterItemRow struct {
	ID       string
	Name     string
	Quantity string
	Category string
	ListType string
}

// GetStarterItems returns items of a given type ("starter", "favorite", "recent").
func (s *Store) GetStarterItems(listType string) ([]StarterItemRow, error) {
	rows, err := s.db.Query(
		`SELECT id, name, quantity, category, list_type FROM starter_items WHERE list_type = ? ORDER BY name`,
		listType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []StarterItemRow
	for rows.Next() {
		var it StarterItemRow
		if err := rows.Scan(&it.ID, &it.Name, &it.Quantity, &it.Category, &it.ListType); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// CategoryRow holds a distinct category from items.
type CategoryRow struct {
	MatchID string
	Name    string
}

// GetCategories returns distinct categories derived from items.
func (s *Store) GetCategories() ([]CategoryRow, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT category_match_id, category FROM items WHERE category != '' ORDER BY category_match_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cats []CategoryRow
	for rows.Next() {
		var c CategoryRow
		if err := rows.Scan(&c.MatchID, &c.Name); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// StoreRow holds a store record.
type StoreRow struct {
	ID        string
	ListID    string
	Name      string
	SortIndex int
}

// GetStores returns all stores ordered by sort_index.
func (s *Store) GetStores() ([]StoreRow, error) {
	rows, err := s.db.Query(
		`SELECT id, list_id, name, sort_index FROM stores ORDER BY sort_index, name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stores []StoreRow
	for rows.Next() {
		var st StoreRow
		if err := rows.Scan(&st.ID, &st.ListID, &st.Name, &st.SortIndex); err != nil {
			return nil, err
		}
		stores = append(stores, st)
	}
	return stores, rows.Err()
}

// --- helpers ---

func scanItems(rows *sql.Rows) ([]ItemRow, error) {
	var items []ItemRow
	for rows.Next() {
		var it ItemRow
		var checkedInt int
		var storeIDsStr string
		if err := rows.Scan(
			&it.ID, &it.ListID, &it.Name, &it.Quantity, &it.Details,
			&it.Category, &it.CategoryMatchID, &checkedInt, &it.SortIndex, &storeIDsStr,
		); err != nil {
			return nil, err
		}
		it.Checked = checkedInt != 0
		it.StoreIDs = parseStoreIDs(storeIDsStr)
		items = append(items, it)
	}
	return items, rows.Err()
}

func scanRecipes(rows *sql.Rows) ([]RecipeRow, error) {
	var recipes []RecipeRow
	for rows.Next() {
		var r RecipeRow
		if err := rows.Scan(
			&r.ID, &r.Name, &r.Note, &r.SourceName, &r.SourceURL,
			&r.Rating, &r.PrepTime, &r.CookTime, &r.Servings,
			&r.Timestamp, &r.CreationTimestamp,
		); err != nil {
			return nil, err
		}
		recipes = append(recipes, r)
	}
	return recipes, rows.Err()
}

func scanIngredients(rows *sql.Rows) ([]IngredientRow, error) {
	var ings []IngredientRow
	for rows.Next() {
		var ing IngredientRow
		if err := rows.Scan(
			&ing.ID, &ing.RecipeID, &ing.RawIngredient, &ing.Name,
			&ing.Quantity, &ing.Note, &ing.SortIndex,
		); err != nil {
			return nil, err
		}
		ings = append(ings, ing)
	}
	return ings, rows.Err()
}

func parseStoreIDs(s string) []string {
	var ids []string
	if err := json.Unmarshal([]byte(s), &ids); err != nil {
		return nil
	}
	return ids
}
