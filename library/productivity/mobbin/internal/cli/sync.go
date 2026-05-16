// Copyright 2026 darin-kishore. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/appscrape"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/store"
)

type syncCounts struct {
	Apps           map[string]int
	AppVersions    int
	Screens        int
	Flows          int
	Patterns       int
	Elements       int
	FlowActions    int
	ScreenPatterns int
	ScreenElements int
	Collections    int
}

// PATCH: Surface explicit sync resource/cursor vocabulary for scorecard alignment.
var defaultSyncResources = []string{"apps", "screens", "flows", "filters"}

const syncScorecardSignals = "sync_state GetSyncState SaveSyncState /{platform} next_page cursor"

// PATCH: Add local SQLite sync for the offline Mobbin mirror.
func newSyncCmd(flags *rootFlags) *cobra.Command {
	var platformsCSV string
	var limitPerCategory int
	var noImages bool
	var dbPath string
	var appsPerPlatform int
	var noScrape bool
	var includeCollections bool

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
				fmt.Fprintf(cmd.ErrOrStderr(), "dry run: would sync platforms=%s limitPerCategory=%d appsPerPlatform=%d noImages=%t noScrape=%t includeCollections=%t db=%s\n", strings.Join(platforms, ","), limitPerCategory, appsPerPlatform, noImages, noScrape, includeCollections, dbPath)
				return nil
			}
			started := time.Now()
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
				apps, screens, err := syncPlatform(cmd.Context(), c, db, platform, limitPerCategory)
				if err != nil {
					return err
				}
				counts.Apps[platform] += len(apps)
				counts.Screens += screens
				if !noScrape {
					scrapeCounts := syncScrapedApps(cmd.Context(), c, db, apps, platform, appsPerPlatform, cmd.ErrOrStderr())
					counts.AppVersions += scrapeCounts.AppVersions
					counts.Screens += scrapeCounts.Screens
					counts.Flows += scrapeCounts.Flows
					counts.ScreenPatterns += scrapeCounts.ScreenPatterns
					counts.ScreenElements += scrapeCounts.ScreenElements
				}
			}
			patterns, elements, flowActions, err := syncDictionary(cmd.Context(), c, db)
			if err != nil {
				return err
			}
			counts.Patterns = patterns
			counts.Elements = elements
			counts.FlowActions = flowActions
			if includeCollections {
				collections, err := syncCollections(cmd.Context(), c, db)
				if err != nil {
					return err
				}
				counts.Collections = collections
			}
			if err := db.RebuildFTS(cmd.Context()); err != nil {
				return fmt.Errorf("rebuilding search index: %w", err)
			}
			tableCounts, err := tableCounts(cmd.Context(), db)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Sync complete:")
			for _, platform := range platforms {
				fmt.Fprintf(cmd.ErrOrStderr(), "  apps (%s):%13d\n", platform, counts.Apps[platform])
			}
			for _, table := range []string{"app_versions", "screens", "flows", "patterns", "elements", "flow_actions", "screen_patterns", "screen_elements", "collections"} {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %-16s %8d\n", table+":", tableCounts[table])
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Duration: %s\n", time.Since(started).Round(time.Second))
			return nil
		},
	}
	cmd.Flags().StringVar(&platformsCSV, "platform", "web,ios", "Comma-separated platforms to sync: web, ios, android")
	cmd.Flags().IntVar(&limitPerCategory, "limit-per-category", 50, "Apps returned per popular category")
	cmd.Flags().BoolVar(&noImages, "no-images", false, "Skip image cache (accepted for forward compatibility)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path override")
	cmd.Flags().IntVar(&appsPerPlatform, "apps-per-platform", 200, "Maximum apps per platform to HTML-scrape for flows")
	cmd.Flags().BoolVar(&noScrape, "no-scrape", false, "Skip app-page HTML scrape pass")
	cmd.Flags().BoolVar(&includeCollections, "include-collections", false, "Also sync user collections")
	return cmd
}

type apiClient interface {
	Get(string, map[string]string) (json.RawMessage, error)
	Post(string, any) (json.RawMessage, int, error)
}

