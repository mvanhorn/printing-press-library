package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/spf13/cobra"
)

func newCategoriesPromotedCmd(flags *rootFlags) *cobra.Command {
	var withCounts bool
	cmd := &cobra.Command{
		Use:         "categories",
		Short:       "List all item categories",
		Long:        "Shortcut for 'categories list'. List all item categories with optional item counts per category.",
		Example:     "  anylist-pp-cli categories\n  anylist-pp-cli categories --with-counts --json",
		Annotations: map[string]string{"pp:endpoint": "categories.list", "pp:method": "POST", "pp:path": "/data/user-data/get", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			st, err := store.Open(cfg)
			if err != nil {
				return fmt.Errorf("no local data found — run 'anylist-pp-cli sync' first")
			}
			defer st.Close()

			cats, err := st.GetCategories()
			if err != nil {
				return fmt.Errorf("reading categories: %w", err)
			}

			if len(cats) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No categories found — run 'anylist-pp-cli sync' first")
				return nil
			}

			// Build item count per category using GROUP BY aggregation
			itemCounts := map[string]int{}
			if withCounts {
				db := st.DB()
				rows, err := db.QueryContext(cmd.Context(),
					`SELECT category_match_id, COUNT(*) as count
					 FROM items
					 WHERE category_match_id != ''
					 GROUP BY category_match_id`)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var matchID string
						var count int
						if rows.Scan(&matchID, &count) == nil {
							itemCounts[matchID] = count
						}
					}
				}
			}

			if flags.asJSON {
				type catJSON struct {
					MatchID   string `json:"match_id"`
					Name      string `json:"name"`
					ItemCount int    `json:"item_count,omitempty"`
				}
				out := make([]catJSON, len(cats))
				for i, c := range cats {
					out[i] = catJSON{MatchID: c.MatchID, Name: c.Name, ItemCount: itemCounts[c.MatchID]}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			if withCounts {
				fmt.Fprintln(tw, "MATCH ID\tNAME\tITEMS")
				for _, c := range cats {
					fmt.Fprintf(tw, "%s\t%s\t%d\n", c.MatchID, c.Name, itemCounts[c.MatchID])
				}
			} else {
				fmt.Fprintln(tw, "MATCH ID\tNAME")
				for _, c := range cats {
					fmt.Fprintf(tw, "%s\t%s\n", c.MatchID, c.Name)
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&withCounts, "with-counts", false, "Show item count per category")
	return cmd
}
