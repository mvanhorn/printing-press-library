// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written: cache list/stats/clear — local-store inspection.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/allrecipes/internal/recipes"

	"github.com/spf13/cobra"
)

func newCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Inspect, list, or clear the local recipe cache",
	}
	cmd.AddCommand(newCacheStatsCmd(flags))
	cmd.AddCommand(newCacheListCmd(flags))
	cmd.AddCommand(newCacheClearCmd(flags))
	return cmd
}

func newCacheStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Show summary stats for the local recipe cache (count, on-disk path)",
		Example:     "  allrecipes-pp-cli cache stats --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s := openStoreForCommand()
			if s == nil {
				return fmt.Errorf("no local cache available")
			}
			defer s.Close()
			n, err := recipes.CountRecipes(s)
			if err != nil {
				return err
			}
			out := map[string]any{
				"recipeCount": n,
				"path":        s.Path(),
			}
			return renderJSON(cmd, flags, out)
		},
	}
	return cmd
}

func newCacheListCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagOrder string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List cached recipes from the local store, sorted by recency, rating, review count, total time, or name",
		Example:     "  allrecipes-pp-cli cache list --limit 50 --order rating --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s := openStoreForCommand()
			if s == nil {
				return fmt.Errorf("no local cache available")
			}
			defer s.Close()
			limit := flagLimit
			if limit <= 0 {
				limit = 50
			}
			order := flagOrder
			if order == "" {
				order = "fetched_at"
			}
			rows, err := recipes.QueryIndex(s, recipes.IndexQuery{
				Limit:     limit,
				OrderBy:   order,
				OrderDesc: true,
			})
			if err != nil {
				return err
			}
			return renderJSON(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum results")
	cmd.Flags().StringVar(&flagOrder, "order", "fetched_at", "Order by: fetched_at | rating | review_count | total_time | name")
	return cmd
}

func newCacheClearCmd(flags *rootFlags) *cobra.Command {
	var flagYes bool
	cmd := &cobra.Command{
		Use:     "clear",
		Short:   "Clear all cached recipes (irreversible)",
		Example: "  allrecipes-pp-cli cache clear --yes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !flagYes && !flags.yes {
				return fmt.Errorf("refusing to wipe cache without --yes (or global --yes)")
			}
			s := openStoreForCommand()
			if s == nil {
				return fmt.Errorf("no local cache available")
			}
			defer s.Close()
			if err := recipes.ClearCache(s); err != nil {
				return err
			}
			return renderJSON(cmd, flags, map[string]any{"cleared": true})
		},
	}
	cmd.Flags().BoolVar(&flagYes, "yes", false, "Confirm cache wipe")
	return cmd
}
