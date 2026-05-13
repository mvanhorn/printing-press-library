// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel sources-rank command — rank domains by avg relevance score from stored searches.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/tavily/internal/store"
)

func newSourcesRankCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "sources-rank",
		Short: "Rank domains by average relevance score across all stored searches",
		Long: `Analyze all locally cached search results and compute the average
Tavily relevance score per domain. High-scoring domains are the most
consistently reliable sources for your search queries.

Useful for building domain allowlists or identifying authoritative sources
in your research domain.`,
		Example: `  tavily-pp-cli sources-rank
  tavily-pp-cli sources-rank --limit 20
  tavily-pp-cli sources-rank --json`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				limit = 20
			}

			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			scores, err := st.DomainScores(limit)
			if err != nil {
				return fmt.Errorf("computing domain scores: %w", err)
			}

			if flags.asJSON {
				data, _ := json.MarshalIndent(scores, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			if len(scores) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No search results cached yet. Run some searches first.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Top %d domains by avg relevance score:\n\n", len(scores))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-5s  %-8s  %-6s  %s\n", "Rank", "Avg Score", "Count", "Domain")
			fmt.Fprintf(cmd.OutOrStdout(), "  %-5s  %-8s  %-6s  %s\n", "----", "---------", "-----", "------")
			for i, s := range scores {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-5d  %-8.4f  %-6d  %s\n",
					i+1, s.AvgScore, s.Count, s.Domain)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Number of domains to show")
	return cmd
}
