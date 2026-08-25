// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// Catalog commands (search, template, facets, industries, open, sync) read
// Placeit's public Algolia catalog. search/template/facets/industries query
// Algolia live; sync mirrors the catalog into local SQLite so the analytics
// commands (top, pod, kit, gaps, rank, watch) can run offline.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/algolia"
	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/store"
	"github.com/spf13/cobra"
)

// templateResource is the resource_type used for cached catalog templates.
const templateResource = "template"

// cleanStage strips Algolia bookkeeping fields from a raw hit and returns a
// decoded map suitable for storage and output.
func cleanStage(raw json.RawMessage) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	delete(m, "_highlightResult")
	delete(m, "_snippetResult")
	delete(m, "invisible_tags")
	delete(m, "strategy_mapping")
	delete(m, "meta_description")
	return m, nil
}

// stageID returns a stable string id for a hit (objectID, else id).
func stageID(m map[string]any) string {
	if v, ok := m["objectID"]; ok {
		if s := store.ResourceIDString(v); s != "" {
			return s
		}
	}
	if v, ok := m["id"]; ok {
		return store.ResourceIDString(v)
	}
	return ""
}

// stageDeepLink returns the absolute placeit.net URL for a stage.
func stageDeepLink(m map[string]any) string {
	link, _ := m["stage_link"].(string)
	if link == "" {
		return ""
	}
	if strings.HasPrefix(link, "http") {
		return link
	}
	return "https://placeit.net" + link
}

// buildFacetFilters assembles Algolia facetFilters from the standard flags.
func buildFacetFilters(category, templateType, device string, tags []string) ([][]string, string, error) {
	var ff [][]string
	if category != "" {
		cat, err := algolia.CategoryFacet(category)
		if err != nil {
			return nil, "", err
		}
		if cat != "" {
			ff = append(ff, []string{"category_name:" + cat})
		}
	}
	if templateType != "" {
		ff = append(ff, []string{"template_type:" + strings.ToLower(templateType)})
	}
	if device != "" {
		ff = append(ff, []string{"device_tags:" + device})
	}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !strings.Contains(t, ":") {
			// Bare value: match against device_tags by default.
			t = "device_tags:" + t
		}
		ff = append(ff, []string{t})
	}
	return ff, "", nil
}

// --- search ---------------------------------------------------------------

// pp:data-source auto
func newSearchCmd(flags *rootFlags) *cobra.Command {
	var category, templateType, sortBy, device string
	var tags []string
	var free, printify bool
	var limit, page int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search Placeit's catalog of 164k templates by keyword and filters",
		Long: strings.Trim(`
Search Placeit's public template catalog (mockups, logos, videos, design
templates) by keyword, category, type, and tags. Queries Algolia live by
default; pass --data-source local to search a synced offline mirror instead.`, "\n"),
		Example: strings.Trim(`
  placeit-pp-cli search "t-shirt mockup" --category mockups --limit 10
  placeit-pp-cli search "logo" --category logos --sort best-selling --agent
  placeit-pp-cli search "hoodie" --printify --free --json --select name,stage_link`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			if query == "" && len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would search catalog for %q\n", query)
				return nil
			}
			ff, _, err := buildFacetFilters(category, templateType, device, tags)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			var filterParts []string
			if free {
				filterParts = append(filterParts, "is_free=1")
			}
			if printify {
				filterParts = append(filterParts, "is_printify=1")
			}

			if flags.dataSource == "local" {
				return searchLocal(cmd, flags, query, category, templateType, free, printify, limit, page)
			}

			index, err := algolia.IndexForSort(sortBy)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ac := algolia.New(flags.timeout)
			res, err := ac.Search(ctx, algolia.SearchParams{
				Index:        index,
				Query:        query,
				HitsPerPage:  limit,
				Page:         page,
				FacetFilters: ff,
				Filters:      strings.Join(filterParts, " AND "),
			})
			if err != nil {
				if flags.dataSource == "auto" {
					lerr := searchLocal(cmd, flags, query, category, templateType, free, printify, limit, page)
					if lerr == nil {
						return nil
					}
					return apiErr(fmt.Errorf("live search failed (%v); local fallback also failed: %w", err, lerr))
				}
				return apiErr(err)
			}
			out := make([]map[string]any, 0, len(res.Hits))
			for _, h := range res.Hits {
				m, err := cleanStage(h)
				if err != nil {
					continue
				}
				out = append(out, m)
			}
			printProvenance(cmd, len(out), DataProvenance{Source: "live", ResourceType: templateResource})
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Filter by category: mockups, logos, videos, designs")
	cmd.Flags().StringVar(&templateType, "type", "", "Filter by template type: image, blender, video, multi-stage")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Sort order: relevance (default), newest, best-selling, free")
	cmd.Flags().StringVar(&device, "device", "", "Filter by device tag (e.g. 'T-Shirt', 'Coffee Mug')")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Filter by tag, repeatable; 'facet:value' or a bare device tag")
	cmd.Flags().BoolVar(&free, "free", false, "Only free templates")
	cmd.Flags().BoolVar(&printify, "printify", false, "Only Printify-compatible (POD-ready) mockups")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")
	cmd.Flags().IntVar(&page, "page", 0, "Zero-based page index")
	return cmd
}