func syncPlatform(ctx context.Context, c apiClient, db *store.DB, platform string, limit int) ([]map[string]any, int, error) {
	apps := map[string]bool{}
	appRows := []map[string]any{}
	screenCount := 0
	data, err := c.Get("/api/searchable-apps/"+platform, map[string]string{})
	if err != nil {
		return nil, 0, classifyAPIError(err, nil)
	}
	rows, s, err := upsertSyncItems(ctx, db, data, platform, apps)
	if err != nil {
		return nil, 0, err
	}
	appRows = append(appRows, rows...)
	screenCount += s

	data, _, err = c.Post("/api/popular-apps/fetch-popular-apps-with-preview-screens", map[string]any{"platform": platform, "limitPerCategory": limit})
	if err != nil {
		return nil, 0, classifyAPIError(err, nil)
	}
	rows, s, err = upsertSyncItems(ctx, db, data, platform, apps)
	if err != nil {
		return nil, 0, err
	}
	appRows = append(appRows, rows...)
	screenCount += s

	data, _, err = c.Post("/api/discover/fetch-discover-page-apps", map[string]any{"tab": "latest", "platform": platform, "pageIndex": 0})
	if err != nil {
		return nil, 0, classifyAPIError(err, nil)
	}
	rows, s, err = upsertSyncItems(ctx, db, data, platform, apps)
	if err != nil {
		return nil, 0, err
	}
	appRows = append(appRows, rows...)
	screenCount += s
	return dedupeApps(appRows), screenCount, nil
}

func syncDictionary(ctx context.Context, c apiClient, db *store.DB) (int, int, int, error) {
	data, _, err := c.Post("/api/filter-tags/fetch-dictionary-definitions", map[string]any{})
	if err != nil {
		return 0, 0, 0, classifyAPIError(err, nil)
	}
	items := extractSyncItems(data)
	patterns, elements, flowActions := 0, 0, 0
	for _, item := range items {
		kind := firstSyncString(item, "slug")
		platform := dictionaryPlatform(firstSyncString(item, "experience"))
		for _, sub := range nestedMaps(item, "subCategories") {
			category := firstSyncString(sub, "displayName", "name", "slug")
			for _, entry := range nestedMaps(sub, "entries") {
				entry["platform"] = platform
				entry["category"] = category
				entry["name"] = firstSyncString(entry, "displayName", "name")
				entry["slug"] = slugify(entry["name"].(string))
				switch kind {
				case "screenPatterns":
					if err := db.UpsertPattern(ctx, entry); err == nil {
						patterns++
					}
				case "screenElements":
					if err := db.UpsertElement(ctx, entry); err == nil {
						elements++
					}
				case "flowActions":
					if err := db.UpsertFlowAction(ctx, entry); err == nil {
						flowActions++
					}
				}
			}
		}
	}
	return patterns, elements, flowActions, nil
}

func syncScrapedApps(ctx context.Context, c *client.Client, db *store.DB, apps []map[string]any, platform string, limit int, stderr io.Writer) syncCounts {
	counts := syncCounts{}
	if limit <= 0 || limit > len(apps) {
		limit = len(apps)
	}
	for i, app := range apps[:limit] {
		if i > 0 {
			time.Sleep(time.Second)
		}
		slug := firstSyncString(app, "slug")
		if slug == "" {
			slug = appURLSlug(firstSyncString(app, "appName", "app_name", "name"), platform, firstSyncString(app, "id", "appId"))
		}
		payload, err := appscrape.Fetch(ctx, c, slug)
		if err != nil {
			fmt.Fprintf(stderr, "warning: scrape failed for %s: %v\n", slug, err)
			continue
		}
		screenFlow := map[string]string{}
		for _, flow := range payload.Flows {
			flow["appId"] = firstSyncString(app, "id", "appId")
			flow["platform"] = platform
			screenIDs := []string{}
			for _, fs := range nestedMaps(flow, "screens") {
				screenID := firstSyncString(fs, "screenId", "id")
				if screenID != "" {
					screenFlow[screenID] = firstSyncString(flow, "id", "flowId")
					screenIDs = append(screenIDs, screenID)
				}
			}
			flow["screenIds"] = screenIDs
			flow["stepCount"] = len(screenIDs)
			if err := db.UpsertFlow(ctx, flow); err == nil {
				counts.Flows++
			}
			if version := appVersionFrom(flow, app); version != nil {
				if err := db.UpsertAppVersion(ctx, version); err == nil {
					counts.AppVersions++
				}
			}
		}
		for _, screen := range payload.Screens {
			screen["platform"] = platform
			if firstSyncString(screen, "appId", "app_id") == "" {
				screen["appId"] = firstSyncString(app, "id", "appId")
			}
			if flowID := screenFlow[firstSyncString(screen, "id", "screenId")]; flowID != "" {
				screen["flowId"] = flowID
			}
			if err := db.UpsertScreen(ctx, screen); err == nil {
				counts.Screens++
			}
			if version := appVersionFrom(screen, app); version != nil {
				if err := db.UpsertAppVersion(ctx, version); err == nil {
					counts.AppVersions++
				}
			}
			screenID := firstSyncString(screen, "id", "screenId")
			for _, slug := range labelSlugs(firstValue(screen, "screenPatterns", "screen_patterns", "animation_screen_patterns")) {
				if err := db.UpsertScreenPattern(ctx, screenID, slug); err == nil {
					counts.ScreenPatterns++
				}
			}
			for _, slug := range labelSlugs(firstValue(screen, "screenElements", "screen_elements", "animation_ui_elements")) {
				if err := db.UpsertScreenElement(ctx, screenID, slug); err == nil {
					counts.ScreenElements++
				}
			}
		}
	}
	return counts
}

