// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `search` — keyword search across Atlas Obscura (hand-authored).
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/cliutil"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var page int
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Atlas Obscura wonders by keyword",
		Long: "Search Atlas Obscura's catalog of hidden wonders by keyword (relevance-ranked).\n" +
			"Results are cached to the local SQLite store for offline re-query.\n" +
			"Community-sourced from atlasobscura.com; not an official API.",
		Example: "  atlas-obscura-pp-cli search \"catacombs\" --json\n" +
			"  atlas-obscura-pp-cli search \"abandoned amusement park\" --limit 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search Atlas Obscura")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			query := args[0]
			if limit < 1 {
				limit = 15
			}
			if page < 1 {
				page = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			places, total, err := collectSearch(cmd.Context(), c, query, limit, page)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if s, err := aoDB(cmd.Context()); err == nil {
				for _, p := range places {
					cachePlace(s, p)
				}
				_ = s.Close()
			}

			return aoEmitPlaces(cmd, flags, map[string]any{
				"query":         query,
				"total_matches": total,
			}, places)
		},
	}
	cmd.Flags().IntVar(&page, "page", 1, "Page of results to start from (15 per page)")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum number of results to return")
	return cmd
}

// collectSearch pages through keyword search until limit results are gathered.
func collectSearch(ctx context.Context, c *client.Client, query string, limit, startPage int) ([]AOPlace, int, error) {
	var out []AOPlace
	total := 0
	for page := startPage; len(out) < limit; page++ {
		resp, err := aoSearch(ctx, c, query, "keyword", page)
		if err != nil {
			return nil, 0, err
		}
		total = resp.Total.Value
		if len(resp.Results) == 0 {
			break
		}
		for _, e := range resp.Results {
			p := e.toPlace()
			p.Score = aoScore(p)
			out = append(out, p)
			if len(out) >= limit {
				break
			}
		}
		// Stop when the page came back short (last page) or under dogfood.
		if len(resp.Results) < resp.PerPage || resp.PerPage == 0 || cliutil.IsDogfoodEnv() {
			break
		}
		if page-startPage >= 20 {
			break
		}
	}
	return out, total, nil
}
