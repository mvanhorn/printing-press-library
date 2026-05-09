// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// DiffResult represents a product present in one market but not another.
type DiffResult struct {
	Slug     string `json:"slug"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Market   string `json:"market"` // "from_only" or "to_only"
}

func newDiffCmd(flags *rootFlags) *cobra.Command {
	var fromCountry string
	var toCountry string
	var category string
	var selectFields string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "See which products exist in one country but not another",
		Long: `Compare product catalogs across two IQOS markets and show what's exclusive to each.
Uses the local synced catalog when available; falls back to live sitemap fetch.

Requires at least one synced market: iqos-pp-cli sync --country <code>`,
		Example: strings.Trim(`
  iqos-pp-cli diff --from us --to gb --json
  iqos-pp-cli diff --from de --to fr --category heets
  iqos-pp-cli diff --from us --to gb --select slug,category --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if fromCountry == "" || toCountry == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Usage: iqos-pp-cli diff --from <country> --to <country>")
				return cmd.Help()
			}

			fromSlugs, err := catalogSlugsForCountry(cmd, flags, fromCountry)
			if err != nil {
				return fmt.Errorf("fetching catalog for %s: %w", fromCountry, err)
			}
			toSlugs, err := catalogSlugsForCountry(cmd, flags, toCountry)
			if err != nil {
				return fmt.Errorf("fetching catalog for %s: %w", toCountry, err)
			}

			toSet := make(map[string]string, len(toSlugs)) // slug → url
			for _, p := range toSlugs {
				toSet[p.Slug] = p.URL
			}
			fromSet := make(map[string]string, len(fromSlugs))
			for _, p := range fromSlugs {
				fromSet[p.Slug] = p.URL
			}

			var results []DiffResult
			for _, p := range fromSlugs {
				if _, inTo := toSet[p.Slug]; !inTo {
					if category == "" || strings.EqualFold(p.Category, category) {
						results = append(results, DiffResult{
							Slug:     p.Slug,
							URL:      p.URL,
							Category: p.Category,
							Market:   fromCountry + "_only",
						})
					}
				}
			}
			for _, p := range toSlugs {
				if _, inFrom := fromSet[p.Slug]; !inFrom {
					if category == "" || strings.EqualFold(p.Category, category) {
						results = append(results, DiffResult{
							Slug:     p.Slug,
							URL:      p.URL,
							Category: p.Category,
							Market:   toCountry + "_only",
						})
					}
				}
			}

			if results == nil {
				results = []DiffResult{}
			}

			data, err := json.Marshal(results)
			if err != nil {
				return err
			}
			if selectFields != "" {
				data = filterFields(data, selectFields)
			}
			return printOutput(cmd.OutOrStdout(), data, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
		},
	}
	cmd.Flags().StringVar(&fromCountry, "from", "", "Source country code (e.g. us)")
	cmd.Flags().StringVar(&toCountry, "to", "", "Target country code (e.g. gb)")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category (heets, terea, devices, accessories, veev)")
	cmd.Flags().StringVar(&selectFields, "select", "", "Comma-separated fields to include (e.g. slug,category)")
	return cmd
}

// catalogEntry is a minimal product row for diff/availability queries.
type catalogEntry struct {
	Slug     string
	URL      string
	Category string
	Price    string
}

// catalogSlugsForCountry returns the product slugs for a country, first from
// the local DB, then from the live sitemap.
func catalogSlugsForCountry(cmd *cobra.Command, flags *rootFlags, country string) ([]catalogEntry, error) {
	// Try local DB first
	db, err := iqosOpenDB(cmd.Context())
	if err == nil {
		defer db.Close()
		rows, qErr := db.DB().QueryContext(cmd.Context(),
			`SELECT slug, url, category, COALESCE(price,'') FROM iqos_products WHERE country=? ORDER BY slug`,
			country)
		if qErr == nil {
			defer rows.Close()
			var entries []catalogEntry
			for rows.Next() {
				var e catalogEntry
				if sErr := rows.Scan(&e.Slug, &e.URL, &e.Category, &e.Price); sErr == nil {
					entries = append(entries, e)
				}
			}
			if len(entries) > 0 {
				return entries, nil
			}
		}
	}

	// Fall back to live sitemap fetch
	baseURL := "https://www.iqos.com"
	lang := langForCountry(country)
	sitemapURLs, err := fetchSitemap(cmd.Context(), baseURL, country, lang, "product")
	if err != nil {
		return nil, err
	}

	entries := make([]catalogEntry, 0, len(sitemapURLs))
	for _, u := range sitemapURLs {
		slug := extractSlugFromURL(u.Loc)
		if slug == "" {
			continue
		}
		entries = append(entries, catalogEntry{
			Slug:     slug,
			URL:      u.Loc,
			Category: extractCategoryFromURL(u.Loc),
		})
	}
	return entries, nil
}
