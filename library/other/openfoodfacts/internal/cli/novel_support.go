// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written support for the novel (transcendence) commands: diary, compare,
// swap, recipe, allergens, budget, rank. Shared parsing, nutriment math, and
// local-store schema live here.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/openfoodfacts/internal/store"
)

// nutriments holds the per-100g (or scaled) macro values we care about.
type nutriments struct {
	Kcal    float64 `json:"kcal"`
	Protein float64 `json:"protein_g"`
	Fat     float64 `json:"fat_g"`
	SatFat  float64 `json:"saturated_fat_g"`
	Carbs   float64 `json:"carbs_g"`
	Sugars  float64 `json:"sugars_g"`
	Fiber   float64 `json:"fiber_g"`
	Salt    float64 `json:"salt_g"`
	Sodium  float64 `json:"sodium_g"`
}

// scaled returns the nutriments multiplied by factor (e.g. grams/100).
func (n nutriments) scaled(factor float64) nutriments {
	return nutriments{
		Kcal:    n.Kcal * factor,
		Protein: n.Protein * factor,
		Fat:     n.Fat * factor,
		SatFat:  n.SatFat * factor,
		Carbs:   n.Carbs * factor,
		Sugars:  n.Sugars * factor,
		Fiber:   n.Fiber * factor,
		Salt:    n.Salt * factor,
		Sodium:  n.Sodium * factor,
	}
}

// add returns the element-wise sum of two nutriment sets.
func (n nutriments) add(o nutriments) nutriments {
	return nutriments{
		Kcal:    n.Kcal + o.Kcal,
		Protein: n.Protein + o.Protein,
		Fat:     n.Fat + o.Fat,
		SatFat:  n.SatFat + o.SatFat,
		Carbs:   n.Carbs + o.Carbs,
		Sugars:  n.Sugars + o.Sugars,
		Fiber:   n.Fiber + o.Fiber,
		Salt:    n.Salt + o.Salt,
		Sodium:  n.Sodium + o.Sodium,
	}
}

// coerceFloat best-effort converts an arbitrary JSON value to float64.
func coerceFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	}
	return 0
}

// prodString reads a top-level string field from a product object.
func prodString(prod map[string]any, key string) string {
	if v, ok := prod[key]; ok {
		switch x := v.(type) {
		case string:
			return x
		case float64:
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
	}
	return ""
}

// prodFloat reads a top-level numeric field from a product object.
func prodFloat(prod map[string]any, key string) float64 {
	if v, ok := prod[key]; ok {
		return coerceFloat(v)
	}
	return 0
}

// prodTags returns the string members of a *_tags array, with the leading
// language prefix ("en:") stripped for display/matching.
func prodTags(prod map[string]any, key string) []string {
	raw, ok := prod[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			continue
		}
		if i := strings.IndexByte(s, ':'); i >= 0 && i < 5 {
			s = s[i+1:]
		}
		s = strings.TrimSpace(strings.ToLower(s))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// nutrFromObject extracts per-100g macros from a product's nutriments map.
func nutrFromObject(prod map[string]any) nutriments {
	nm, _ := prod["nutriments"].(map[string]any)
	get := func(k string) float64 {
		if nm == nil {
			return 0
		}
		return coerceFloat(nm[k])
	}
	return nutriments{
		Kcal:    get("energy-kcal_100g"),
		Protein: get("proteins_100g"),
		Fat:     get("fat_100g"),
		SatFat:  get("saturated-fat_100g"),
		Carbs:   get("carbohydrates_100g"),
		Sugars:  get("sugars_100g"),
		Fiber:   get("fiber_100g"),
		Salt:    get("salt_100g"),
		Sodium:  get("sodium_100g"),
	}
}

// servingGrams returns the serving size in grams if the product declares one.
func servingGrams(prod map[string]any) (float64, bool) {
	if g := prodFloat(prod, "serving_quantity"); g > 0 {
		return g, true
	}
	return 0, false
}

// fetchProduct fetches a product by barcode. found=false when OFF reports
// status 0 (no such product).
func fetchProduct(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, code string) (prod map[string]any, found bool, err error) {
	data, err := c.Get(ctx, "/api/v2/product/"+code+".json", nil)
	if err != nil {
		return nil, false, err
	}
	var envelope struct {
		Status  int            `json:"status"`
		Product map[string]any `json:"product"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, false, fmt.Errorf("parsing product %s: %w", code, err)
	}
	if envelope.Status == 0 || envelope.Product == nil {
		return nil, false, nil
	}
	return envelope.Product, true, nil
}

// offSearch runs an OFF v2 search and returns the products array.
func offSearch(ctx context.Context, c interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}, params map[string]string) ([]map[string]any, error) {
	data, err := c.Get(ctx, "/api/v2/search", params)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Products []map[string]any `json:"products"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing search results: %w", err)
	}
	return envelope.Products, nil
}

// friendlyNutrientKey maps a user-facing nutrient name to the OFF per-100g key.
func friendlyNutrientKey(name string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "kcal", "energy", "calories":
		return "energy-kcal_100g", true
	case "protein", "proteins":
		return "proteins_100g", true
	case "fat":
		return "fat_100g", true
	case "satfat", "saturated-fat", "saturated_fat", "saturatedfat":
		return "saturated-fat_100g", true
	case "carbs", "carbohydrates", "carbohydrate":
		return "carbohydrates_100g", true
	case "sugar", "sugars":
		return "sugars_100g", true
	case "fiber", "fibre":
		return "fiber_100g", true
	case "salt":
		return "salt_100g", true
	case "sodium":
		return "sodium_100g", true
	}
	return "", false
}

// isProductNotFound reports whether a product-endpoint response is an Open
// Food Facts "not found" envelope ({"status":0}). OFF returns HTTP 200 for
// unknown barcodes, so callers must inspect the body to detect a miss.
func isProductNotFound(data json.RawMessage) bool {
	var env struct {
		Status *float64 `json:"status"`
	}
	if json.Unmarshal(data, &env) != nil {
		return false
	}
	return env.Status != nil && *env.Status == 0
}

// parsePositiveFloat parses a strictly-positive float flag value.
func parsePositiveFloat(s, name string) (float64, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid --%s %q: must be a number", name, s)
	}
	if f <= 0 {
		return 0, fmt.Errorf("invalid --%s %q: must be greater than zero", name, s)
	}
	return f, nil
}