// searchLocal queries the synced mirror via FTS, then applies category/type/free filters.
func searchLocal(cmd *cobra.Command, flags *rootFlags, query, category, templateType string, free, printify bool, limit, page int) error {
	dbPath := defaultDBPath("placeit-pp-cli")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: placeit-pp-cli sync\n", dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return apiErr(err)
	}
	defer db.Close()
	if !hintIfUnsynced(cmd, db, templateResource) {
		hintIfStale(cmd, db, templateResource, flags.maxAge)
	}
	if page < 0 {
		page = 0
	}
	offset := page * limit
	scanLimit := (offset + limit) * 4
	if scanLimit < 200 {
		scanLimit = 200
	}
	rows, err := db.Search(query, scanLimit, templateResource)
	if err != nil {
		return apiErr(err)
	}
	wantCat, _ := algolia.CategoryFacet(category)
	out := make([]map[string]any, 0, limit)
	matched := 0
	for _, raw := range rows {
		m, err := cleanStage(raw)
		if err != nil {
			continue
		}
		if wantCat != "" && fmt.Sprint(m["category_name"]) != wantCat {
			continue
		}
		if templateType != "" && !strings.EqualFold(fmt.Sprint(m["template_type"]), templateType) {
			continue
		}
		if free && !truthy(m["is_free"]) {
			continue
		}
		if printify && !truthy(m["is_printify"]) {
			continue
		}
		matched++
		if matched <= offset {
			continue // skip earlier pages
		}
		out = append(out, m)
		if len(out) >= limit {
			break
		}
	}
	syncedAt := parseStoreTime(db.GetLastSyncedAt(templateResource))
	printProvenance(cmd, len(out), DataProvenance{Source: "local", SyncedAt: syncedAt, ResourceType: templateResource})
	return flags.printJSON(cmd, out)
}

// --- template -------------------------------------------------------------

// pp:data-source auto
func newTemplateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "template <id|slug>",
		Short:       "Show a single template's details, deep link, and thumbnails",
		Example:     "  placeit-pp-cli template 41935 --agent --select name,stage_link,large_thumb",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a template id or slug is required"))
			}
			arg := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ac := algolia.New(flags.timeout)

			var params algolia.SearchParams
			params.HitsPerPage = 1
			if isNumericID(arg) {
				params.Filters = "id=" + arg
			} else {
				// slug or name — strip a stage_link path to its slug tail
				slug := arg
				if i := strings.LastIndex(slug, "/"); i >= 0 {
					slug = slug[i+1:]
				}
				params.Query = strings.ReplaceAll(slug, "-", " ")
			}
			res, err := ac.Search(ctx, params)
			if err != nil {
				return apiErr(err)
			}
			if len(res.Hits) == 0 {
				return notFoundErr(fmt.Errorf("no template found for %q", arg))
			}
			m, err := cleanStage(res.Hits[0])
			if err != nil {
				return apiErr(err)
			}
			m["deep_link"] = stageDeepLink(m)
			return flags.printJSON(cmd, m)
		},
	}
	return cmd
}

