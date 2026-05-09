// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// FlavorMatch is a flavor result from the local FTS index.
type FlavorMatch struct {
	Slug     string `json:"slug"`
	Country  string `json:"country"`
	Name     string `json:"name"`
	Category string `json:"category"`
	URL      string `json:"url"`
}

func newFlavorsFindCmd(flags *rootFlags) *cobra.Command {
	var profile string
	var country string
	var limit int

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search HEETS and TEREA sticks by flavor profile across markets",
		Long: `Full-text search the local synced flavor catalog for sticks matching a flavor
profile such as menthol, tobacco, berry, or citrus.

Requires at least one sync run: iqos-pp-cli sync --country gb`,
		Example: strings.Trim(`
  iqos-pp-cli flavors find --profile menthol --json
  iqos-pp-cli flavors find --profile "tropical fruit" --country gb
  iqos-pp-cli flavors find --profile menthol --country all --limit 20 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if profile == "" && len(args) > 0 {
				profile = strings.Join(args, " ")
			}
			if dryRunOK(flags) {
				return nil
			}
			if profile == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "Usage: iqos-pp-cli flavors find --profile <keyword>")
				return cmd.Help()
			}

			db, err := iqosOpenDB(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			matches, err := searchFlavors(cmd.Context(), db.DB(), profile, country, limit)
			if err != nil {
				return fmt.Errorf("searching flavors: %w", err)
			}

			if len(matches) == 0 {
				msg := fmt.Sprintf("No flavors found matching %q", profile)
				if country != "" && country != "all" {
					msg += fmt.Sprintf(" in %s", country)
				}
				msg += ". Run 'iqos-pp-cli sync --country gb' to populate the local catalog."
				fmt.Fprintln(cmd.OutOrStdout(), msg)
				return nil
			}

			data, err := json.Marshal(matches)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), data, flags.asJSON || !isTerminal(cmd.OutOrStdout()))
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Flavor keyword to search (e.g. menthol, tobacco, berry)")
	cmd.Flags().StringVar(&country, "country", "", "Limit to a specific country or 'all'")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results to return")
	return cmd
}

// searchFlavors queries the iqos_products table (and its FTS index when available)
// for products matching the flavor profile keyword. Falls back to LIKE when FTS
// returns no results.
func searchFlavors(ctx context.Context, rawDB *sql.DB, profile, country string, limit int) ([]FlavorMatch, error) {
	if limit <= 0 {
		limit = 50
	}

	// Only search flavor categories
	flavorCats := []string{"heets", "terea", "levia", "veev"}
	placeholders := make([]string, len(flavorCats))
	args := make([]interface{}, 0)
	for i, cat := range flavorCats {
		placeholders[i] = "?"
		args = append(args, cat)
	}
	catClause := "category IN (" + strings.Join(placeholders, ",") + ")"

	// Try FTS first
	ftsQuery := fmt.Sprintf(`
		SELECT p.slug, p.country, COALESCE(p.name,''), p.category, COALESCE(p.url,'')
		FROM iqos_products_fts fts
		JOIN iqos_products p ON p.slug=fts.slug AND p.country=fts.country
		WHERE fts.iqos_products_fts MATCH ? AND %s`, catClause)

	var countryArgs []interface{}
	if country != "" && country != "all" {
		ftsQuery += " AND p.country=?"
		countryArgs = append(countryArgs, country)
	}
	ftsQuery += " LIMIT ?"

	queryArgs := append([]interface{}{profile}, args...)
	queryArgs = append(queryArgs, countryArgs...)
	queryArgs = append(queryArgs, limit)

	rows, err := rawDB.QueryContext(ctx, ftsQuery, queryArgs...)
	if err == nil {
		matches := scanFlavorRows(rows)
		if len(matches) > 0 {
			return matches, nil
		}
	}

	// Fallback: LIKE search on name
	likePattern := "%" + strings.ToLower(profile) + "%"
	likeQuery := fmt.Sprintf(`
		SELECT slug, country, COALESCE(name,''), category, COALESCE(url,'')
		FROM iqos_products
		WHERE LOWER(name) LIKE ? AND %s`, catClause)

	likeArgs := append([]interface{}{likePattern}, args...)
	if country != "" && country != "all" {
		likeQuery += " AND country=?"
		likeArgs = append(likeArgs, country)
	}
	likeQuery += " ORDER BY country, name LIMIT ?"
	likeArgs = append(likeArgs, limit)

	rows2, err := rawDB.QueryContext(ctx, likeQuery, likeArgs...)
	if err != nil {
		return nil, err
	}
	return scanFlavorRows(rows2), nil
}

func scanFlavorRows(rows *sql.Rows) []FlavorMatch {
	defer rows.Close()
	var matches []FlavorMatch
	for rows.Next() {
		var m FlavorMatch
		if err := rows.Scan(&m.Slug, &m.Country, &m.Name, &m.Category, &m.URL); err == nil {
			matches = append(matches, m)
		}
	}
	return matches
}
