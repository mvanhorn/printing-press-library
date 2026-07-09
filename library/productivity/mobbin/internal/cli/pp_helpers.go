// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/imagecache"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/store"
)

type screenHit struct {
	ID         string `json:"id"`
	App        string `json:"app"`
	AppSlug    string `json:"app_slug"`
	Pattern    string `json:"pattern"`
	CapturedAt string `json:"captured_at"`
	ImageURL   string `json:"image_url"`
	LocalPath  string `json:"local_path,omitempty"`
	Platform   string `json:"platform"`
}

func fetchDictionary(ctx context.Context, c *client.Client, want, platform string) ([]map[string]any, error) {
	raw, _, err := c.Post(ctx, "/api/filter-tags/fetch-dictionary-definitions", map[string]any{})
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	items := extractNamedArray(raw, "value")
	if len(items) == 0 {
		items = extractNamedArray(raw, "data")
	}
	out := []map[string]any{}
	for _, it := range items {
		// Dictionary groups are identified by top-level slug; entries live in subCategories.
		if fmt.Sprint(it["slug"]) != want {
			continue
		}
		if platform != "" && fmt.Sprint(it["experience"]) != platform {
			continue
		}
		if sub, ok := it["subCategories"].([]any); ok {
			for _, row := range sub {
				if m, ok := row.(map[string]any); ok {
					if entries, ok := m["entries"].([]any); ok {
						for _, entry := range entries {
							if entryMap, ok := entry.(map[string]any); ok {
								out = append(out, entryMap)
							}
						}
						continue
					}
					out = append(out, m)
				}
			}
		}
	}
	return out, nil
}

func searchScreensAPI(ctx context.Context, c *client.Client, platform, pattern, industry string, limit int) ([]screenHit, error) {
	// screen search moved to
	// /api/search/fetch-search-page-screens with filters nested under
	// searchQuery. Filter VALUES are display names ("Paywall"), not slugs, so
	// the pattern/industry args are passed through verbatim. Response rows live
	// under value.data, which extractSyncItems unwraps.
	searchQuery := map[string]any{
		"platform":              platform,
		"type":                  "filters",
		"screenPatterns":        []string{pattern},
		"screenElements":        nil,
		"textInScreenshotQuery": nil,
		"categories":            nil,
		"hasAnimation":          nil,
		"sortBy":                "trending",
	}
	if industry != "" {
		searchQuery["categories"] = []string{industry}
	}
	body := map[string]any{
		"searchRequestId": "",
		"pageIndex":       0,
		"searchQuery":     searchQuery,
	}
	raw, _, err := c.Post(ctx, "/api/search/fetch-search-page-screens", body)
	if err != nil {
		return nil, classifyAPIError(err, nil)
	}
	// Surface Mobbin's HTTP-200 unauthenticated error body ({"error":{...}} or an
	// absent "value") instead of silently returning zero hits — otherwise cross,
	// grab, and deck report empty results with no hint to re-authenticate.
	if _, envErr := parseSearchEnvelope(raw); envErr != nil {
		return nil, envErr
	}
	rows := extractSyncItems(raw)
	hits := make([]screenHit, 0, len(rows))
	for _, row := range rows {
		appID := val(row, "appId", "app_id")
		appName := val(row, "appName", "app_name", "app")
		appSlug := val(row, "appSlug", "app_slug", "slug")
		if appSlug == "" && appID != "" && appName != "" {
			// Mobbin's own slug form: <slugified-app-name>-<platform>-<appId>
			appSlug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(appName), " ", "-")) + "-" + platform + "-" + appID
		}
		h := screenHit{
			ID:         val(row, "id", "screenId"),
			App:        appName,
			AppSlug:    appSlug,
			Pattern:    pattern,
			CapturedAt: val(row, "createdAt", "capturedAt", "captured_at"),
			ImageURL:   val(row, "fullpageScreenUrl", "screenUrl", "imageUrlFull", "image_url_full", "fullImageUrl", "imageUrl", "image_url"),
			Platform:   platform,
		}
		if h.ID != "" {
			hits = append(hits, h)
		}
	}
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

func cacheHits(ctx context.Context, hits []screenHit) ([]screenHit, []error) {
	cache, err := imagecache.New("")
	if err != nil {
		return hits, []error{err}
	}
	items := make([]imagecache.FetchItem, 0, len(hits))
	for _, h := range hits {
		items = append(items, imagecache.FetchItem{ImageURL: h.ImageURL, Platform: h.Platform, AppSlug: h.AppSlug, ScreenID: h.ID})
	}
	paths, errs := cache.FetchMany(ctx, items, imagecache.CDNOpts{}, 8)
	for i := range hits {
		hits[i].LocalPath = paths[hits[i].ID]
	}
	return hits, errs
}

func writeDeckZip(path string, hits []screenHit) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	// Build the full CSV in memory first, then write it as a single zip entry.
	// Writing manifest.csv concurrently with subsequent zip.Create() calls
	// breaks because zip writers serialize their open entry.
	var csvBuf bytes.Buffer
	cw := csv.NewWriter(&csvBuf)
	_ = cw.Write([]string{"id", "app", "pattern", "captured_at", "local_path", "image_url"})
	for _, h := range hits {
		_ = cw.Write([]string{h.ID, h.App, h.Pattern, h.CapturedAt, h.LocalPath, h.ImageURL})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}
	mw, err := zw.Create("manifest.csv")
	if err != nil {
		return err
	}
	if _, err := mw.Write(csvBuf.Bytes()); err != nil {
		return err
	}
	for _, h := range hits {
		if h.LocalPath == "" {
			continue
		}
		if err := addZipFile(zw, "images/"+filepath.Base(h.LocalPath), h.LocalPath); err != nil {
			return err
		}
	}
	return nil
}

func addZipFile(zw *zip.Writer, name, path string) error {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = src.WriteTo(dst)
	return err
}

// openStore opens the local store READ-ONLY for the human-facing query
// commands (sql, bench, audit, drift, app-slug lookup). mode=ro is the
// engine-level guarantee that a crafted single-statement CTE write
// ("WITH x AS (SELECT 1) DELETE ...") cannot mutate the cache — RawQuery's
// leading-token allowlist alone cannot catch that. Returns (nil, nil) when the
// store file does not exist yet so callers degrade to a "run sync first"
// message instead of erroring on a mode=ro open of a missing file. The path
// defaults to the same location `sync` writes and the MCP `sql` tool reads.
func openStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("mobbin-pp-cli")
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return store.OpenReadOnlyContext(ctx, dbPath)
}

func sqlQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func parseSince(s string) (time.Duration, error) {
	return cliutil.ParseDurationLoose(s)
}

func val(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}

func appNameSlug(s string) string {
	s = strings.ToLower(s)
	parts := strings.Split(s, "-")
	if len(parts) > 0 {
		return parts[0]
	}
	return s
}

func sortRows(rows []map[string]any, key string) {
	sort.Slice(rows, func(i, j int) bool { return fmt.Sprint(rows[i][key]) < fmt.Sprint(rows[j][key]) })
}
