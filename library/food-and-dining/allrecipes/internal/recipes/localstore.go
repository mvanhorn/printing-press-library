// Package recipes — localstore.go provides Allrecipes-specific persistence
// helpers on top of the generated *store.Store. The generator-managed `recipes`
// table holds the JSON Recipe payload keyed by URL; this file adds the
// ingredient denormalization table that powers `pantry`, `with-ingredient`,
// `top-rated`, `quick`, and `dietary`.
package recipes

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/store"
)

// EnsureSchema creates the Allrecipes-specific tables if they don't exist. Safe
// to call repeatedly. Caller is the command boot path.
func EnsureSchema(s *store.Store) error {
	if s == nil {
		return fmt.Errorf("EnsureSchema: nil store")
	}
	db := s.DB()
	stmts := []string{
		// recipe_ingredients: one row per (recipe, ingredient line). Used by
		// pantry, with-ingredient, dietary. Recipe URL is the foreign key.
		`CREATE TABLE IF NOT EXISTS recipe_ingredients (
			recipe_url TEXT NOT NULL,
			position INTEGER NOT NULL,
			raw TEXT NOT NULL,
			qty REAL,
			unit TEXT,
			name TEXT NOT NULL,
			PRIMARY KEY (recipe_url, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_recipe_ingredients_name ON recipe_ingredients(name)`,
		// FTS5 index on ingredient names for fast token matches.
		`CREATE VIRTUAL TABLE IF NOT EXISTS recipe_ingredients_fts USING fts5(
			recipe_url, name, tokenize='unicode61 remove_diacritics 1'
		)`,
		// recipe_index: one row per recipe with the high-leverage scalar fields
		// needed for ranking, time-cap filtering, dietary, and category browse
		// without parsing the JSON blob.
		`CREATE TABLE IF NOT EXISTS recipe_index (
			url TEXT PRIMARY KEY,
			recipe_id TEXT,
			slug TEXT,
			name TEXT,
			total_time INTEGER,
			rating REAL,
			review_count INTEGER,
			category TEXT,
			cuisine TEXT,
			keywords TEXT,
			fetched_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_recipe_index_total_time ON recipe_index(total_time)`,
		`CREATE INDEX IF NOT EXISTS idx_recipe_index_rating ON recipe_index(rating)`,
		`CREATE INDEX IF NOT EXISTS idx_recipe_index_category ON recipe_index(category)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("ensure schema: %w (statement: %s)", err, firstLine(s))
		}
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i > 0 {
		return s[:i]
	}
	return s
}

// SaveRecipe persists a parsed Recipe into the store: the JSON payload via the
// generated UpsertRecipes path, plus the ingredient denormalization and the
// scalar index row. Idempotent.
func SaveRecipe(s *store.Store, r *Recipe) error {
	if s == nil || r == nil {
		return fmt.Errorf("SaveRecipe: nil arg")
	}
	id, slug := ParseURL(r.URL)
	// Marshal full Recipe. We add an `id` field the generator's UpsertRecipes
	// looks for as the row primary key (it expects either `id` or a known field).
	payload := map[string]any{
		"id":       r.URL,
		"url":      r.URL,
		"recipeId": id,
		"slug":     slug,
		"recipe":   r,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := s.UpsertRecipes(data); err != nil {
		return fmt.Errorf("upsert recipes: %w", err)
	}

	// Ingredient denormalization.
	parsed := ParseIngredients(r.RecipeIngredient)
	if err := upsertIngredients(s.DB(), r.URL, parsed); err != nil {
		return err
	}
	// Scalar index for fast queries.
	if err := upsertRecipeIndex(s.DB(), r, id, slug); err != nil {
		return err
	}
	return nil
}

func upsertIngredients(db *sql.DB, url string, items []ParsedIngredient) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM recipe_ingredients WHERE recipe_url = ?`, url); err != nil {
		return fmt.Errorf("delete recipe_ingredients: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM recipe_ingredients_fts WHERE recipe_url = ?`, url); err != nil {
		return fmt.Errorf("delete recipe_ingredients_fts: %w", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO recipe_ingredients (recipe_url, position, raw, qty, unit, name) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	ftsStmt, err := tx.Prepare(`INSERT INTO recipe_ingredients_fts (recipe_url, name) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare fts insert: %w", err)
	}
	defer ftsStmt.Close()

	for i, p := range items {
		if _, err := stmt.Exec(url, i, p.Raw, p.Quantity, p.Unit, p.Name); err != nil {
			return fmt.Errorf("insert recipe_ingredients[%d]: %w", i, err)
		}
		if _, err := ftsStmt.Exec(url, p.Name); err != nil {
			return fmt.Errorf("insert recipe_ingredients_fts[%d]: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func upsertRecipeIndex(db *sql.DB, r *Recipe, id, slug string) error {
	keywords := strings.Join(r.Keywords, ",")
	category := strings.Join(r.RecipeCategory, ",")
	cuisine := strings.Join(r.RecipeCuisine, ",")
	fetchedAt := r.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}
	_, err := db.Exec(`INSERT INTO recipe_index
		(url, recipe_id, slug, name, total_time, rating, review_count, category, cuisine, keywords, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			recipe_id=excluded.recipe_id,
			slug=excluded.slug,
			name=excluded.name,
			total_time=excluded.total_time,
			rating=excluded.rating,
			review_count=excluded.review_count,
			category=excluded.category,
			cuisine=excluded.cuisine,
			keywords=excluded.keywords,
			fetched_at=excluded.fetched_at`,
		r.URL, id, slug, r.Name, r.TotalTime, r.AggregateRating.Value, r.AggregateRating.Count,
		category, cuisine, keywords, fetchedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("upsert recipe_index: %w", err)
	}
	return nil
}

// IndexRow is a row from recipe_index, used by ranking/filter commands.
type IndexRow struct {
	URL         string  `json:"url"`
	RecipeID    string  `json:"recipeId,omitempty"`
	Slug        string  `json:"slug,omitempty"`
	Name        string  `json:"name"`
	TotalTime   int     `json:"totalTime,omitempty"`
	Rating      float64 `json:"rating,omitempty"`
	ReviewCount int     `json:"reviewCount,omitempty"`
	Category    string  `json:"category,omitempty"`
	Cuisine     string  `json:"cuisine,omitempty"`
	Keywords    string  `json:"keywords,omitempty"`
	FetchedAt   string  `json:"fetchedAt,omitempty"`
}

// QueryIndex runs a flexible SELECT against recipe_index.
type IndexQuery struct {
	MaxTime         int     // <= total_time filter; 0 = no filter
	MinRating       float64 // >= rating filter
	Category        string  // substring of category column
	Cuisine         string  // substring of cuisine column
	IngredientToken string  // FTS token; recipes that contain this ingredient
	Limit           int     // result limit; 0 → 50
	OrderBy         string  // "rating", "review_count", "fetched_at", "total_time"
	OrderDesc       bool
}

// QueryIndex runs an IndexQuery and returns the matching rows.
func QueryIndex(s *store.Store, q IndexQuery) ([]IndexRow, error) {
	if s == nil {
		return nil, fmt.Errorf("QueryIndex: nil store")
	}
	if q.Limit <= 0 {
		q.Limit = 50
	}
	conds := []string{}
	args := []any{}
	if q.MaxTime > 0 {
		conds = append(conds, "total_time > 0 AND total_time <= ?")
		args = append(args, q.MaxTime)
	}
	if q.MinRating > 0 {
		conds = append(conds, "rating >= ?")
		args = append(args, q.MinRating)
	}
	if q.Category != "" {
		conds = append(conds, "lower(category) LIKE ?")
		args = append(args, "%"+strings.ToLower(q.Category)+"%")
	}
	if q.Cuisine != "" {
		conds = append(conds, "lower(cuisine) LIKE ?")
		args = append(args, "%"+strings.ToLower(q.Cuisine)+"%")
	}
	join := ""
	if q.IngredientToken != "" {
		join = `JOIN recipe_ingredients_fts f ON f.recipe_url = recipe_index.url`
		conds = append(conds, "recipe_ingredients_fts MATCH ?")
		args = append(args, q.IngredientToken)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	order := "fetched_at DESC"
	switch q.OrderBy {
	case "rating", "review_count", "fetched_at", "total_time", "name":
		order = q.OrderBy
		if q.OrderDesc {
			order += " DESC"
		} else {
			order += " ASC"
		}
	}
	query := fmt.Sprintf(`SELECT DISTINCT recipe_index.url, recipe_index.recipe_id, recipe_index.slug, recipe_index.name, recipe_index.total_time, recipe_index.rating, recipe_index.review_count, recipe_index.category, recipe_index.cuisine, recipe_index.keywords, recipe_index.fetched_at FROM recipe_index %s %s ORDER BY recipe_index.%s LIMIT ?`, join, where, order)
	args = append(args, q.Limit)

	rows, err := s.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	out := []IndexRow{}
	for rows.Next() {
		var r IndexRow
		var totalTime, reviewCount sql.NullInt64
		var rating sql.NullFloat64
		var recipeID, slug, name, category, cuisine, keywords, fetchedAt sql.NullString
		if err := rows.Scan(&r.URL, &recipeID, &slug, &name, &totalTime, &rating, &reviewCount, &category, &cuisine, &keywords, &fetchedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		r.RecipeID = recipeID.String
		r.Slug = slug.String
		r.Name = name.String
		r.TotalTime = int(totalTime.Int64)
		r.Rating = rating.Float64
		r.ReviewCount = int(reviewCount.Int64)
		r.Category = category.String
		r.Cuisine = cuisine.String
		r.Keywords = keywords.String
		r.FetchedAt = fetchedAt.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// PantryMatch holds the result of a pantry query.
type PantryMatch struct {
	URL         string   `json:"url"`
	Name        string   `json:"name"`
	Have        []string `json:"have"`
	Missing     []string `json:"missing"`
	Score       float64  `json:"score"`
	TotalTime   int      `json:"totalTime,omitempty"`
	Rating      float64  `json:"rating,omitempty"`
	ReviewCount int      `json:"reviewCount,omitempty"`
}

// PantryQuery scores cached recipes by ingredient overlap with `pantry`. Only
// recipes whose ingredient set is at least `minOverlap` are returned.
//
// `pantry` is a list of normalized ingredient tokens (lowercase). The score
// is `len(have) / len(have+missing)` — a proportion between 0 and 1.
func PantryQuery(s *store.Store, pantry []string, minOverlap float64, queryFilter string, limit int) ([]PantryMatch, error) {
	if s == nil {
		return nil, fmt.Errorf("PantryQuery: nil store")
	}
	if limit <= 0 {
		limit = 25
	}
	pantrySet := map[string]bool{}
	for _, p := range pantry {
		t := strings.ToLower(strings.TrimSpace(p))
		if t != "" {
			pantrySet[t] = true
		}
	}
	if len(pantrySet) == 0 {
		return nil, fmt.Errorf("PantryQuery: empty pantry")
	}

	// Pull all recipes (or ones matching queryFilter on name).
	queryFilter = strings.ToLower(strings.TrimSpace(queryFilter))
	q := `SELECT url, name, total_time, rating, review_count FROM recipe_index`
	args := []any{}
	if queryFilter != "" {
		q += ` WHERE lower(name) LIKE ?`
		args = append(args, "%"+queryFilter+"%")
	}
	rows, err := s.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query recipe_index: %w", err)
	}
	defer rows.Close()

	type recipeStub struct {
		url, name string
		total     int
		rating    float64
		count     int
	}
	stubs := []recipeStub{}
	for rows.Next() {
		var rs recipeStub
		var totalTime, reviewCount sql.NullInt64
		var rating sql.NullFloat64
		var name sql.NullString
		if err := rows.Scan(&rs.url, &name, &totalTime, &rating, &reviewCount); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		rs.name = name.String
		rs.total = int(totalTime.Int64)
		rs.rating = rating.Float64
		rs.count = int(reviewCount.Int64)
		stubs = append(stubs, rs)
	}

	// For each recipe, fetch its ingredients and compute overlap.
	out := []PantryMatch{}
	for _, rs := range stubs {
		ings, err := loadIngredientNames(s.DB(), rs.url)
		if err != nil {
			continue
		}
		if len(ings) == 0 {
			continue
		}
		have := []string{}
		missing := []string{}
		for _, name := range ings {
			lc := strings.ToLower(name)
			matched := false
			// Token-level match: an ingredient is "had" if any of its tokens
			// appears in the pantry. "boneless skinless chicken thighs"
			// matches "chicken".
			for _, tok := range tokenize(lc) {
				if pantrySet[tok] {
					matched = true
					break
				}
			}
			if matched {
				have = append(have, name)
			} else {
				missing = append(missing, name)
			}
		}
		total := len(have) + len(missing)
		if total == 0 {
			continue
		}
		score := float64(len(have)) / float64(total)
		if score < minOverlap {
			continue
		}
		out = append(out, PantryMatch{
			URL: rs.url, Name: rs.name, Have: have, Missing: missing, Score: score,
			TotalTime: rs.total, Rating: rs.rating, ReviewCount: rs.count,
		})
	}

	// Sort by score desc, then by Bayesian-smoothed rating as tiebreak.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			if betterPantry(out[j], out[j-1]) {
				out[j], out[j-1] = out[j-1], out[j]
			} else {
				break
			}
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func betterPantry(a, b PantryMatch) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	bayesA := BayesianRating(a.Rating, a.ReviewCount, 4.0, 200)
	bayesB := BayesianRating(b.Rating, b.ReviewCount, 4.0, 200)
	return bayesA > bayesB
}

func loadIngredientNames(db *sql.DB, url string) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM recipe_ingredients WHERE recipe_url = ? ORDER BY position`, url)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// tokenize lowercases and splits on whitespace + comma + paren. Used for
// pantry matching: a recipe-side ingredient name like "boneless skinless
// chicken thighs" produces tokens ["boneless","skinless","chicken","thighs"].
func tokenize(s string) []string {
	repl := strings.NewReplacer(",", " ", "(", " ", ")", " ", "/", " ")
	s = repl.Replace(s)
	parts := strings.Fields(strings.ToLower(s))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 {
			out = append(out, p)
		}
	}
	return out
}

// CountRecipes returns how many recipes are cached locally.
func CountRecipes(s *store.Store) (int, error) {
	if s == nil {
		return 0, nil
	}
	row := s.DB().QueryRow(`SELECT COUNT(*) FROM recipe_index`)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// SearchCachedRecipes runs the FTS query against the generic resources table
// scoped to recipes, returning raw JSON payloads. Replaces the prior
// store.SearchRecipes method, kept here so we don't add files inside the
// generator-owned internal/store package.
func SearchCachedRecipes(s *store.Store, query string, limit int) ([]json.RawMessage, error) {
	if s == nil {
		return nil, fmt.Errorf("SearchCachedRecipes: nil store")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB().Query(
		`SELECT r.data FROM resources r
		 JOIN resources_fts f ON r.id = f.id
		 WHERE resources_fts MATCH ? AND r.type = 'recipes'
		 ORDER BY rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

// ClearCache wipes the local recipe cache (recipe_index, recipe_ingredients,
// recipe_ingredients_fts, and the generic resources rows for recipes).
func ClearCache(s *store.Store) error {
	if s == nil {
		return nil
	}
	stmts := []string{
		`DELETE FROM recipe_ingredients_fts`,
		`DELETE FROM recipe_ingredients`,
		`DELETE FROM recipe_index`,
		`DELETE FROM recipes`,
		`DELETE FROM resources WHERE type = 'recipes'`,
	}
	for _, s2 := range stmts {
		if _, err := s.DB().Exec(s2); err != nil {
			return fmt.Errorf("clear: %w (%s)", err, s2)
		}
	}
	return nil
}
