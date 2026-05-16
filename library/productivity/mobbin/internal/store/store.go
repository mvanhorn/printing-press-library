// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct{ *sql.DB }

// PATCH: Add a pure-Go SQLite store for offline Mobbin search.
func Open(ctx context.Context, path string) (*DB, error) {
	if path == "" {
		return nil, fmt.Errorf("store path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating store directory: %w", err)
	}
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}
	db := &DB{DB: sqlDB}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error {
	if db == nil || db.DB == nil {
		return nil
	}
	return db.DB.Close()
}

func (db *DB) UpsertApp(ctx context.Context, app map[string]any) error {
	id := firstString(app, "id", "appId")
	if id == "" {
		return fmt.Errorf("app id is required")
	}
	slug := firstString(app, "slug")
	if slug == "" {
		slug = appURLSlug(firstString(app, "appName", "app_name", "name"), firstString(app, "platform"), id)
	}
	raw, synced := rawJSON(app), now()
	return db.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO apps
(id, slug, app_name, platform, app_categories, thumbnail_url, latest_version_id, created_at, updated_at, raw_json, synced_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET slug=excluded.slug, app_name=excluded.app_name, platform=excluded.platform,
app_categories=excluded.app_categories, thumbnail_url=excluded.thumbnail_url, latest_version_id=excluded.latest_version_id,
created_at=excluded.created_at, updated_at=excluded.updated_at, raw_json=excluded.raw_json, synced_at=excluded.synced_at`,
			id, slug, firstString(app, "appName", "app_name", "name"), firstString(app, "platform"),
			jsonString(firstValue(app, "appCategories", "app_categories", "categories")), firstString(app, "thumbnailUrl", "thumbnail_url", "iconUrl"),
			firstString(app, "latestVersionId", "latest_version_id"), firstString(app, "createdAt", "created_at"),
			firstString(app, "updatedAt", "updated_at"), raw, synced)
		return err
	})
}

func (db *DB) UpsertAppVersion(ctx context.Context, v map[string]any) error {
	id := firstString(v, "id", "appVersionId", "versionId")
	if id == "" {
		return fmt.Errorf("app version id is required")
	}
	return db.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO app_versions
(id, app_id, version, captured_at, raw_json, synced_at) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET app_id=excluded.app_id, version=excluded.version, captured_at=excluded.captured_at,
raw_json=excluded.raw_json, synced_at=excluded.synced_at`,
			id, firstString(v, "appId", "app_id"), firstString(v, "version"), firstString(v, "capturedAt", "captured_at"), rawJSON(v), now())
		return err
	})
}

func (db *DB) UpsertScreen(ctx context.Context, s map[string]any) error {
	id := firstString(s, "id", "screenId")
	if id == "" {
		return fmt.Errorf("screen id is required")
	}
	return db.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO screens
(id, app_id, app_version_id, flow_id, platform, image_url, image_url_full, ocr_text, raw_json, captured_at, synced_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET app_id=excluded.app_id, app_version_id=excluded.app_version_id, flow_id=excluded.flow_id,
platform=excluded.platform, image_url=excluded.image_url, image_url_full=excluded.image_url_full, ocr_text=excluded.ocr_text,
raw_json=excluded.raw_json, captured_at=excluded.captured_at, synced_at=excluded.synced_at`,
			id, firstString(s, "appId", "app_id"), firstString(s, "appVersionId", "app_version_id"), firstString(s, "flowId", "flow_id"),
			firstString(s, "platform"), firstString(s, "imageUrl", "image_url"), firstString(s, "imageUrlFull", "image_url_full", "fullImageUrl"),
			firstString(s, "ocrText", "ocr_text", "text"), rawJSON(s), firstString(s, "capturedAt", "captured_at"), now())
		return err
	})
}

func (db *DB) UpsertFlow(ctx context.Context, f map[string]any) error {
	id := firstString(f, "id", "flowId")
	if id == "" {
		return fmt.Errorf("flow id is required")
	}
	return db.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO flows
(id, app_id, app_version_id, name, flow_actions, screen_ids, step_count, platform, raw_json, captured_at, synced_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET app_id=excluded.app_id, app_version_id=excluded.app_version_id, name=excluded.name,
flow_actions=excluded.flow_actions, screen_ids=excluded.screen_ids, step_count=excluded.step_count, platform=excluded.platform,
raw_json=excluded.raw_json, captured_at=excluded.captured_at, synced_at=excluded.synced_at`,
			id, firstString(f, "appId", "app_id"), firstString(f, "appVersionId", "app_version_id"), firstString(f, "name"),
			jsonString(firstValue(f, "flowActions", "flow_actions", "actions")), jsonString(firstValue(f, "screenIds", "screen_ids")),
			firstInt(f, "stepCount", "step_count"), firstString(f, "platform"), rawJSON(f), firstString(f, "capturedAt", "captured_at"), now())
		return err
	})
}

func (db *DB) UpsertPattern(ctx context.Context, p map[string]any) error {
	return db.upsertDictionary(ctx, "patterns", p)
}

func (db *DB) UpsertElement(ctx context.Context, e map[string]any) error {
	return db.upsertDictionary(ctx, "elements", e)
}

func (db *DB) UpsertFlowAction(ctx context.Context, a map[string]any) error {
	return db.upsertDictionary(ctx, "flow_actions", a)
}

