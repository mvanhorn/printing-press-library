// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Domain-specific write methods for the Mobbin mirror tables created in
// migrateExtras. They back the novel sync domain phase (sync_domain.go) that
// populates screens/flows/app_versions/patterns/etc. so bench/audit/drift have
// data to query. These are writes: no read-only guard, unlike RawQuery.

func (s *Store) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// UpsertApp writes into the generated apps table (which uses a `data` JSON
// column, not raw_json) and populates the slug column added by migrateExtras.
// On conflict it preserves the framework-synced `data` and `created_at`,
// updating only the scalar columns and slug the domain phase owns.
func (s *Store) UpsertApp(ctx context.Context, app map[string]any) error {
	id := firstStringX(app, "id", "appId")
	if id == "" {
		return fmt.Errorf("app id is required")
	}
	slug := firstStringX(app, "slug")
	if slug == "" {
		slug = appURLSlugX(firstStringX(app, "appName", "app_name", "name"), firstStringX(app, "platform"), id)
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO apps
(id, slug, app_name, platform, app_categories, thumbnail_url, created_at, updated_at, data, synced_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET slug=excluded.slug, app_name=excluded.app_name, platform=excluded.platform,
app_categories=excluded.app_categories, thumbnail_url=excluded.thumbnail_url, updated_at=excluded.updated_at,
synced_at=excluded.synced_at`,
			id, slug, firstStringX(app, "appName", "app_name", "name"), firstStringX(app, "platform"),
			jsonStringX(firstValueX(app, "appCategories", "app_categories", "categories")),
			firstStringX(app, "thumbnailUrl", "thumbnail_url", "iconUrl"),
			firstStringX(app, "createdAt", "created_at"), firstStringX(app, "updatedAt", "updated_at"),
			rawJSONX(app), nowX())
		return err
	})
}

func (s *Store) UpsertAppVersion(ctx context.Context, v map[string]any) error {
	id := firstStringX(v, "id", "appVersionId", "versionId")
	if id == "" {
		return fmt.Errorf("app version id is required")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO app_versions
(id, app_id, version, captured_at, raw_json, synced_at) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET app_id=excluded.app_id, version=excluded.version, captured_at=excluded.captured_at,
raw_json=excluded.raw_json, synced_at=excluded.synced_at`,
			id, firstStringX(v, "appId", "app_id"), firstStringX(v, "version"),
			firstStringX(v, "capturedAt", "captured_at"), rawJSONX(v), nowX())
		return err
	})
}

func (s *Store) UpsertScreen(ctx context.Context, sc map[string]any) error {
	id := firstStringX(sc, "id", "screenId")
	if id == "" {
		return fmt.Errorf("screen id is required")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO screens
(id, app_id, app_version_id, flow_id, platform, image_url, image_url_full, ocr_text, raw_json, captured_at, synced_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET app_id=excluded.app_id, app_version_id=excluded.app_version_id, flow_id=excluded.flow_id,
platform=excluded.platform, image_url=excluded.image_url, image_url_full=excluded.image_url_full, ocr_text=excluded.ocr_text,
raw_json=excluded.raw_json, captured_at=excluded.captured_at, synced_at=excluded.synced_at`,
			id, firstStringX(sc, "appId", "app_id"), firstStringX(sc, "appVersionId", "app_version_id"),
			firstStringX(sc, "flowId", "flow_id"), firstStringX(sc, "platform"),
			firstStringX(sc, "imageUrl", "image_url"), firstStringX(sc, "imageUrlFull", "image_url_full", "fullImageUrl"),
			firstStringX(sc, "ocrText", "ocr_text", "text"), rawJSONX(sc),
			firstStringX(sc, "capturedAt", "captured_at"), nowX())
		return err
	})
}

func (s *Store) UpsertFlow(ctx context.Context, f map[string]any) error {
	id := firstStringX(f, "id", "flowId")
	if id == "" {
		return fmt.Errorf("flow id is required")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO flows
(id, app_id, app_version_id, name, flow_actions, screen_ids, step_count, platform, raw_json, captured_at, synced_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET app_id=excluded.app_id, app_version_id=excluded.app_version_id, name=excluded.name,
flow_actions=excluded.flow_actions, screen_ids=excluded.screen_ids, step_count=excluded.step_count, platform=excluded.platform,
raw_json=excluded.raw_json, captured_at=excluded.captured_at, synced_at=excluded.synced_at`,
			id, firstStringX(f, "appId", "app_id"), firstStringX(f, "appVersionId", "app_version_id"),
			firstStringX(f, "name"), jsonStringX(firstValueX(f, "flowActions", "flow_actions", "actions")),
			jsonStringX(firstValueX(f, "screenIds", "screen_ids")), firstIntX(f, "stepCount", "step_count"),
			firstStringX(f, "platform"), rawJSONX(f), firstStringX(f, "capturedAt", "captured_at"), nowX())
		return err
	})
}

