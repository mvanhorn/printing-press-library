// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/store"
)

type syncCounts struct {
	Apps     map[string]int
	Patterns int
	Elements int
	Screens  int
}

// PATCH: Add local SQLite sync for the offline Mobbin mirror.
func newSyncCmd(flags *rootFlags) *cobra.Command {
	var platformsCSV string
	var limitPerCategory int
	var noImages bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Mobbin apps, screens, patterns, and elements into the local SQLite store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			platforms := splitCSV(platformsCSV)
			if len(platforms) == 0 {
				return usageErr(fmt.Errorf("--platform must include at least one platform"))
			}
			if dbPath == "" {
				dbPath = defaultStorePath()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "dry run: would sync platforms=%s limitPerCategory=%d noImages=%t db=%s\n", strings.Join(platforms, ","), limitPerCategory, noImages, dbPath)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.Open(context.Background(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			counts := syncCounts{Apps: map[string]int{}}
			for _, platform := range platforms {
				n, screens, err := syncPlatform(cmd.Context(), c, db, platform, limitPerCategory)
				if err != nil {
					return err
				}
				counts.Apps[platform] += n
				counts.Screens += screens
			}
			patterns, elements, err := syncDictionary(cmd.Context(), c, db)
			if err != nil {
				return err
			}
			counts.Patterns = patterns
			counts.Elements = elements
			for _, platform := range platforms {
				fmt.Fprintf(cmd.ErrOrStderr(), "apps: %d synced (%s)\n", counts.Apps[platform], platform)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "screens: %d\npatterns: %d\nelements: %d\n", counts.Screens, counts.Patterns, counts.Elements)
			return nil
		},
	}
	cmd.Flags().StringVar(&platformsCSV, "platform", "web,ios", "Comma-separated platforms to sync: web, ios, android")
	cmd.Flags().IntVar(&limitPerCategory, "limit-per-category", 50, "Apps returned per popular category")
	cmd.Flags().BoolVar(&noImages, "no-images", false, "Skip image cache (accepted for forward compatibility)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	return cmd
}

type apiClient interface {
	Get(string, map[string]string) (json.RawMessage, error)
	Post(string, any) (json.RawMessage, int, error)
}

func syncPlatform(ctx context.Context, c apiClient, db *store.DB, platform string, limit int) (int, int, error) {
	apps := map[string]bool{}
	screenCount := 0
	data, err := c.Get("/api/searchable-apps/"+platform, map[string]string{})
	if err != nil {
		return 0, 0, classifyAPIError(err, nil)
	}
	n, s, err := upsertSyncItems(ctx, db, data, platform, apps)
	if err != nil {
		return 0, 0, err
	}
	screenCount += s

	data, _, err = c.Post("/api/popular-apps/fetch-popular-apps-with-preview-screens", map[string]any{"platform": platform, "limitPerCategory": limit})
	if err != nil {
		return 0, 0, classifyAPIError(err, nil)
	}
	n, s, err = upsertSyncItems(ctx, db, data, platform, apps)
	if err != nil {
		return 0, 0, err
	}
	screenCount += s
	_ = n

	data, _, err = c.Post("/api/discover/fetch-discover-page-apps", map[string]any{"tab": "latest", "platform": platform, "pageIndex": 0})
	if err != nil {
		return 0, 0, classifyAPIError(err, nil)
	}
	_, s, err = upsertSyncItems(ctx, db, data, platform, apps)
	if err != nil {
		return 0, 0, err
	}
	screenCount += s
	return len(apps), screenCount, nil
}

func syncDictionary(ctx context.Context, c apiClient, db *store.DB) (int, int, error) {
	data, _, err := c.Post("/api/filter-tags/fetch-dictionary-definitions", map[string]any{})
	if err != nil {
		return 0, 0, classifyAPIError(err, nil)
	}
	items := extractSyncItems(data)
	patterns, elements := 0, 0
	for _, item := range items {
		kind := strings.ToLower(firstSyncString(item, "type", "kind", "category"))
		switch {
		case strings.Contains(kind, "pattern"), item["definition"] != nil && item["slug"] != nil && strings.Contains(strings.ToLower(firstSyncString(item, "group")), "pattern"):
			if err := db.UpsertPattern(ctx, item); err == nil {
				patterns++
			}
		case strings.Contains(kind, "element"), item["definition"] != nil && item["slug"] != nil:
			if err := db.UpsertElement(ctx, item); err == nil {
				elements++
			}
		}
	}
	if patterns == 0 || elements == 0 {
		for _, key := range []string{"patterns", "screenPatterns"} {
			for _, item := range extractNamedArray(data, key) {
				if err := db.UpsertPattern(ctx, item); err == nil {
					patterns++
				}
			}
		}
		for _, key := range []string{"elements", "screenElements"} {
			for _, item := range extractNamedArray(data, key) {
				if err := db.UpsertElement(ctx, item); err == nil {
					elements++
				}
			}
		}
	}
	return patterns, elements, nil
}

func upsertSyncItems(ctx context.Context, db *store.DB, data json.RawMessage, platform string, seen map[string]bool) (int, int, error) {
	items := extractSyncItems(data)
	screenCount := 0
	for _, item := range items {
		if platform != "" && item["platform"] == nil {
			item["platform"] = platform
		}
		if looksLikeScreen(item) {
			if err := db.UpsertScreen(ctx, item); err == nil {
				screenCount++
			}
			continue
		}
		if id := firstSyncString(item, "id", "appId"); id != "" {
			if err := db.UpsertApp(ctx, item); err != nil {
				return len(seen), screenCount, err
			}
			seen[id] = true
		}
		for _, child := range nestedMaps(item, "screens", "previewScreens") {
			if platform != "" && child["platform"] == nil {
				child["platform"] = platform
			}
			if firstSyncString(child, "appId", "app_id") == "" {
				child["appId"] = firstSyncString(item, "id", "appId")
			}
			if err := db.UpsertScreen(ctx, child); err == nil {
				screenCount++
			}
		}
	}
	return len(seen), screenCount, nil
}

func extractSyncItems(data json.RawMessage) []map[string]any {
	if items := extractNamedArray(data, "value", "data"); len(items) > 0 {
		return items
	}
	if items := extractNamedArray(data, "data"); len(items) > 0 {
		return items
	}
	var direct []map[string]any
	if json.Unmarshal(data, &direct) == nil {
		return direct
	}
	return collectMaps(data)
}

func extractNamedArray(data json.RawMessage, path ...string) []map[string]any {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return nil
	}
	for _, key := range path {
		obj, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = obj[key]
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func collectMaps(data json.RawMessage) []map[string]any {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return nil
	}
	var out []map[string]any
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			if firstSyncString(t, "id", "appId", "screenId") != "" {
				out = append(out, t)
			}
			for _, v := range t {
				walk(v)
			}
		}
	}
	walk(v)
	return out
}

func nestedMaps(item map[string]any, keys ...string) []map[string]any {
	var out []map[string]any
	for _, key := range keys {
		if arr, ok := item[key].([]any); ok {
			for _, v := range arr {
				if m, ok := v.(map[string]any); ok {
					out = append(out, m)
				}
			}
		}
	}
	return out
}

func defaultStorePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "mobbin-pp-cli", "data.db")
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstSyncString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok {
			return s
		}
	}
	return ""
}

func looksLikeScreen(m map[string]any) bool {
	return firstSyncString(m, "screenId") != "" || firstSyncString(m, "imageUrl", "image_url", "ocrText", "ocr_text") != ""
}