// --- facets ---------------------------------------------------------------

// pp:data-source live
func newFacetsCmd(flags *rootFlags) *cobra.Command {
	var facet, category string
	cmd := &cobra.Command{
		Use:         "facets",
		Short:       "Show catalog facet distributions (categories, types, tags) with counts",
		Example:     "  placeit-pp-cli facets --facet device_tags --category mockups",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if facet == "" {
				facet = "category_name"
			}
			ff, _, err := buildFacetFilters(category, "", "", nil)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ac := algolia.New(flags.timeout)
			res, err := ac.Search(ctx, algolia.SearchParams{
				HitsPerPage:  0,
				Facets:       []string{facet},
				FacetFilters: ff,
			})
			if err != nil {
				return apiErr(err)
			}
			type facetCount struct {
				Value string `json:"value"`
				Count int    `json:"count"`
			}
			counts := make([]facetCount, 0)
			for v, c := range res.Facets[facet] {
				counts = append(counts, facetCount{Value: v, Count: c})
			}
			sortByCountDesc(counts, func(i int) int { return counts[i].Count })
			return flags.printJSON(cmd, map[string]any{
				"facet":  facet,
				"total":  res.NbHits,
				"values": counts,
			})
		},
	}
	cmd.Flags().StringVar(&facet, "facet", "", "Facet to break down (default category_name; e.g. device_tags, template_type, color_tags)")
	cmd.Flags().StringVar(&category, "category", "", "Restrict the facet breakdown to a category")
	return cmd
}

// --- industries -----------------------------------------------------------

// pp:data-source live
func newIndustriesCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "industries [query]",
		Short:       "List Placeit's 152 industry taxonomy entries",
		Example:     "  placeit-pp-cli industries \"coffee\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ac := algolia.New(flags.timeout)
			res, err := ac.Search(ctx, algolia.SearchParams{
				Index:       algolia.IndexIndustries,
				Query:       strings.Join(args, " "),
				HitsPerPage: limit,
			})
			if err != nil {
				return apiErr(err)
			}
			out := make([]map[string]any, 0, len(res.Hits))
			for _, h := range res.Hits {
				var m map[string]any
				if json.Unmarshal(h, &m) != nil {
					continue
				}
				delete(m, "_highlightResult")
				delete(m, "asset_groups")
				if name, ok := m["industry"]; ok {
					m["name"] = name
				}
				out = append(out, m)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum industries to return")
	return cmd
}

// --- open -----------------------------------------------------------------