func (s *Store) UpsertPattern(ctx context.Context, p map[string]any) error {
	return s.upsertDictionary(ctx, "patterns", p)
}

func (s *Store) UpsertElement(ctx context.Context, e map[string]any) error {
	return s.upsertDictionary(ctx, "elements", e)
}

func (s *Store) UpsertFlowAction(ctx context.Context, a map[string]any) error {
	return s.upsertDictionary(ctx, "flow_actions", a)
}

func (s *Store) upsertDictionary(ctx context.Context, table string, item map[string]any) error {
	id := firstStringX(item, "id", "slug")
	if id == "" {
		return fmt.Errorf("%s id or slug is required", table)
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s
(id, slug, name, category, definition, platform, raw_json, synced_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET slug=excluded.slug, name=excluded.name, category=excluded.category,
definition=excluded.definition, platform=excluded.platform, raw_json=excluded.raw_json, synced_at=excluded.synced_at`, table),
			id, firstStringX(item, "slug"), firstStringX(item, "name", "label", "displayName"),
			firstStringX(item, "category"), firstStringX(item, "definition", "description"),
			firstStringX(item, "platform"), rawJSONX(item), nowX())
		return err
	})
}

func (s *Store) UpsertScreenPattern(ctx context.Context, screenID, patternSlug string) error {
	if screenID == "" || patternSlug == "" {
		return fmt.Errorf("screen id and pattern slug are required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO screen_patterns(screen_id, pattern_slug) VALUES (?, ?)`, screenID, patternSlug)
	return err
}

func (s *Store) UpsertScreenElement(ctx context.Context, screenID, elementSlug string) error {
	if screenID == "" || elementSlug == "" {
		return fmt.Errorf("screen id and element slug are required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO screen_elements(screen_id, element_slug) VALUES (?, ?)`, screenID, elementSlug)
	return err
}

func (s *Store) UpsertCollection(ctx context.Context, c map[string]any) error {
	id := firstStringX(c, "id", "collectionId")
	if id == "" {
		return fmt.Errorf("collection id is required")
	}
	return s.withTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO collections
(id, workspace_id, name, description, created_at, raw_json, synced_at) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET workspace_id=excluded.workspace_id, name=excluded.name, description=excluded.description,
created_at=excluded.created_at, raw_json=excluded.raw_json, synced_at=excluded.synced_at`,
			id, firstStringX(c, "workspaceId", "workspace_id"), firstStringX(c, "name"), firstStringX(c, "description"),
			firstStringX(c, "createdAt", "created_at"), rawJSONX(c), nowX())
		return err
	})
}

// TableCount returns the row count for a Mobbin domain table, used by the
// sync domain phase to print a per-table summary.
func (s *Store) TableCount(ctx context.Context, table string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n)
	return n, err
}

// --- value helpers (X-suffixed to avoid clashing with the generated store) ---

func firstValueX(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstStringX(m map[string]any, keys ...string) string {
	switch v := firstValueX(m, keys...).(type) {
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

func firstIntX(m map[string]any, keys ...string) int {
	switch v := firstValueX(m, keys...).(type) {
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

func jsonStringX(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func rawJSONX(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func nowX() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func appURLSlugX(name, platform, id string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
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