// resolveDBPath returns the explicit --db value or the default location.
func resolveDBPath(dbFlag string) string {
	if strings.TrimSpace(dbFlag) != "" {
		return dbFlag
	}
	return defaultDBPath("openfoodfacts-pp-cli")
}

// openLocalStore opens the local SQLite store and ensures the novel-feature
// tables (diary_entry, diary_goal, pref_kv) exist.
func openLocalStore(ctx context.Context, dbFlag string) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, resolveDBPath(dbFlag))
	if err != nil {
		return nil, err
	}
	if err := ensureNovelSchema(ctx, s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// ensureNovelSchema creates the local-only tables used by diary/goal/allergens.
func ensureNovelSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS diary_entry (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			day TEXT NOT NULL,
			code TEXT,
			name TEXT,
			grams REAL,
			kcal REAL, protein REAL, fat REAL, satfat REAL,
			carbs REAL, sugars REAL, fiber REAL, salt REAL, sodium REAL,
			created_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_diary_entry_day ON diary_entry(day)`,
		`CREATE TABLE IF NOT EXISTS diary_goal (
			id INTEGER PRIMARY KEY CHECK(id=1),
			kcal REAL, protein REAL, fat REAL, carbs REAL
		)`,
		`CREATE TABLE IF NOT EXISTS pref_kv (k TEXT PRIMARY KEY, v TEXT)`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring novel schema: %w", err)
		}
	}
	return nil
}

// loadGoal reads the single diary_goal row. ok=false when no goal is set.
func loadGoal(ctx context.Context, db *sql.DB) (g nutriments, ok bool, err error) {
	row := db.QueryRowContext(ctx, `SELECT kcal, protein, fat, carbs FROM diary_goal WHERE id=1`)
	var kcal, protein, fat, carbs sql.NullFloat64
	switch err := row.Scan(&kcal, &protein, &fat, &carbs); err {
	case sql.ErrNoRows:
		return nutriments{}, false, nil
	case nil:
		return nutriments{Kcal: kcal.Float64, Protein: protein.Float64, Fat: fat.Float64, Carbs: carbs.Float64}, true, nil
	default:
		return nutriments{}, false, err
	}
}

// wantsJSONOut reports whether output should be machine JSON (explicit --json
// or a non-terminal stdout without another explicit format flag).
func wantsJSONOut(cmd *cobra.Command, flags *rootFlags) bool {
	if flags.asJSON {
		return true
	}
	return !isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain
}
