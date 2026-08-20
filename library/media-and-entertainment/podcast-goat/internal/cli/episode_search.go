// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `episode search` — FTS5 BM25 over cached transcripts.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newEpisodeSearchCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "FTS5 search across cached transcripts (BM25 ranking)",
		Example:     `  podcast-goat-pp-cli episode search "supply chain"`,
		Annotations: map[string]string{"pp:endpoint": "episode.search", "pp:method": "GET", "mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := ps.SearchEpisodes(cmd.Context(), query, flagLimit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no matches for %q in cached transcripts\n", query)
				return nil
			}
			headers := []string{"show", "title", "source", "fetched_at", "url"}
			var data [][]string
			for _, r := range rows {
				data = append(data, []string{r.Show, r.Title, r.Source, r.FetchedAt.Format("2006-01-02"), r.URL})
			}
			return flags.printTable(cmd, headers, data)
		},
	}
	cmd.Flags().IntVarP(&flagLimit, "limit", "n", 20, "Max number of hits")
	return cmd
}
