// Copyright 2026 Pietro Cimmaruta and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: rank synced products by a nutriment, offline.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type rankRow struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	Nutrient   string  `json:"nutrient"`
	NutriScore string  `json:"nutriscore_grade"`
}

func newNovelRankCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string
	var flagSort string
	var flagLimit int
	var flagAsc bool
	var flagDB string

	cmd := &cobra.Command{
		Use:         "rank",
		Short:       "Rank synced products in a category by any nutriment, entirely from the local store.",
		Long:        "Query the locally synced products (run `sync --resources find` first), optionally filter by --category, and rank by a nutriment (kcal, sugar, protein, fat, salt, satfat, carbs, fiber, sodium). Runs fully offline so it never touches the rate limit.",
		Example:     strings.Trim("\n  openfoodfacts-pp-cli rank --category breakfast-cereals --sort sugar\n  openfoodfacts-pp-cli rank --sort protein --asc --json", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			nutrientKey, ok := friendlyNutrientKey(flagSort)
			if !ok {
				return usageErr(fmt.Errorf("unknown --sort nutrient %q: use kcal, protein, fat, satfat, carbs, sugar, fiber, salt, or sodium", flagSort))
			}

			db, err := openLocalStore(cmd.Context(), flagDB)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM "find"`)
			if err != nil {
				return fmt.Errorf("reading local store: %w", err)
			}
			defer rows.Close()

			wantCat := strings.ToLower(strings.TrimSpace(flagCategory))
			results := []rankRow{}
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					return err
				}
				var prod map[string]any
				if json.Unmarshal([]byte(raw), &prod) != nil {
					continue
				}
				if wantCat != "" && !categoryMatches(prod, wantCat) {
					continue
				}
				nm, _ := prod["nutriments"].(map[string]any)
				if nm == nil {
					continue
				}
				v, present := nm[nutrientKey]
				if !present {
					continue
				}
				results = append(results, rankRow{
					Code:       prodString(prod, "code"),
					Name:       prodString(prod, "product_name"),
					Value:      coerceFloat(v),
					Nutrient:   nutrientKey,
					NutriScore: prodString(prod, "nutriscore_grade"),
				})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			sort.SliceStable(results, func(i, j int) bool {
				if flagAsc {
					return results[i].Value < results[j].Value
				}
				return results[i].Value > results[j].Value
			})
			if flagLimit > 0 && len(results) > flagLimit {
				results = results[:flagLimit]
			}

			if len(results) == 0 {
				// Hint on stderr in every mode (JSON consumers still get a clean
				// [] on stdout) so an unsynced store doesn't look like "no data".
				fmt.Fprintln(cmd.ErrOrStderr(), "no synced products matched. Run `sync --resources find` first to populate the local store.")
			}
			if wantsJSONOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no synced products matched. Run `sync --resources find` first to populate the local store.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintf(tw, "CODE\tNAME\t%s\tNUTRI\n", strings.ToUpper(flagSort))
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%.1f\t%s\n", r.Code, truncate(r.Name, 36), r.Value, strings.ToUpper(grade(r.NutriScore)))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagCategory, "category", "", "Filter to products whose categories include this term")
	cmd.Flags().StringVar(&flagSort, "sort", "kcal", "Nutriment to rank by (kcal, protein, fat, satfat, carbs, sugar, fiber, salt, sodium)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum number of rows")
	cmd.Flags().BoolVar(&flagAsc, "asc", false, "Sort ascending (lowest first) instead of descending")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

// categoryMatches reports whether a product's categories include the term.
func categoryMatches(prod map[string]any, term string) bool {
	for _, c := range prodTags(prod, "categories_tags") {
		if strings.Contains(c, term) {
			return true
		}
	}
	return false
}
