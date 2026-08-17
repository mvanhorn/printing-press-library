// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: chef roster analytics over the local mirror.
// Preserved on regen.
package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelChefsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
		sortBy string
	)

	cmd := &cobra.Command{
		Use:   "chefs",
		Short: "Leaderboard of chefs by dish count, average rating, average price, and cuisines on the current menu.",
		Long: `Aggregate the locally-synced menu by chef: dish count, average rating,
average price, and the cuisines each chef covers.

Run 'cookunity-pp-cli sync' first to populate the local store.`,
		Example:     "  cookunity-pp-cli chefs --sort dishes --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			path := resolveMealDBPath(dbPath)
			if !mealDBExists(path) {
				return missingMirror(cmd, path, flags)
			}
			meals, err := loadMeals(ctx, path)
			if err != nil {
				return err
			}

			type agg struct {
				Chef        string   `json:"chef"`
				ChefID      int      `json:"chef_id"`
				Dishes      int      `json:"dishes"`
				AvgRating   float64  `json:"avg_rating"`
				AvgPrice    float64  `json:"avg_price"`
				Cuisines    []string `json:"cuisines"`
				ratingSum   float64
				priceSum    float64
				cuisineSeen map[string]bool
			}
			byChef := map[string]*agg{}
			for _, m := range meals {
				if m.ChefName == "" {
					continue
				}
				a := byChef[m.ChefName]
				if a == nil {
					a = &agg{Chef: m.ChefName, ChefID: m.ChefId, cuisineSeen: map[string]bool{}}
					byChef[m.ChefName] = a
				}
				a.Dishes++
				a.ratingSum += m.Stars
				a.priceSum += m.FinalPrice
				// m.Cuisines is a ", "-joined list; add each cuisine individually.
				for _, cz := range strings.Split(m.Cuisines, ",") {
					cz = strings.TrimSpace(cz)
					if cz != "" && !a.cuisineSeen[cz] {
						a.cuisineSeen[cz] = true
						a.Cuisines = append(a.Cuisines, cz)
					}
				}
			}
			out := make([]*agg, 0, len(byChef))
			for _, a := range byChef {
				if a.Dishes > 0 {
					a.AvgRating = round1(a.ratingSum / float64(a.Dishes))
					a.AvgPrice = round2(a.priceSum / float64(a.Dishes))
				}
				out = append(out, a)
			}
			sort.SliceStable(out, func(i, j int) bool {
				switch sortBy {
				case "rating":
					return out[i].AvgRating > out[j].AvgRating
				case "price":
					return out[i].AvgPrice < out[j].AvgPrice
				default: // dishes
					return out[i].Dishes > out[j].Dishes
				}
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			return flags.printJSON(cmd, out)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard local path)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum chefs to return (0 = all)")
	cmd.Flags().StringVar(&sortBy, "sort", "dishes", "Sort by: dishes | rating | price")
	return cmd
}
