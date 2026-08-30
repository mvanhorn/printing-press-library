// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/store"
)

// mcpMarketListItem is one entry from an ItemList's itemListElement array on
// MCP Market: {"@type":"ListItem","position":N,"item":{...SoftwareApplication}}.
type mcpMarketListItem struct {
	Position int            `json:"position"`
	Item     map[string]any `json:"item"`
}

var jsonLDScriptPattern = regexp.MustCompile(`(?is)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)

// fetchAllJSONLD does a raw GET against path and returns every parsed
// <script type="application/ld+json"> block on the page, in document order.
// MCP Market listing pages carry Organization + WebSite + ItemList blocks in
// that order, so the generator's built-in embedded-json extractor (which
// only ever returns the first match) cannot reach the ItemList. This walks
// all of them.
func fetchAllJSONLD(ctx context.Context, c *client.Client, path string, params map[string]string) ([]map[string]any, error) {
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	matches := jsonLDScriptPattern.FindAllSubmatch(raw, -1)
	blocks := make([]map[string]any, 0, len(matches))
	for _, m := range matches {
		var obj map[string]any
		if err := json.Unmarshal(m[1], &obj); err != nil {
			continue
		}
		blocks = append(blocks, obj)
	}
	return blocks, nil
}

// findJSONLDByType returns the first parsed JSON-LD block whose "@type"
// matches ldType.
func findJSONLDByType(blocks []map[string]any, ldType string) (map[string]any, bool) {
	for _, b := range blocks {
		if t, _ := b["@type"].(string); strings.EqualFold(t, ldType) {
			return b, true
		}
	}
	return nil, false
}

// itemListEntries extracts the flattened []item objects from an ItemList
// JSON-LD block's itemListElement array (unwrapping the ListItem wrapper).
func itemListEntries(itemList map[string]any) []map[string]any {
	raw, ok := itemList["itemListElement"]
	if !ok {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var wrapped []mcpMarketListItem
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil
	}
	out := make([]map[string]any, 0, len(wrapped))
	for _, w := range wrapped {
		if w.Item == nil {
			continue
		}
		out = append(out, w.Item)
	}
	return out
}

// fetchItemList fetches path and returns the flattened item entries of the
// page's ItemList JSON-LD data. MCP Market pages carry this two ways: some
// (leaderboards, /daily, /server, /tools/skills) have a top-level ItemList
// block; others (/categories/<slug>, /search) wrap it under a CollectionPage
// or SearchResultsPage's "mainEntity" field. This checks both shapes.
func fetchItemList(ctx context.Context, c *client.Client, path string, params map[string]string) ([]map[string]any, error) {
	blocks, err := fetchAllJSONLD(ctx, c, path, params)
	if err != nil {
		return nil, err
	}
	if itemList, ok := findJSONLDByType(blocks, "ItemList"); ok {
		return itemListEntries(itemList), nil
	}
	for _, wrapperType := range []string{"CollectionPage", "SearchResultsPage"} {
		page, ok := findJSONLDByType(blocks, wrapperType)
		if !ok {
			continue
		}
		mainEntity, ok := page["mainEntity"].(map[string]any)
		if !ok {
			return nil, nil
		}
		return itemListEntries(mainEntity), nil
	}
	return nil, fmt.Errorf("no ItemList structured data found on %s", path)
}

// fetchSearchResults fetches /search?q=<query> and returns the flattened
// item entries from the SearchResultsPage's mainEntity ItemList.
func fetchSearchResults(ctx context.Context, c *client.Client, query string) ([]map[string]any, error) {
	return fetchItemList(ctx, c, "/search", map[string]string{"q": query})
}

// slugFromMCPMarketURL derives the trailing slug from an MCP Market entity
// URL, e.g. "https://mcpmarket.com/server/firecrawl" -> "firecrawl".
func slugFromMCPMarketURL(url string) string {
	parts := strings.Split(strings.TrimRight(url, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// slugifyCategory forgives display-name input ("Developer Tools", "API
// Development") by lowercasing and hyphenating, since users naturally type
// the name shown on the site rather than its URL slug. Already-slugged
// input passes through unchanged.
func slugifyCategory(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), "-")
	return s
}

// applyLimit truncates items to at most limit entries. limit <= 0 means no cap.
func applyLimit(items []map[string]any, limit int) []map[string]any {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

// storeOpenForNovel opens the local store for hand-written history commands
// (trending, diff, leaderboard --as-of, watch category, author, dedupe,
// stack). It is a thin wrapper so those commands share one open call site.
func storeOpenForNovel(ctx context.Context, dbPath string) (*store.Store, error) {
	return store.OpenWithContext(ctx, dbPath)
}

// persistItems mirrors browsed catalog items into the local resources table,
// keyed by the slug derived from each item's "url" field. There is no more
// generated "list" sync endpoint for server/mcpclient/skill (MCP Market's
// listing pages needed hand-written multi-JSON-LD extraction — see
// fetchItemList), so browsing is what populates local history for trending,
// diff, leaderboard --as-of, author, dedupe, and watch. Best-effort: a
// persistence failure is logged to stderr, never surfaced as a command error,
// since the live browse result is already in hand and should still print.
func persistItems(ctx context.Context, flags *rootFlags, resourceType string, items []map[string]any) {
	if len(items) == 0 {
		return
	}
	dbPath := defaultDBPath("mcpmarket-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	for _, item := range items {
		url, _ := item["url"].(string)
		slug := slugFromMCPMarketURL(url)
		if slug == "" {
			continue
		}
		data, err := json.Marshal(item)
		if err != nil {
			continue
		}
		_ = db.Upsert(resourceType, slug, data)
	}
}