// pp:data-source auto
func newOpenCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:   "open <id|slug>",
		Short: "Resolve a template's deep link (and editor link) to open in a browser",
		Long: strings.Trim(`
Placeit's render and download flow runs only in the browser editor, so this
command resolves a template to its placeit.net deep link and editor link. By
default it prints the URLs; pass --launch to open the editor in your browser.`, "\n"),
		Example:     "  placeit-pp-cli open 41935\n  placeit-pp-cli open 41935 --launch",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a template id or slug is required"))
			}
			arg := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ac := algolia.New(flags.timeout)
			params := algolia.SearchParams{HitsPerPage: 1}
			if isNumericID(arg) {
				params.Filters = "id=" + arg
			} else {
				slug := arg
				if i := strings.LastIndex(slug, "/"); i >= 0 {
					slug = slug[i+1:]
				}
				params.Query = strings.ReplaceAll(slug, "-", " ")
			}
			res, err := ac.Search(ctx, params)
			if err != nil {
				return apiErr(err)
			}
			if len(res.Hits) == 0 {
				return notFoundErr(fmt.Errorf("no template found for %q", arg))
			}
			m, err := cleanStage(res.Hits[0])
			if err != nil {
				return apiErr(err)
			}
			deep := stageDeepLink(m)
			editor, _ := m["editor_link"].(string)
			if editor != "" && !strings.HasPrefix(editor, "http") {
				editor = "https://placeit.net" + editor
			}
			result := map[string]any{"id": m["id"], "name": m["name"], "deep_link": deep, "editor_link": editor}
			if launch {
				target := editor
				if target == "" {
					target = deep
				}
				if cliutil.IsVerifyEnv() {
					fmt.Fprintln(cmd.OutOrStdout(), "would launch:", target)
					return flags.printJSON(cmd, result)
				}
				if err := openInBrowser(target); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "could not launch browser: %v\n", err)
				}
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Open the template's editor link in your default browser")
	return cmd
}

// --- sync (catalog mirror) ------------------------------------------------

