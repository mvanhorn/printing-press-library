package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/spf13/cobra"
)

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var topN int

	cmd := &cobra.Command{
		Use:         "trends",
		Short:       "Show frequently recurring items and top-rated recipes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Analyzes the local cache to surface items that appear across multiple lists
and recipes sorted by rating. Useful for identifying pantry staples and
popular meals. Requires sync — run 'anylist-pp-cli sync' first.`,
		Example: `  anylist-pp-cli trends
  anylist-pp-cli trends --top 10 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			st, err := store.Open(cfg)
			if err != nil {
				return fmt.Errorf("no local data found — run 'anylist-pp-cli sync' first")
			}
			defer st.Close()

			lists, err := st.GetLists()
			if err != nil {
				return fmt.Errorf("reading lists: %w", err)
			}

			// Aggregate item frequency across all lists
			freq := map[string]int{}
			checkedFreq := map[string]int{}
			for _, l := range lists {
				items, _ := st.GetItems(l.ID, nil)
				for _, it := range items {
					freq[it.Name]++
					if it.Checked {
						checkedFreq[it.Name]++
					}
				}
			}

			type itemTrend struct {
				Name           string  `json:"name"`
				Occurrences    int     `json:"occurrences"`
				CompletionRate float64 `json:"completion_rate_pct"`
			}

			var trends []itemTrend
			for name, count := range freq {
				rate := 0.0
				if count > 0 {
					rate = float64(checkedFreq[name]) / float64(count) * 100
				}
				trends = append(trends, itemTrend{Name: name, Occurrences: count, CompletionRate: rate})
			}
			sort.Slice(trends, func(i, j int) bool {
				return trends[i].Occurrences > trends[j].Occurrences
			})
			if topN > 0 && len(trends) > topN {
				trends = trends[:topN]
			}

			// Top-rated recipes
			recipes, err := st.GetRecipes()
			if err != nil {
				return fmt.Errorf("reading recipes: %w", err)
			}
			type recipeTrend struct {
				Name   string `json:"name"`
				Rating int    `json:"rating"`
			}
			var topRecipes []recipeTrend
			for _, r := range recipes {
				if r.Rating > 0 {
					topRecipes = append(topRecipes, recipeTrend{Name: r.Name, Rating: r.Rating})
				}
			}
			sort.Slice(topRecipes, func(i, j int) bool {
				return topRecipes[i].Rating > topRecipes[j].Rating
			})
			if topN > 0 && len(topRecipes) > topN {
				topRecipes = topRecipes[:topN]
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"top_items":   trends,
					"top_recipes": topRecipes,
				}, flags)
			}

			w := cmd.OutOrStdout()
			if len(trends) > 0 {
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "ITEM\tOCCURRENCES\tCOMPLETION %")
				for _, t := range trends {
					fmt.Fprintf(tw, "%s\t%d\t%.0f%%\n", t.Name, t.Occurrences, t.CompletionRate)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}

			if len(topRecipes) > 0 {
				fmt.Fprintln(w)
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "RECIPE\tRATING")
				for _, r := range topRecipes {
					fmt.Fprintf(tw, "%s\t%d\n", r.Name, r.Rating)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}

			if len(trends) == 0 && len(topRecipes) == 0 {
				fmt.Fprintln(w, "No data — run 'anylist-pp-cli sync' first")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&topN, "top", 20, "Limit results to this many entries per section (0 = all)")
	return cmd
}
