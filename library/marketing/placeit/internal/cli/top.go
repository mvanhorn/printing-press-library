// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// top ranks cached templates by real purchase count — popularity Placeit's
// own UI never exposes as a sort.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/algolia"
	"github.com/spf13/cobra"
)

func newNovelTopCmd(flags *rootFlags) *cobra.Command {
	var category, device string
	var limit int

	cmd := &cobra.Command{
		Use:   "top [query]",
		Short: "Rank cached templates by real purchase count, not opaque relevance.",
		Long: strings.Trim(`
Rank templates in your local mirror by Placeit's real per-template purchase
count — the popularity signal the web UI never lets you sort by. Run 'sync'
first to populate the mirror. Scope with a query, --category, and --device.`, "\n"),
		Example: strings.Trim(`
  placeit-pp-cli top "t-shirt" --category mockups --agent
  placeit-pp-cli top "logo" --category logos --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")

			// Live-first: Algolia's best-selling replica already ranks by
			// purchases, so 'top' works with no sync. --data-source local
			// ranks the offline mirror instead.
			if flags.dataSource != "local" {
				ff, _, ferr := buildFacetFilters(category, "", device, nil)
				if ferr != nil {
					_ = cmd.Usage()
					return usageErr(ferr)
				}
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				ac := algolia.New(flags.timeout)
				res, serr := ac.Search(ctx, algolia.SearchParams{
					Index:        algolia.IndexBestSelling,
					Query:        query,
					HitsPerPage:  limit,
					FacetFilters: ff,
				})
				if serr != nil {
					if flags.dataSource == "live" {
						return apiErr(serr)
					}
					// auto: fall through to local mirror below
				} else {
					out := make([]map[string]any, 0, len(res.Hits))
					for _, h := range res.Hits {
						m, cerr := cleanStage(h)
						if cerr != nil {
							continue
						}
						out = append(out, projectStage(m))
					}
					printProvenance(cmd, len(out), DataProvenance{Source: "live", ResourceType: templateResource})
					return flags.printJSON(cmd, out)
				}
			}

			db, ok, err := openCatalogStore(cmd, flags)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, templateResource) {
				hintIfStale(cmd, db, templateResource, flags.maxAge)
			}
			all, err := loadCatalog(db)
			if err != nil {
				return apiErr(err)
			}
			wantCat, _ := catFacetOrEmpty(category)
			type ranked struct {
				m         map[string]any
				purchases float64
			}
			matches := make([]ranked, 0)
			for _, m := range all {
				if wantCat != "" && fmt.Sprint(m["category_name"]) != wantCat {
					continue
				}
				if device != "" && !containsTag(m, "device_tags", device) {
					continue
				}
				if !matchesQuery(m, query) {
					continue
				}
				matches = append(matches, ranked{m: m, purchases: asFloat(m["purchases"])})
			}
			sort.SliceStable(matches, func(i, j int) bool { return matches[i].purchases > matches[j].purchases })
			if len(matches) > limit {
				matches = matches[:limit]
			}
			out := make([]map[string]any, 0, len(matches))
			for _, r := range matches {
				out = append(out, projectStage(r.m))
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Filter by category: mockups, logos, videos, designs")
	cmd.Flags().StringVar(&device, "device", "", "Filter by device tag (e.g. 'T-Shirt')")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results to return")
	return cmd
}

// catFacetOrEmpty resolves a category token to its facet value, returning ""
// for an empty token and ignoring unknown values (caller may want a loose match).
func catFacetOrEmpty(category string) (string, error) {
	if strings.TrimSpace(category) == "" {
		return "", nil
	}
	return algoliaCategoryFacet(category)
}

// containsTag reports whether a stage's tag-array field contains value (case-insensitive).
func containsTag(m map[string]any, field, value string) bool {
	for _, t := range tagValues(m, field) {
		if strings.EqualFold(t, value) {
			return true
		}
	}
	return false
}

// projectStage returns the high-gravity display fields for a stage, with a deep link.
func projectStage(m map[string]any) map[string]any {
	out := map[string]any{
		"id":            m["id"],
		"name":          m["name"],
		"category_name": m["category_name"],
		"template_type": m["template_type"],
		"purchases":     m["purchases"],
		"is_free":       m["is_free"],
		"is_printify":   m["is_printify"],
		"stage_link":    m["stage_link"],
		"deep_link":     stageDeepLink(m),
	}
	return out
}
