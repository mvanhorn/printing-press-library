// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/store"
)

// fieldString returns the first non-blank string value among keys.
func fieldString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// hasUTM reports whether link carries at least one utm_* query parameter.
func hasUTM(link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	for k := range u.Query() {
		if strings.HasPrefix(strings.ToLower(k), "utm_") {
			return true
		}
	}
	return false
}

// isHTTPURL reports whether s is an absolute http(s) URL with a host.
func isHTTPURL(s string) bool {
	u, err := url.ParseRequestURI(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

type localBoard struct {
	ID   string
	Name string
}

// loadSyncedPins returns synced Pins (flat and board-scoped rows), deduplicated
// by Pin id and ordered by id for deterministic output.
func loadSyncedPins(ctx context.Context, db *store.Store) ([]map[string]any, error) {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data FROM resources WHERE resource_type IN ('pins', 'boards_pins')`)
	if err != nil {
		return nil, fmt.Errorf("querying synced pins: %w", err)
	}
	raw := make([]string, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning pin row: %w", err)
		}
		raw = append(raw, data)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating pin rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	byID := make(map[string]map[string]any, len(raw))
	for _, data := range raw {
		var pin map[string]any
		if json.Unmarshal([]byte(data), &pin) != nil {
			continue
		}
		id := fieldString(pin, "id")
		if id == "" {
			continue
		}
		if _, dup := byID[id]; !dup {
			byID[id] = pin
		}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	pins := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		pins = append(pins, byID[id])
	}
	return pins, nil
}

// loadSyncedBoards returns synced boards from the typed boards table.
func loadSyncedBoards(ctx context.Context, db *store.Store) ([]localBoard, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT id, COALESCE(name, '') FROM boards ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("querying synced boards: %w", err)
	}
	boards := make([]localBoard, 0)
	for rows.Next() {
		var b localBoard
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning board row: %w", err)
		}
		boards = append(boards, b)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return boards, rows.Close()
}

// loadSyncedSections returns synced board sections as (id, name, board_id).
// Sections are optional in the local store, so a missing table yields none.
func loadSyncedSections(ctx context.Context, db *store.Store) []map[string]string {
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data FROM resources WHERE resource_type IN ('boards_sections', 'sections')`)
	if err != nil {
		return nil
	}
	raw := make([]string, 0)
	for rows.Next() {
		var data string
		if rows.Scan(&data) == nil {
			raw = append(raw, data)
		}
	}
	_ = rows.Close()
	out := make([]map[string]string, 0, len(raw))
	for _, data := range raw {
		var m map[string]any
		if json.Unmarshal([]byte(data), &m) != nil {
			continue
		}
		out = append(out, map[string]string{
			"id":       fieldString(m, "id"),
			"name":     fieldString(m, "name"),
			"board_id": fieldString(m, "board_id", "parent_id"),
		})
	}
	return out
}

var nowUTC = func() time.Time { return time.Now().UTC() }

func splitCSV(s string) []string {
	out := make([]string, 0)
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// containsString reports whether s is in list.
func containsString(list []string, s string) bool { return slices.Contains(list, s) }