// pp:data-source live
func newCatalogSyncCmd(flags *rootFlags) *cobra.Command {
	var category, templateType, sortBy, query string
	var maxPages int
	var free, printify bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror the Placeit catalog into local SQLite for offline search and analytics",
		Long: strings.Trim(`
Pull templates from Placeit's catalog into a local SQLite database so the
analytics commands (top, pod, kit, gaps, rank, watch) and offline search work
without the network. Scope the mirror with --category, --type, and a --query;
--max-pages bounds how much is pulled (1000 templates per page).`, "\n"),
		Example: strings.Trim(`
  placeit-pp-cli sync --category mockups --max-pages 5
  placeit-pp-cli sync --query "t-shirt" --printify --max-pages 3`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync catalog into local mirror")
				return nil
			}
			ff, _, err := buildFacetFilters(category, templateType, "", nil)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			var filterParts []string
			if free {
				filterParts = append(filterParts, "is_free=1")
			}
			if printify {
				filterParts = append(filterParts, "is_printify=1")
			}
			index, err := algolia.IndexForSort(sortBy)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			dbPath := defaultDBPath("placeit-pp-cli")
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return apiErr(err)
			}
			defer db.Close()
			ac := algolia.New(flags.timeout)

			total := 0
			perPage := 1000
			for page := 0; page < maxPages; page++ {
				res, err := ac.Search(ctx, algolia.SearchParams{
					Index:        index,
					Query:        query,
					HitsPerPage:  perPage,
					Page:         page,
					FacetFilters: ff,
					Filters:      strings.Join(filterParts, " AND "),
				})
				if err != nil {
					return apiErr(err)
				}
				items := make([]json.RawMessage, 0, len(res.Hits))
				for _, h := range res.Hits {
					m, err := cleanStage(h)
					if err != nil {
						continue
					}
					id := stageID(m)
					if id == "" {
						continue
					}
					m["__id"] = id
					b, _ := json.Marshal(m)
					items = append(items, b)
				}
				inserted, _, err := db.UpsertBatch(templateResource, items)
				if err != nil {
					return apiErr(err)
				}
				total += inserted
				if humanFriendly {
					fmt.Fprintf(cmd.ErrOrStderr(), "synced page %d (%d templates, %d total)\n", page+1, len(items), total)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"sync_page","page":%d,"count":%d,"total":%d}`+"\n", page+1, len(items), total)
				}
				if page+1 >= res.NbPages || len(res.Hits) == 0 {
					break
				}
			}
			_ = db.SaveSyncState(templateResource, "", total)
			return flags.printJSON(cmd, map[string]any{"synced": total, "resource": templateResource})
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Restrict the mirror to a category: mockups, logos, videos, designs")
	cmd.Flags().StringVar(&templateType, "type", "", "Restrict the mirror to a template type")
	cmd.Flags().StringVar(&sortBy, "sort", "", "Pull order: relevance (default), newest, best-selling, free")
	cmd.Flags().StringVar(&query, "query", "", "Restrict the mirror to templates matching this query")
	cmd.Flags().IntVar(&maxPages, "max-pages", 3, "Maximum pages to pull (1000 templates per page)")
	cmd.Flags().BoolVar(&free, "free", false, "Mirror only free templates")
	cmd.Flags().BoolVar(&printify, "printify", false, "Mirror only Printify-compatible mockups")
	return cmd
}

// --- shared helpers -------------------------------------------------------

// openInBrowser opens a URL in the user's default browser (best-effort).
func openInBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	// #nosec G204 -- cmd is a fixed per-OS literal (open/rundll32/xdg-open); url
	// is passed as a discrete argv element to exec (no shell), so it cannot
	// inject commands. Safe regardless of how the catalog deep link is built.
	return exec.Command(cmd, args...).Start()
}

// algoliaCategoryFacet resolves a category token to its catalog facet value.
func algoliaCategoryFacet(category string) (string, error) {
	return algolia.CategoryFacet(category)
}

func isNumericID(s string) bool {
	if s == "" {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x == "true" || x == "1"
	default:
		return false
	}
}

func parseStoreTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func sortByCountDesc[T any](items []T, count func(int) int) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && count(j) > count(j-1); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// openCatalogStore opens the local mirror read-only and returns a helpful
// hint (and empty output) when it does not exist yet.
func openCatalogStore(cmd *cobra.Command, flags *rootFlags) (*store.Store, bool, error) {
	dbPath := defaultDBPath("placeit-pp-cli")
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: placeit-pp-cli sync\n", dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, false, nil
	}
	// Analytics commands (top, rank, gaps, pod, kit) never write, so open the
	// mirror read-only: this skips schema migration and the WAL write lock, and
	// the driver rejects writes outright. The os.Stat check above guarantees the
	// file exists, satisfying OpenReadOnly's precondition (it does not migrate).
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, false, apiErr(err)
	}
	return db, true, nil
}

// loadCatalog reads every cached template into decoded maps. Analytics
// commands compute distribution-wide statistics (percentiles, cross-facet
// pivots, per-industry counts) that need the full local set, so this
// intentionally materializes all rows rather than a LIMIT-ed page. Past a
// large threshold, warn on stderr so the memory cost of an unusually large
// mirror is visible; stdout (including --json) stays clean.
func loadCatalog(db *store.Store) ([]map[string]any, error) {
	rows, err := db.List(templateResource, 1000000)
	if err != nil {
		return nil, err
	}
	if len(rows) > 50000 {
		fmt.Fprintf(os.Stderr, "note: loading %d templates into memory for analytics; sync a narrower slice if this is slow\n", len(rows))
	}
	out := make([]map[string]any, 0, len(rows))
	for _, raw := range rows {
		var m map[string]any
		if json.Unmarshal(raw, &m) == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// asFloat coerces a JSON-decoded numeric/string value to float64.
func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}

// tagValues returns the string values of a tag-array field on a stage.
func tagValues(m map[string]any, field string) []string {
	raw, ok := m[field].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// matchesQuery reports whether a stage's name/tags loosely contain all query terms.
func matchesQuery(m map[string]any, query string) bool {
	if strings.TrimSpace(query) == "" {
		return true
	}
	hay := strings.ToLower(fmt.Sprint(m["name"]))
	for _, f := range []string{"device_tags", "stage_tags", "bundle_tags"} {
		hay += " " + strings.ToLower(strings.Join(tagValues(m, f), " "))
	}
	for _, term := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(hay, term) {
			return false
		}
	}
	return true
}