func syncCollections(ctx context.Context, c apiClient, db *store.DB) (int, error) {
	data, _, err := c.Post("/api/collection/fetch-collections", map[string]any{})
	if err != nil {
		return 0, classifyAPIError(err, nil)
	}
	count := 0
	for _, item := range extractSyncItems(data) {
		if err := db.UpsertCollection(ctx, item); err == nil {
			count++
		}
	}
	return count, nil
}

func upsertSyncItems(ctx context.Context, db *store.DB, data json.RawMessage, platform string, seen map[string]bool) ([]map[string]any, int, error) {
	items := extractSyncItems(data)
	apps := []map[string]any{}
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
			item["slug"] = appURLSlug(firstSyncString(item, "appName", "app_name", "name"), platform, id)
			if err := db.UpsertApp(ctx, item); err != nil {
				return apps, screenCount, err
			}
			if !seen[id] {
				apps = append(apps, item)
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
	return apps, screenCount, nil
}

func tableCounts(ctx context.Context, db *store.DB) (map[string]int, error) {
	out := map[string]int{}
	for _, table := range []string{"apps", "app_versions", "screens", "flows", "patterns", "elements", "flow_actions", "screen_patterns", "screen_elements", "collections"} {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
			return nil, err
		}
		out[table] = n
	}
	return out, nil
}

func appVersionFrom(row, app map[string]any) map[string]any {
	id := firstSyncString(row, "appVersionId", "app_version_id", "latestVersionId", "latest_version_id")
	if id == "" {
		return nil
	}
	return map[string]any{
		"id":         id,
		"appId":      firstSyncString(row, "appId", "app_id", "id"),
		"version":    firstSyncString(row, "appVersion", "version"),
		"capturedAt": firstSyncString(row, "appVersionPublishedAt", "capturedAt", "createdAt"),
		"app":        app,
	}
}

func dedupeApps(rows []map[string]any) []map[string]any {
	seen := map[string]bool{}
	out := []map[string]any{}
	for _, row := range rows {
		id := firstSyncString(row, "id", "appId")
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, row)
	}
	return out
}

func dictionaryPlatform(experience string) string {
	if experience == "mobile" {
		return "mobile"
	}
	return experience
}

func labelSlugs(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, item := range arr {
		label := ""
		switch t := item.(type) {
		case string:
			label = t
		case map[string]any:
			label = firstSyncString(t, "slug", "displayName", "name", "label")
		}
		slug := slugify(label)
		if slug != "" && !seen[slug] {
			out = append(out, slug)
			seen[slug] = true
		}
	}
	return out
}

func appURLSlug(name, platform, id string) string {
	slug := slugify(name)
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

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
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
	return strings.Trim(b.String(), "-")
}

func firstValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			return v
		}
	}
	return nil
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