func (db *DB) UpsertScreenPattern(ctx context.Context, screenID, patternSlug string) error {
	if screenID == "" || patternSlug == "" {
		return fmt.Errorf("screen id and pattern slug are required")
	}
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO screen_patterns(screen_id, pattern_slug) VALUES (?, ?)`, screenID, patternSlug)
	return err
}

func (db *DB) UpsertScreenElement(ctx context.Context, screenID, elementSlug string) error {
	if screenID == "" || elementSlug == "" {
		return fmt.Errorf("screen id and element slug are required")
	}
	_, err := db.ExecContext(ctx, `INSERT OR IGNORE INTO screen_elements(screen_id, element_slug) VALUES (?, ?)`, screenID, elementSlug)
	return err
}

func (db *DB) UpsertCollection(ctx context.Context, c map[string]any) error {
	id := firstString(c, "id", "collectionId")
	if id == "" {
		return fmt.Errorf("collection id is required")
	}
	return db.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO collections
(id, workspace_id, name, description, created_at, raw_json, synced_at) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET workspace_id=excluded.workspace_id, name=excluded.name, description=excluded.description,
created_at=excluded.created_at, raw_json=excluded.raw_json, synced_at=excluded.synced_at`,
			id, firstString(c, "workspaceId", "workspace_id"), firstString(c, "name"), firstString(c, "description"),
			firstString(c, "createdAt", "created_at"), rawJSON(c), now())
		return err
	})
}

func (db *DB) SearchApps(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	return db.search(ctx, "app", `SELECT 'app' AS entity, apps.id, apps.slug, apps.app_name, apps.platform, apps.raw_json, bm25(apps_fts) AS rank
FROM apps_fts JOIN apps ON apps_fts.rowid = apps.rowid WHERE apps_fts MATCH ? ORDER BY rank LIMIT ?`, query, limit)
}

func (db *DB) SearchScreens(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	return db.search(ctx, "screen", `SELECT 'screen' AS entity, screens.id, screens.app_id, screens.platform, screens.image_url, screens.ocr_text, screens.raw_json, bm25(screens_fts) AS rank
FROM screens_fts JOIN screens ON screens_fts.rowid = screens.rowid WHERE screens_fts MATCH ? ORDER BY rank LIMIT ?`, query, limit)
}

func (db *DB) SearchFlows(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	return db.search(ctx, "flow", `SELECT 'flow' AS entity, flows.id, flows.app_id, flows.name, flows.platform, flows.raw_json, bm25(flows_fts) AS rank
FROM flows_fts JOIN flows ON flows_fts.rowid = flows.rowid WHERE flows_fts MATCH ? ORDER BY rank LIMIT ?`, query, limit)
}

func (db *DB) SearchAll(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	apps, err := db.SearchApps(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	screens, err := db.SearchScreens(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	flows, err := db.SearchFlows(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	rows := append(append(apps, screens...), flows...)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (db *DB) RawQuery(ctx context.Context, sqlText string) ([]map[string]any, error) {
	token := firstSQLToken(sqlText)
	if token != "SELECT" && token != "WITH" && token != "EXPLAIN" {
		return nil, fmt.Errorf("only SELECT, WITH, and EXPLAIN queries are allowed")
	}
	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (db *DB) upsertDictionary(ctx context.Context, table string, item map[string]any) error {
	id := firstString(item, "id", "slug")
	if id == "" {
		return fmt.Errorf("%s id or slug is required", table)
	}
	return db.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s
(id, slug, name, category, definition, platform, raw_json, synced_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET slug=excluded.slug, name=excluded.name, category=excluded.category,
definition=excluded.definition, platform=excluded.platform, raw_json=excluded.raw_json, synced_at=excluded.synced_at`, table),
			id, firstString(item, "slug"), firstString(item, "name", "label", "displayName"), firstString(item, "category"),
			firstString(item, "definition", "description"), firstString(item, "platform"), rawJSON(item), now())
		return err
	})
}

func (db *DB) search(ctx context.Context, entity, sqlText, query string, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx, sqlText, query, limit)
	if err != nil {
		return nil, fmt.Errorf("searching %s: %w", entity, err)
	}
	defer rows.Close()
	return scanRows(rows)
}

func (db *DB) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// RebuildFTS rebuilds the FTS5 indexes from their external content tables.
// Call once after a sync batch completes — rebuilding per row is O(n²).
func (db *DB) RebuildFTS(ctx context.Context) error {
	for _, ftsTable := range []string{"apps_fts", "screens_fts", "flows_fts"} {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s(%s) VALUES('rebuild')`, ftsTable, ftsTable)); err != nil {
			return fmt.Errorf("rebuilding %s: %w", ftsTable, err)
		}
	}
	return nil
}

func scanRows(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, col := range cols {
			switch v := vals[i].(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func firstValue(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstString(m map[string]any, keys ...string) string {
	switch v := firstValue(m, keys...).(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func firstInt(m map[string]any, keys ...string) int {
	switch v := firstValue(m, keys...).(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func jsonString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func rawJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func appURLSlug(name, platform, id string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if platform != "" {
		if slug != "" {
			slug += "-"
		}
		slug += platform
	}
	if id != "" {
		if slug != "" {
			slug += "-"
		}
		slug += id
	}
	return slug
}

func firstSQLToken(sqlText string) string {
	fields := strings.Fields(strings.TrimSpace(sqlText))
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}
