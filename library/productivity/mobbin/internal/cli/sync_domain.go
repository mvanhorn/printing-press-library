// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/appscrape"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/mobbin/internal/store"
)

// domainSyncOpts controls the Mobbin domain-population phase that runs inside
// `sync` after the framework resource sync. It populates the domain tables
// (screens/flows/app_versions/patterns/elements/flow_actions/screen_*), which
// the framework sync (apps/resources only) leaves empty, so bench/audit/drift
// return real rows.
type domainSyncOpts struct {
	Platforms          []string
	LimitPerCategory   int
	AppsPerPlatform    int
	NoScrape           bool
	IncludeCollections bool
}

// runDomainSync populates the Mobbin domain tables. Per-endpoint failures are
// warnings, not fatal: the public dictionary populates without auth, while
// screens/flows/collections need a Mobbin Pro session and degrade to a warning
// when absent. Callers gate network work off dry-run/verify before invoking.
// Returns the total number of domain rows written across all tables so the
// caller can treat a successful domain population as a successful sync even
// when the framework resource phase synced nothing (e.g. the only flat-list
// resource needs auth).
func runDomainSync(ctx context.Context, c *client.Client, db *store.Store, opts domainSyncOpts, stderr io.Writer) int {
	// Dogfood runs under a flat per-command timeout; cap the expensive
	// HTML-scrape fan-out so the domain phase never trips it.
	if cliutil.IsDogfoodEnv() {
		if opts.AppsPerPlatform > 2 {
			opts.AppsPerPlatform = 2
		}
		if opts.LimitPerCategory > 5 {
			opts.LimitPerCategory = 5
		}
	}

	counts := syncCounts{Apps: map[string]int{}}
	for _, platform := range opts.Platforms {
		apps, screens, err := syncPlatform(ctx, c, db, platform, opts.LimitPerCategory)
		if err != nil {
			fmt.Fprintf(stderr, "domain sync: platform %s: %v\n", platform, err)
			continue
		}
		counts.Apps[platform] += len(apps)
		counts.Screens += screens
		if !opts.NoScrape {
			sc := syncScrapedApps(ctx, c, db, apps, platform, opts.AppsPerPlatform, stderr)
			counts.AppVersions += sc.AppVersions
			counts.Screens += sc.Screens
			counts.Flows += sc.Flows
			counts.ScreenPatterns += sc.ScreenPatterns
			counts.ScreenElements += sc.ScreenElements
		}
	}
	if patterns, elements, flowActions, err := syncDictionary(ctx, c, db); err != nil {
		fmt.Fprintf(stderr, "domain sync: dictionary: %v\n", err)
	} else {
		counts.Patterns, counts.Elements, counts.FlowActions = patterns, elements, flowActions
	}
	if opts.IncludeCollections {
		if collections, err := syncCollections(ctx, c, db); err != nil {
			fmt.Fprintf(stderr, "domain sync: collections: %v\n", err)
		} else {
			counts.Collections = collections
		}
	}

	fmt.Fprintln(stderr, "Domain sync complete:")
	for _, platform := range opts.Platforms {
		fmt.Fprintf(stderr, "  apps (%s):%13d\n", platform, counts.Apps[platform])
	}
	for _, table := range []string{"app_versions", "screens", "flows", "patterns", "elements", "flow_actions", "screen_patterns", "screen_elements", "collections"} {
		n, err := db.TableCount(ctx, table)
		if err != nil {
			continue
		}
		fmt.Fprintf(stderr, "  %-16s %8d\n", table+":", n)
	}
	// Return rows written THIS run, not all-time TableCount: sync's exit-code
	// gate treats a nonzero return as success, so summing pre-existing table
	// totals would mask a fully-failed domain phase over a populated store.
	total := counts.Screens + counts.Flows + counts.AppVersions +
		counts.Patterns + counts.Elements + counts.FlowActions +
		counts.ScreenPatterns + counts.ScreenElements + counts.Collections
	for _, platform := range opts.Platforms {
		total += counts.Apps[platform]
	}
	return total
}

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

func syncPlatform(ctx context.Context, c *client.Client, db *store.Store, platform string, limit int) ([]map[string]any, int, error) {
	apps := map[string]bool{}
	appRows := []map[string]any{}
	screenCount := 0

	data, err := c.Get(ctx, "/api/searchable-apps/"+platform, map[string]string{})
	if err != nil {
		return nil, 0, classifyAPIError(err, nil)
	}
	rows, s, err := upsertSyncItems(ctx, db, data, platform, apps)
	if err != nil {
		return nil, 0, err
	}
	appRows = append(appRows, rows...)
	screenCount += s

	data, _, err = c.Post(ctx, "/api/popular-apps/fetch-popular-apps-with-preview-screens", map[string]any{"platform": platform, "limitPerCategory": limit})
	if err != nil {
		return nil, 0, classifyAPIError(err, nil)
	}
	rows, s, err = upsertSyncItems(ctx, db, data, platform, apps)
	if err != nil {
		return nil, 0, err
	}
	appRows = append(appRows, rows...)
	screenCount += s

	data, _, err = c.Post(ctx, "/api/discover/fetch-discover-page-apps", map[string]any{"tab": "latest", "platform": platform, "pageIndex": 0})
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

func syncDictionary(ctx context.Context, c *client.Client, db *store.Store) (int, int, int, error) {
	data, _, err := c.Post(ctx, "/api/filter-tags/fetch-dictionary-definitions", map[string]any{})
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
				entry["slug"] = slugify(firstSyncString(entry, "name"))
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

func syncScrapedApps(ctx context.Context, c *client.Client, db *store.Store, apps []map[string]any, platform string, limit int, stderr io.Writer) syncCounts {
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

func syncCollections(ctx context.Context, c *client.Client, db *store.Store) (int, error) {
	data, _, err := c.Post(ctx, "/api/collection/fetch-collections", map[string]any{})
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

func upsertSyncItems(ctx context.Context, db *store.Store, data json.RawMessage, platform string, seen map[string]bool) ([]map[string]any, int, error) {
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

func looksLikeScreen(m map[string]any) bool {
	return firstSyncString(m, "screenId") != "" || firstSyncString(m, "imageUrl", "image_url", "ocrText", "ocr_text") != ""
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

func firstValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			return v
		}
	}
	return nil
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
	var b []rune
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b = append(b, r+('a'-'A'))
			lastDash = false
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b = append(b, r)
			lastDash = false
		default:
			if !lastDash && len(b) > 0 {
				b = append(b, '-')
				lastDash = true
			}
		}
	}
	out := string(b)
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	return out
}
